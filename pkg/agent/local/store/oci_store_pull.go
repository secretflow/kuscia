// Copyright 2024 Ant Group Co., Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package store

import (
	"compress/gzip"
	"context"
	"fmt"
	"hash/fnv"
	"io"
	"os"
	"path"
	"runtime"
	"sync"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/match"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/google/uuid"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

type imageAction struct {
	err      error
	status   int // 0-not started, 1-pulling, 2-finished
	image    string
	auth     *runtimeapi.AuthConfig
	wg       *sync.WaitGroup
	finishAt time.Time
}

// interface [Store]
func (s *ociStore) PullImage(image string, auth *runtimeapi.AuthConfig) error {
	var action *imageAction

	s.mutex.Lock()
	if pulling, ok := s.pullingImages[image]; ok {
		action = pulling
	}
	s.mutex.Unlock()

	if action == nil {
		action = &imageAction{
			status: 0,
			image:  image,
			auth:   auth,
			wg:     &sync.WaitGroup{},
		}
		action.wg.Add(1)

		s.mutex.Lock()
		s.pullingImages[image] = action

		if s.ctx == nil {
			s.ctx = context.Background()
			for i := uint32(0); i < s.pullWorkers; i++ {
				go s.pullImageWorker(s.ctx, i)
			}
		}
		s.mutex.Unlock()
	}

	action.wg.Wait()
	return action.err
}

func (s *ociStore) pullImageWorker(ctx context.Context, workerIdx uint32) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
		}

		var newPull *imageAction
		s.mutex.Lock()
		// clean up finsihed pull context
		removeFinished := make(map[string]*imageAction)
		for _, action := range s.pullingImages {
			if action.status == 2 && time.Since(action.finishAt) > time.Minute {
				continue
			}
			removeFinished[action.image] = action
		}
		s.pullingImages = removeFinished

		for _, action := range s.pullingImages {
			if action.status == 0 {
				hasher := fnv.New32()
				hasher.Write([]byte(action.image))
				if hasher.Sum32()%s.pullWorkers == workerIdx {
					newPull = action
					newPull.status = 1
					break
				}
			}
		}
		s.mutex.Unlock()

		if newPull != nil {
			newPull.err = s.doPullImage(newPull.image, newPull.auth)
			newPull.status = 2
			newPull.finishAt = time.Now()
			newPull.wg.Done()
		}
	}
}

func (s *ociStore) doPullImage(image string, auth *runtimeapi.AuthConfig) error {
	nlog.Infof("[OCI] Start to pull image(%s) ...", image)
	ref, err := name.ParseReference(image, s.options...)
	if err != nil {
		nlog.Warnf("Parse image(%s) failed with error: %s", image, err.Error())
		return err
	}

	nlog.Infof("Current Platform: OS=%s, Architecture=%s", runtime.GOOS, runtime.GOARCH)

	ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
	defer cancel()

	remoteOptions := s.getRemoteOpts(ctx, auth)

	rmt, err := remote.Get(ref, remoteOptions...)
	if err != nil {
		return err
	}

	var img v1.Image
	if rmt.MediaType.IsIndex() {
		nlog.Infof("[OCI] Image(%s) is multi-platform image", image)
		// multi-platform image
		// find the matched image
		idx, indexErr := rmt.ImageIndex()
		if indexErr != nil {
			return fmt.Errorf("query image index failed with %s", indexErr.Error())
		}
		if img, err = s.findMatchImage(idx); err != nil {
			return err
		}
	} else {
		if img, err = rmt.Image(); err != nil {
			return fmt.Errorf("query image failed with %s", err.Error())
		}
	}

	// combine all layers to one layer. for mount quickly
	nlog.Infof("[OCI] Start to flatten image(%s) ...", image)
	flat, cacheFile, err := s.flattenImage(img)
	if err != nil {
		nlog.Warnf("Flatten image(%s) failed with error: %s", image, err.Error())
		return err
	}
	//remove cache file
	defer os.Remove(cacheFile)
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// if image aready exists, will update it
	if err = s.imagePath.ReplaceImage(flat, match.Name(ref.Name()), s.kusciaImageAnnotation(ref.Name(), "")); err != nil {
		return err
	}

	nlog.Infof("[OCI] Finished pull image %s", image)

	return nil
}

func (s *ociStore) flattenImage(old v1.Image) (v1.Image, string, error) {
	m, err := old.Manifest()
	if err != nil {
		return nil, "", fmt.Errorf("reading manifest: %w", err)
	}

	cf, err := old.ConfigFile()
	if err != nil {
		return nil, "", fmt.Errorf("getting config: %w", err)
	}
	cf = cf.DeepCopy()

	// combine all layers and write to new image
	cf.RootFS.DiffIDs = []v1.Hash{}
	cf.History = []v1.History{}

	img, err := mutate.ConfigFile(empty.Image, cf)
	if err != nil {
		return nil, "", fmt.Errorf("mutating config: %w", err)
	}

	layer, cacheFile, err := s.combineLayers(old)
	if err != nil {
		return nil, "", err
	}
	img, err = mutate.AppendLayers(img, layer)
	if err != nil {
		return nil, "", err
	}

	// Retain any annotations from the original image.
	if len(m.Annotations) != 0 {
		img = mutate.Annotations(img, m.Annotations).(v1.Image)
	}

	return img, cacheFile, nil
}

func (s *ociStore) combineLayers(old v1.Image) (v1.Layer, string, error) {
	cacheFile := path.Join(s.cachePath, uuid.NewString())
	file, err := os.OpenFile(cacheFile, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return nil, "", err
	}

	defer file.Close()

	if _, err = io.Copy(file, mutate.Extract(old)); err != nil {
		return nil, "", err
	}
	layer, err := tarball.LayerFromFile(cacheFile, tarball.WithCompressionLevel(gzip.BestSpeed))
	return layer, cacheFile, err
}

func (s *ociStore) getRemoteOpts(ctx context.Context, auth *runtimeapi.AuthConfig) []remote.Option {
	remoteOptions := []remote.Option{
		remote.WithContext(ctx),
		remote.WithUserAgent("kuscia"),

		remote.WithPlatform(v1.Platform{
			OS:           runtime.GOOS,
			Architecture: runtime.GOARCH,
		}),
	}

	if auth != nil {
		remoteOptions = append(remoteOptions, remote.WithAuth(authn.FromConfig(authn.AuthConfig{
			Username:      auth.Username,
			Password:      auth.Password,
			IdentityToken: auth.IdentityToken,
			RegistryToken: auth.RegistryToken,
		})))
	}

	return remoteOptions
}
