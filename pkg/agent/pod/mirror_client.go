/*
Copyright 2015 The Kubernetes Authors.

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

package pod

import (
	"context"
	"errors"
	"fmt"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clientset "k8s.io/client-go/kubernetes"
	kubetypes "k8s.io/kubernetes/pkg/kubelet/types"

	pkgcontainer "github.com/secretflow/kuscia/pkg/agent/container"
	"github.com/secretflow/kuscia/pkg/agent/utils/format"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

// MirrorClient knows how to create/delete a mirror pod in the API server.
type MirrorClient interface {
	// CreateMirrorPod creates a mirror pod in the API server for the given
	// pod or returns an error.  The mirror pod will have the same annotations
	// as the given pod as well as an extra annotation containing the hash of
	// the static pod.
	CreateMirrorPod(pod *v1.Pod) error
	// DeleteMirrorPod deletes the mirror pod with the given full name from
	// the API server or returns an error.
	DeleteMirrorPod(podFullName string, uid *types.UID) (bool, error)
}

// nodeGetter is a subset of NodeLister, simplified for testing.
type nodeGetter interface {
	// GetNode retrieves the Node for a given name.
	GetNode() (*v1.Node, error)
}

// basicMirrorClient is a functional MirrorClient.  Mirror pods are stored in
// the kubelet directly because they need to be in sync with the internal
// pods.
type basicMirrorClient struct {
	apiserverClient clientset.Interface
	nodeGetter      nodeGetter
}

// NewBasicMirrorClient returns a new MirrorClient.
func NewBasicMirrorClient(apiserverClient clientset.Interface, nodeGetter nodeGetter) MirrorClient {
	return &basicMirrorClient{
		apiserverClient: apiserverClient,
		nodeGetter:      nodeGetter,
	}
}

func (mc *basicMirrorClient) CreateMirrorPod(pod *v1.Pod) error {
	if mc.apiserverClient == nil {
		return nil
	}
	// Make a copy of the pod.
	copyPod := *pod
	copyPod.Annotations = make(map[string]string)

	for k, v := range pod.Annotations {
		copyPod.Annotations[k] = v
	}
	hash := getPodHash(pod)
	copyPod.Annotations[kubetypes.ConfigMirrorAnnotationKey] = hash

	// With the MirrorPodNodeRestriction feature, mirror pods are required to have an owner reference
	// to the owning node.
	// See https://git.k8s.io/enhancements/keps/sig-auth/1314-node-restriction-pods/README.md
	node, err := mc.getNode()
	if err != nil {
		return fmt.Errorf("failed to get node UID: %v", err)
	}
	controller := true
	copyPod.OwnerReferences = []metav1.OwnerReference{{
		APIVersion: v1.SchemeGroupVersion.String(),
		Kind:       "Node",
		Name:       node.Name,
		UID:        node.UID,
		Controller: &controller,
	}}

	apiPod, err := mc.apiserverClient.CoreV1().Pods(copyPod.Namespace).Create(context.TODO(), &copyPod, metav1.CreateOptions{})
	if err != nil && apierrors.IsAlreadyExists(err) {
		// Check if the existing pod is the same as the pod we want to create.
		if h, ok := apiPod.Annotations[kubetypes.ConfigMirrorAnnotationKey]; ok && h == hash {
			return nil
		}
	}
	return err
}

// DeleteMirrorPod deletes a mirror pod.
// It takes the full name of the pod and optionally a UID.  If the UID
// is non-nil, the pod is deleted only if its UID matches the supplied UID.
// It returns whether the pod was actually deleted, and any error returned
// while parsing the name of the pod.
// Non-existence of the pod or UID mismatch is not treated as an error; the
// routine simply returns false in that case.
func (mc *basicMirrorClient) DeleteMirrorPod(podFullName string, uid *types.UID) (bool, error) {
	if mc.apiserverClient == nil {
		return false, nil
	}
	name, namespace, err := pkgcontainer.ParsePodFullName(podFullName)
	if err != nil {
		return false, fmt.Errorf("failed to parse a pod full name %q", podFullName)
	}

	var uidValue types.UID
	if uid != nil {
		uidValue = *uid
	}
	nlog.Infof("Deleting a mirror pod %q", format.PodDesc(name, namespace, uidValue))

	var GracePeriodSeconds int64
	if err := mc.apiserverClient.CoreV1().Pods(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{GracePeriodSeconds: &GracePeriodSeconds, Preconditions: &metav1.Preconditions{UID: uid}}); err != nil {
		// Unfortunately, there's no generic error for failing a precondition
		if !(apierrors.IsNotFound(err) || apierrors.IsConflict(err)) {
			// We should return the error here, but historically this routine does
			// not return an error unless it can't parse the pod name
			nlog.Warnf("Failed deleting a mirror pod %q", format.PodDesc(name, namespace, uidValue))
		}
		return false, nil
	}
	return true, nil
}

func (mc *basicMirrorClient) getNode() (*v1.Node, error) {
	node, err := mc.nodeGetter.GetNode()
	if err != nil {
		return nil, err
	}
	if node.UID == "" {
		return nil, errors.New("UID unset for current node")
	}

	return node, nil
}

func getHashFromMirrorPod(pod *v1.Pod) (string, bool) {
	hash, ok := pod.Annotations[kubetypes.ConfigMirrorAnnotationKey]
	return hash, ok
}

func getPodHash(pod *v1.Pod) string {
	// The annotation exists for all static pods.
	return pod.Annotations[kubetypes.ConfigHashAnnotationKey]
}
