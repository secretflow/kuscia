/*
Copyright 2014 The Kubernetes Authors.

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
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	api "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/apis/core/helper"
	kubetypes "k8s.io/kubernetes/pkg/kubelet/types"

	// Ensure that core apis are installed
	_ "k8s.io/kubernetes/pkg/apis/core/install"
	k8s_api_v1 "k8s.io/kubernetes/pkg/apis/core/v1"
	"k8s.io/kubernetes/pkg/apis/core/validation"
	"k8s.io/kubernetes/pkg/util/hash"
)

const (
	maxConfigLength = 10 * 1 << 20 // 10MB
)

// Generate a pod name that is unique among nodes by appending the nodeName.
func generatePodName(name string, nodeName types.NodeName) string {
	return fmt.Sprintf("%s-%s", name, strings.ToLower(string(nodeName)))
}

func applyDefaults(pod *api.Pod, source string, isFile bool, nodeName types.NodeName, namespace string) error {
	if len(pod.UID) == 0 {
		hasher := md5.New()
		hash.DeepHashObject(hasher, pod)
		// DeepHashObject resets the hash, so we should write the pod source
		// information AFTER it.
		if isFile {
			fmt.Fprintf(hasher, "host:%s", nodeName)
			fmt.Fprintf(hasher, "file:%s", source)
		} else {
			fmt.Fprintf(hasher, "url:%s", source)
		}
		pod.UID = types.UID(hex.EncodeToString(hasher.Sum(nil)[0:]))
	}

	pod.Name = generatePodName(pod.Name, nodeName)
	pod.Namespace = namespace

	// Set the Host field to indicate this pod is scheduled on the current node.
	pod.Spec.NodeName = string(nodeName)

	if pod.Annotations == nil {
		pod.Annotations = make(map[string]string)
	}
	// The generated UID is the hash of the file.
	pod.Annotations[kubetypes.ConfigHashAnnotationKey] = string(pod.UID)

	if isFile {
		// Applying the default Taint tolerations to static pods,
		// so they are not evicted when there are node problems.
		helper.AddOrUpdateTolerationInPod(pod, &api.Toleration{
			Operator: "Exists",
			Effect:   api.TaintEffectNoExecute,
		})
	}

	// Set the default status to pending.
	pod.Status.Phase = api.PodPending
	return nil
}

type defaultFunc func(pod *api.Pod) error

// tryDecodeSinglePod takes data and tries to extract valid Pod config information from it.
func tryDecodeSinglePod(data []byte, defaultFn defaultFunc) (parsed bool, pod *corev1.Pod, err error) {
	// JSON is valid YAML, so this should work for everything.
	json, err := utilyaml.ToJSON(data)
	if err != nil {
		return false, nil, err
	}
	obj, err := runtime.Decode(legacyscheme.Codecs.UniversalDecoder(), json)
	if err != nil {
		return false, pod, err
	}

	newPod, ok := obj.(*api.Pod)
	// Check whether the object could be converted to single pod.
	if !ok {
		return false, pod, fmt.Errorf("invalid pod: %#v", obj)
	}

	// Apply default values and validate the pod.
	if err = defaultFn(newPod); err != nil {
		return true, pod, err
	}
	if errs := validation.ValidatePodCreate(newPod, validation.PodValidationOptions{}); len(errs) > 0 {
		return true, pod, fmt.Errorf("invalid pod: %v", errs)
	}
	v1Pod := &corev1.Pod{}
	if err := k8s_api_v1.Convert_core_Pod_To_v1_Pod(newPod, v1Pod, nil); err != nil {
		return true, nil, fmt.Errorf("pod %v/%v failed to convert to v1, detail-> %v", newPod.Namespace, newPod.Name, err)
	}
	return true, v1Pod, nil
}
