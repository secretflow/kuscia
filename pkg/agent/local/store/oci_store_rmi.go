// Copyright 2024 Ant Group Co., Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package store

import (
	"fmt"
	"strings"

	_ "github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/match"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

const (
	tagDelim = ":"
)

// RemoveImage remove image from local by name or id
func (s *ociStore) RemoveImage(imageNameOrIDs []string) error {

	s.mutex.Lock()
	defer s.mutex.Unlock()

	digestMap, err := s.getAllDigests()
	if err != nil {
		return err
	}
	if len(digestMap) == 0 {
		for _, imageNameOrID := range imageNameOrIDs {
			nlog.Errorf("No such image %s", imageNameOrID)
		}
		return nil
	}
	for _, imageNameOrID := range imageNameOrIDs {
		newImageNameOrID := imageNameOrID
		if !isImageID(imageNameOrID) && !strings.Contains(imageNameOrID, tagDelim) {
			newImageNameOrID = fmt.Sprintf("%s%slatest", imageNameOrID, tagDelim)
		}
		err = s.removeImgByTagOrID(newImageNameOrID, digestMap)
		if err != nil {
			return err
		}
	}

	return nil
}

func isImageID(id string) bool {
	// remove  "sha256:"
	id = strings.TrimPrefix(id, common.ImageIDPrefix)

	// check length
	if l := len(id); l < 4 || l > 64 {
		return false
	}

	// check each character is in '0'-'9' or 'a'-'f' range
	for _, c := range id {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}

	return true
}

func (s *ociStore) removeImgByTagOrID(imageNameOrID string, digestMap map[string][]string) error {
	var err error
	ii, err := s.imagePath.ImageIndex()
	if err != nil {
		return err
	}

	imf, err := ii.IndexManifest()
	if err != nil {
		return err
	}
	imageExist := false
	for _, img := range imf.Manifests {
		var image v1.Image
		if name, ok := s.getImageName(img); ok && (imageNameOrID == name) {
			nlog.Debugf("Found the image(%s) in local store, digest is %s", imageNameOrID, img.Digest.String())
			image, err = s.imagePath.Image(img.Digest)
			if err != nil {
				return err
			}

		}
		if strings.HasPrefix(img.Digest.String(), fmt.Sprintf("%s%s", common.ImageIDPrefix, imageNameOrID)) {
			nlog.Debugf("Found the image(%s) in local store, digest is %s", imageNameOrID, img.Digest.String())
			image, err = s.imagePath.Image(img.Digest)
			if err != nil {
				return err
			}
		}
		if image != nil {
			imageExist = true
			// remove image file
			err = s.removeImageFile(image)
			if err != nil {
				return err
			}
			//print deleted image name
			if v, ok := digestMap[img.Digest.String()]; ok {
				for _, name := range v {
					nlog.Infof("Deleted: %s", name)
				}
			}
			break
		}
	}
	if !imageExist {
		nlog.Errorf("No such image %s", imageNameOrID)
	}
	return nil
}

func (s *ociStore) getAllDigests() (map[string][]string, error) {
	digestMap := make(map[string][]string)
	ii, err := s.imagePath.ImageIndex()
	if err != nil {
		return nil, err
	}
	imf, err := ii.IndexManifest()
	if err != nil {
		return nil, err
	}
	for _, img := range imf.Manifests {
		if name, ok := s.getImageName(img); ok {
			if imageNameArr, ok1 := digestMap[img.Digest.String()]; ok1 {
				imageNameArr = append(imageNameArr, name)
				digestMap[img.Digest.String()] = imageNameArr
			} else {
				digestMap[img.Digest.String()] = []string{name}
			}
		}
	}
	return digestMap, nil
}

func (s *ociStore) removeImageFile(img v1.Image) error {
	digest, err := img.Digest()
	if err != nil {
		return err
	}
	// remove image info from index.json
	err = s.imagePath.RemoveDescriptors(match.Digests(digest))
	if err != nil {
		return err
	}
	nlog.Debug("Remove index.json")
	//remove image config file
	cfgName, err := img.ConfigName()
	if err != nil {
		return err
	}

	err = s.imagePath.RemoveBlob(cfgName)
	if err != nil {
		return err
	}
	nlog.Debugf("Remove configFile name: %s", cfgName)

	// remove image manifest file
	err = s.imagePath.RemoveBlob(digest)
	if err != nil {
		return err
	}
	nlog.Debugf("Remove manifest %s", digest)
	// remove image blobs file
	layers, _ := img.Layers()
	for _, v := range layers {
		digest, err = v.Digest()
		if err != nil {
			return err
		}
		err = s.imagePath.RemoveBlob(digest)
		if err != nil {
			return err
		}
		nlog.Debugf("Remove layer %s", digest)
	}
	return nil
}
