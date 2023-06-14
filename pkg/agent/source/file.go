/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Modified by Ant Group in 2023.

package source

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	api "k8s.io/kubernetes/pkg/apis/core"
	kubetypes "k8s.io/kubernetes/pkg/kubelet/types"
	utilio "k8s.io/utils/io"

	"github.com/secretflow/kuscia/pkg/agent/utils/format"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

type podEventType int

const (
	podAdd podEventType = iota + 1
	podModify
	podDelete

	eventBufferLen = 10
)

type watchEvent struct {
	fileName  string
	eventType podEventType
}

type sourceFile struct {
	path           string
	namespace      string
	nodeName       types.NodeName
	period         time.Duration
	store          cache.Store
	fileKeyMapping map[string]string
	updates        chan<- kubetypes.PodUpdate

	watchEvents chan *watchEvent
}

// NewSourceFile watches a config file for changes.
func newSourceFile(cfg *InitConfig, updates chan<- kubetypes.PodUpdate) *sourceFile {
	send := func(objs []interface{}) {
		var pods []*v1.Pod
		for _, o := range objs {
			pod, ok := o.(*v1.Pod)
			if ok {
				pods = append(pods, pod)
			}
		}
		updates <- kubetypes.PodUpdate{Pods: pods, Op: kubetypes.SET, Source: kubetypes.FileSource}
	}
	store := cache.NewUndeltaStore(send, cache.MetaNamespaceKeyFunc)

	path := strings.TrimRight(cfg.SourceCfg.File.Path, string(os.PathSeparator))

	return &sourceFile{
		path:           path,
		namespace:      cfg.Namespace,
		nodeName:       cfg.NodeName,
		period:         cfg.SourceCfg.File.Period,
		store:          store,
		fileKeyMapping: map[string]string{},
		updates:        updates,
		watchEvents:    make(chan *watchEvent, eventBufferLen),
	}
}

func (s *sourceFile) run() {
	nlog.Infof("Start running file source, watching path: %v", s.path)

	listTicker := time.NewTicker(s.period)

	go func() {
		// Read path immediately to speed up startup.
		if err := s.listConfig(); err != nil {
			nlog.Errorf("Unable to read config path %q: %v", s.path, err)
		}
		for {
			select {
			case <-listTicker.C:
				if err := s.listConfig(); err != nil {
					nlog.Errorf("Unable to read config path %q: %v", s.path, err)
				}
			case e := <-s.watchEvents:
				if err := s.consumeWatchEvent(e); err != nil {
					nlog.Errorf("Unable to process watch event: %v", err)
				}
			}
		}
	}()

	s.startWatch()
}

func (s *sourceFile) applyDefaults(pod *api.Pod, source string) error {
	return applyDefaults(pod, source, true, s.nodeName, s.namespace)
}

func (s *sourceFile) listConfig() error {
	path := s.path
	statInfo, err := os.Stat(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		// Emit an update with an empty PodList to allow FileSource to be marked as seen
		s.updates <- kubetypes.PodUpdate{Pods: []*v1.Pod{}, Op: kubetypes.SET, Source: kubetypes.FileSource}
		return fmt.Errorf("path does not exist, ignoring")
	}

	switch {
	case statInfo.Mode().IsDir():
		pods, err := s.extractFromDir(path)
		if err != nil {
			return err
		}
		if len(pods) == 0 {
			// Emit an update with an empty PodList to allow FileSource to be marked as seen
			s.updates <- kubetypes.PodUpdate{Pods: pods, Op: kubetypes.SET, Source: kubetypes.FileSource}
			return nil
		}

		nlog.Debugf("Extract pods %q from directory %q", format.Pods(pods), path)
		return s.replaceStore(pods...)

	case statInfo.Mode().IsRegular():
		pod, err := s.extractFromFile(path)
		if err != nil {
			return err
		}

		nlog.Debugf("Extract pod %q from file %q", format.Pod(pod), path)
		return s.replaceStore(pod)

	default:
		return fmt.Errorf("path is not a directory or file")
	}
}

// Get as many pod manifests as we can from a directory. Return an error if and only if something
// prevented us from reading anything at all. Do not return an error if only some files
// were problematic.
func (s *sourceFile) extractFromDir(name string) ([]*v1.Pod, error) {
	dirents, err := filepath.Glob(filepath.Join(name, "[^.]*"))
	if err != nil {
		return nil, fmt.Errorf("glob failed: %v", err)
	}

	pods := make([]*v1.Pod, 0, len(dirents))
	if len(dirents) == 0 {
		return pods, nil
	}

	sort.Strings(dirents)
	for _, path := range dirents {
		statInfo, err := os.Stat(path)
		if err != nil {
			nlog.Errorf("Could not get metadata, path=%v", path)
			continue
		}

		switch {
		case statInfo.Mode().IsDir():
			nlog.Warnf("Provided manifest path %q is a directory, not recursing into manifest path", path)
		case statInfo.Mode().IsRegular():
			pod, err := s.extractFromFile(path)
			if err != nil {
				if !os.IsNotExist(err) {
					nlog.Errorf("Could not process manifest file %q: %v", path, err)
				}
			} else {
				pods = append(pods, pod)
			}
		default:
			nlog.Warnf("Manifest path %q is not a directory or file, mode is %q", path, statInfo.Mode())
		}
	}
	return pods, nil
}

// extractFromFile parses a file for Pod configuration information.
func (s *sourceFile) extractFromFile(filename string) (pod *v1.Pod, err error) {
	klog.V(3).InfoS("Reading config file", "path", filename)
	defer func() {
		if err == nil && pod != nil {
			objKey, keyErr := cache.MetaNamespaceKeyFunc(pod)
			if keyErr != nil {
				err = keyErr
				return
			}
			s.fileKeyMapping[filename] = objKey
		}
	}()

	file, err := os.Open(filename)
	if err != nil {
		return pod, err
	}
	defer file.Close()

	data, err := utilio.ReadAtMost(file, maxConfigLength)
	if err != nil {
		return pod, err
	}

	defaultFn := func(pod *api.Pod) error {
		return s.applyDefaults(pod, filename)
	}

	parsed, pod, podErr := tryDecodeSinglePod(data, defaultFn)
	if parsed {
		if podErr != nil {
			return pod, podErr
		}
		return pod, nil
	}

	return pod, fmt.Errorf("%v: couldn't parse as pod(%v), please check config file", filename, podErr)
}

func (s *sourceFile) replaceStore(pods ...*v1.Pod) (err error) {
	var objs []interface{}
	for _, pod := range pods {
		objs = append(objs, pod)
	}
	return s.store.Replace(objs, "")
}
