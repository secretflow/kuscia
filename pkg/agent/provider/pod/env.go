// Copyright © 2017 The virtual-kubelet authors
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

// Modified by Ant Group in 2023.

package pod

import (
	"context"
	"fmt"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	apivalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/tools/record"
	podshelper "k8s.io/kubernetes/pkg/apis/core/pods"
	"k8s.io/kubernetes/pkg/fieldpath"
	"k8s.io/kubernetes/third_party/forked/golang/expansion"

	pkgcontainer "github.com/secretflow/kuscia/pkg/agent/container"

	"github.com/secretflow/kuscia/pkg/agent/resource"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

const (
	// ReasonOptionalConfigMapNotFound is the reason used in events emitted when an optional configmap is not found.
	ReasonOptionalConfigMapNotFound = "OptionalConfigMapNotFound"
	// ReasonOptionalConfigMapKeyNotFound is the reason used in events emitted when an optional configmap key is not found.
	ReasonOptionalConfigMapKeyNotFound = "OptionalConfigMapKeyNotFound"
	// ReasonFailedToReadOptionalConfigMap is the reason used in events emitted when an optional configmap could not be read.
	ReasonFailedToReadOptionalConfigMap = "FailedToReadOptionalConfigMap"

	// ReasonOptionalSecretNotFound is the reason used in events emitted when an optional secret is not found.
	ReasonOptionalSecretNotFound = "OptionalSecretNotFound"
	// ReasonOptionalSecretKeyNotFound is the reason used in events emitted when an optional secret key is not found.
	ReasonOptionalSecretKeyNotFound = "OptionalSecretKeyNotFound"
	// ReasonFailedToReadOptionalSecret is the reason used in events emitted when an optional secret could not be read.
	ReasonFailedToReadOptionalSecret = "FailedToReadOptionalSecret"

	// ReasonMandatoryConfigMapNotFound is the reason used in events emitted when a mandatory configmap is not found.
	ReasonMandatoryConfigMapNotFound = "MandatoryConfigMapNotFound"
	// ReasonMandatoryConfigMapKeyNotFound is the reason used in events emitted when a mandatory configmap key is not found.
	ReasonMandatoryConfigMapKeyNotFound = "MandatoryConfigMapKeyNotFound"
	// ReasonFailedToReadMandatoryConfigMap is the reason used in events emitted when a mandatory configmap could not be read.
	ReasonFailedToReadMandatoryConfigMap = "FailedToReadMandatoryConfigMap"

	// ReasonMandatorySecretNotFound is the reason used in events emitted when a mandatory secret is not found.
	ReasonMandatorySecretNotFound = "MandatorySecretNotFound"
	// ReasonMandatorySecretKeyNotFound is the reason used in events emitted when a mandatory secret key is not found.
	ReasonMandatorySecretKeyNotFound = "MandatorySecretKeyNotFound"
	// ReasonFailedToReadMandatorySecret is the reason used in events emitted when a mandatory secret could not be read.
	ReasonFailedToReadMandatorySecret = "FailedToReadMandatorySecret"

	// ReasonInvalidEnvironmentVariableNames is the reason used in events emitted when a configmap/secret referenced in a ".spec.containers[*].envFrom" field contains invalid environment variable names.
	ReasonInvalidEnvironmentVariableNames = "InvalidEnvironmentVariableNames"
)

// populateContainerEnvironment populates the environment of a single container in the specified pod.
func populateContainerEnvironment(ctx context.Context, pod *corev1.Pod, container *corev1.Container, rm *resource.KubeResourceManager, recorder record.EventRecorder) ([]pkgcontainer.EnvVar, error) {
	// Create an "environment map" based on the value of the specified container's ".envFrom" field.
	tmpEnv, err := makeEnvironmentMapBasedOnEnvFrom(ctx, pod, container, rm, recorder)
	if err != nil {
		return nil, err
	}
	// Create the final "environment map" for the container using the ".env" and ".envFrom" field
	// and service environment variables.
	err = makeEnvironmentMap(ctx, pod, container, rm, recorder, tmpEnv)
	if err != nil {
		return nil, err
	}

	var res []pkgcontainer.EnvVar

	for key, val := range tmpEnv {
		res = append(res, pkgcontainer.EnvVar{
			Name:  key,
			Value: val,
		})
	}

	return res, nil
}

// makeEnvironmentMapBasedOnEnvFrom returns a map representing the resolved environment of the specified container after being populated from the entries in the ".envFrom" field.
func makeEnvironmentMapBasedOnEnvFrom(ctx context.Context, pod *corev1.Pod, container *corev1.Container, rm *resource.KubeResourceManager, recorder record.EventRecorder) (map[string]string, error) {
	// Create a map to hold the resulting environment.
	res := make(map[string]string, 0)
	// Iterate over "envFrom" references in order to populate the environment.
loop:
	for _, envFrom := range container.EnvFrom {
		switch {
		// Handle population from a configmap.
		case envFrom.ConfigMapRef != nil:
			ef := envFrom.ConfigMapRef
			// Check whether the configmap reference is optional.
			// This will control whether we fail when unable to read the configmap.
			optional := ef.Optional != nil && *ef.Optional
			// Try to grab the referenced configmap.
			m, err := rm.GetConfigMap(ef.Name)
			if err != nil {
				// We couldn't fetch the configmap.
				// However, if the configmap reference is optional we should not fail.
				if optional {
					if k8serrors.IsNotFound(err) {
						recorder.Eventf(pod, corev1.EventTypeWarning, ReasonOptionalConfigMapNotFound, "configmap %q not found", ef.Name)
					} else {
						nlog.Warnf("failed to read configmap %q: %v", ef.Name, err)
						recorder.Eventf(pod, corev1.EventTypeWarning, ReasonFailedToReadOptionalConfigMap, "failed to read configmap %q", ef.Name)
					}
					// Continue on to the next reference.
					continue loop
				}
				// At this point we know the configmap reference is mandatory.
				// Hence, we should return a meaningful error.
				if k8serrors.IsNotFound(err) {
					recorder.Eventf(pod, corev1.EventTypeWarning, ReasonMandatoryConfigMapNotFound, "configmap %q not found", ef.Name)
					return nil, fmt.Errorf("configmap %q not found", ef.Name)
				}
				recorder.Eventf(pod, corev1.EventTypeWarning, ReasonFailedToReadMandatoryConfigMap, "failed to read configmap %q", ef.Name)
				return nil, fmt.Errorf("failed to fetch configmap %q: %v", ef.Name, err)
			}
			// At this point we have successfully fetched the target configmap.
			// Iterate over the keys defined in the configmap and populate the environment accordingly.
			// https://github.com/kubernetes/kubernetes/blob/v1.13.1/pkg/kubelet/kubelet_pods.go#L581-L595
			var invalidKeys []string
		mKeys:
			for key, val := range m.Data {
				// If a prefix has been defined, prepend it to the environment variable's name.
				if len(envFrom.Prefix) > 0 {
					key = envFrom.Prefix + key
				}
				// Make sure that the resulting key is a valid environment variable name.
				// If it isn't, it should be appended to the list of invalid keys and skipped.
				if errMsgs := apivalidation.IsEnvVarName(key); len(errMsgs) != 0 {
					invalidKeys = append(invalidKeys, key)
					continue mKeys
				}
				// Add the key and its value to the environment.
				res[key] = val
			}
			// Report any invalid keys.
			if len(invalidKeys) > 0 {
				sort.Strings(invalidKeys)
				recorder.Eventf(pod, corev1.EventTypeWarning, ReasonInvalidEnvironmentVariableNames, "keys [%s] from configmap %s/%s were skipped since they are invalid as environment variable names", strings.Join(invalidKeys, ", "), m.Namespace, m.Name)
			}
		// Handle population from a secret.
		case envFrom.SecretRef != nil:
			ef := envFrom.SecretRef
			// Check whether the secret reference is optional.
			// This will control whether we fail when unable to read the secret.
			optional := ef.Optional != nil && *ef.Optional
			// Try to grab the referenced secret.
			s, err := rm.GetSecret(ef.Name)
			if err != nil {
				// We couldn't fetch the secret.
				// However, if the secret reference is optional we should not fail.
				if optional {
					if k8serrors.IsNotFound(err) {
						recorder.Eventf(pod, corev1.EventTypeWarning, ReasonOptionalSecretNotFound, "secret %q not found", ef.Name)
					} else {
						nlog.Warnf("failed to read secret %q: %v", ef.Name, err)
						recorder.Eventf(pod, corev1.EventTypeWarning, ReasonFailedToReadOptionalSecret, "failed to read secret %q", ef.Name)
					}
					// Continue on to the next reference.
					continue loop
				}
				// At this point we know the secret reference is mandatory.
				// Hence, we should return a meaningful error.
				if k8serrors.IsNotFound(err) {
					recorder.Eventf(pod, corev1.EventTypeWarning, ReasonMandatorySecretNotFound, "secret %q not found", ef.Name)
					return nil, fmt.Errorf("secret %q not found", ef.Name)
				}
				recorder.Eventf(pod, corev1.EventTypeWarning, ReasonFailedToReadMandatorySecret, "failed to read secret %q", ef.Name)
				return nil, fmt.Errorf("failed to fetch secret %q: %v", ef.Name, err)
			}
			// At this point we have successfully fetched the target secret.
			// Iterate over the keys defined in the secret and populate the environment accordingly.
			// https://github.com/kubernetes/kubernetes/blob/v1.13.1/pkg/kubelet/kubelet_pods.go#L581-L595
			var invalidKeys []string
		sKeys:
			for key, val := range s.Data {
				// If a prefix has been defined, prepend it to the environment variable's name.
				if len(envFrom.Prefix) > 0 {
					key = envFrom.Prefix + key
				}
				// Make sure that the resulting key is a valid environment variable name.
				// If it isn't, it should be appended to the list of invalid keys and skipped.
				if errMsgs := apivalidation.IsEnvVarName(key); len(errMsgs) != 0 {
					invalidKeys = append(invalidKeys, key)
					continue sKeys
				}
				// Add the key and its value to the environment.
				res[key] = string(val)
			}
			// Report any invalid keys.
			if len(invalidKeys) > 0 {
				sort.Strings(invalidKeys)
				recorder.Eventf(pod, corev1.EventTypeWarning, ReasonInvalidEnvironmentVariableNames, "keys [%s] from secret %s/%s were skipped since they are invalid as environment variable names", strings.Join(invalidKeys, ", "), s.Namespace, s.Name)
			}
		}
	}
	// Return the populated environment.
	return res, nil
}

// makeEnvironmentMap returns a map representing the resolved environment of the specified container after being populated from the entries in the ".env" and ".envFrom" field.
func makeEnvironmentMap(ctx context.Context, pod *corev1.Pod, container *corev1.Container, rm *resource.KubeResourceManager, recorder record.EventRecorder, res map[string]string) error {
	// If the variable's Value is set, expand the `$(var)` references to other
	// variables in the .Value field; the sources of variables are the declared
	// variables of the container and the service environment variables.
	mappingFunc := expansion.MappingFuncFor(res)

	// Iterate over environment variables in order to populate the map.
loop:
	for _, env := range container.Env {
		switch {
		// Handle values that have been directly provided.
		case env.Value != "":
			// Expand variable references
			res[env.Name] = expansion.Expand(env.Value, mappingFunc)
			continue loop
		// Handle population from a configmap key.
		case env.ValueFrom != nil && env.ValueFrom.ConfigMapKeyRef != nil:
			// The environment variable must be set from a configmap.
			vf := env.ValueFrom.ConfigMapKeyRef
			// Check whether the key reference is optional.
			// This will control whether we fail when unable to read the requested key.
			optional := vf != nil && vf.Optional != nil && *vf.Optional
			// Try to grab the referenced configmap.
			m, err := rm.GetConfigMap(vf.Name)
			if err != nil {
				// We couldn't fetch the configmap.
				// However, if the key reference is optional we should not fail.
				if optional {
					if k8serrors.IsNotFound(err) {
						recorder.Eventf(pod, corev1.EventTypeWarning, ReasonOptionalConfigMapNotFound, "skipping optional envvar %q: configmap %q not found", env.Name, vf.Name)
					} else {
						nlog.Warnf("failed to read configmap %q: %v", vf.Name, err)
						recorder.Eventf(pod, corev1.EventTypeWarning, ReasonFailedToReadOptionalConfigMap, "skipping optional envvar %q: failed to read configmap %q", env.Name, vf.Name)
					}
					// Continue on to the next reference.
					continue loop
				}
				// At this point we know the key reference is mandatory.
				// Hence, we should return a meaningful error.
				if k8serrors.IsNotFound(err) {
					recorder.Eventf(pod, corev1.EventTypeWarning, ReasonMandatoryConfigMapNotFound, "configmap %q not found", vf.Name)
					return fmt.Errorf("configmap %q not found", vf.Name)
				}
				recorder.Eventf(pod, corev1.EventTypeWarning, ReasonFailedToReadMandatoryConfigMap, "failed to read configmap %q", vf.Name)
				return fmt.Errorf("failed to read configmap %q: %v", vf.Name, err)
			}
			// At this point we have successfully fetched the target configmap.
			// We must now try to grab the requested key.
			var (
				keyExists bool
				keyValue  string
			)
			if keyValue, keyExists = m.Data[vf.Key]; !keyExists {
				// The requested key does not exist.
				// However, we should not fail if the key reference is optional.
				if optional {
					// Continue on to the next reference.
					recorder.Eventf(pod, corev1.EventTypeWarning, ReasonOptionalConfigMapKeyNotFound, "skipping optional envvar %q: key %q does not exist in configmap %q", env.Name, vf.Key, vf.Name)
					continue loop
				}
				// At this point we know the key reference is mandatory.
				// Hence, we should fail.
				recorder.Eventf(pod, corev1.EventTypeWarning, ReasonMandatoryConfigMapKeyNotFound, "key %q does not exist in configmap %q", vf.Key, vf.Name)
				return fmt.Errorf("configmap %q doesn't contain the %q key required by pod %s", vf.Name, vf.Key, pod.Name)
			}
			// Populate the environment variable and continue on to the next reference.
			res[env.Name] = keyValue
			continue loop
		// Handle population from a secret key.
		case env.ValueFrom != nil && env.ValueFrom.SecretKeyRef != nil:
			vf := env.ValueFrom.SecretKeyRef
			// Check whether the key reference is optional.
			// This will control whether we fail when unable to read the requested key.
			optional := vf != nil && vf.Optional != nil && *vf.Optional
			// Try to grab the referenced secret.
			s, err := rm.GetSecret(vf.Name)
			if err != nil {
				// We couldn't fetch the secret.
				// However, if the key reference is optional we should not fail.
				if optional {
					if k8serrors.IsNotFound(err) {
						recorder.Eventf(pod, corev1.EventTypeWarning, ReasonOptionalSecretNotFound, "skipping optional envvar %q: secret %q not found", env.Name, vf.Name)
					} else {
						nlog.Warnf("failed to read secret %q: %v", vf.Name, err)
						recorder.Eventf(pod, corev1.EventTypeWarning, ReasonFailedToReadOptionalSecret, "skipping optional envvar %q: failed to read secret %q", env.Name, vf.Name)
					}
					// Continue on to the next reference.
					continue loop
				}
				// At this point we know the key reference is mandatory.
				// Hence, we should return a meaningful error.
				if k8serrors.IsNotFound(err) {
					recorder.Eventf(pod, corev1.EventTypeWarning, ReasonMandatorySecretNotFound, "secret %q not found", vf.Name)
					return fmt.Errorf("secret %q not found", vf.Name)
				}
				recorder.Eventf(pod, corev1.EventTypeWarning, ReasonFailedToReadMandatorySecret, "failed to read secret %q", vf.Name)
				return fmt.Errorf("failed to read secret %q: %v", vf.Name, err)
			}
			// At this point we have successfully fetched the target secret.
			// We must now try to grab the requested key.
			var (
				keyExists bool
				keyValue  []byte
			)
			if keyValue, keyExists = s.Data[vf.Key]; !keyExists {
				// The requested key does not exist.
				// However, we should not fail if the key reference is optional.
				if optional {
					// Continue on to the next reference.
					recorder.Eventf(pod, corev1.EventTypeWarning, ReasonOptionalSecretKeyNotFound, "skipping optional envvar %q: key %q does not exist in secret %q", env.Name, vf.Key, vf.Name)
					continue loop
				}
				// At this point we know the key reference is mandatory.
				// Hence, we should fail.
				recorder.Eventf(pod, corev1.EventTypeWarning, ReasonMandatorySecretKeyNotFound, "key %q does not exist in secret %q", vf.Key, vf.Name)
				return fmt.Errorf("secret %q doesn't contain the %q key required by pod %s", vf.Name, vf.Key, pod.Name)
			}
			// Populate the environment variable and continue on to the next reference.
			res[env.Name] = string(keyValue)
			continue loop
		// Handle population from a field (downward API).
		case env.ValueFrom != nil && env.ValueFrom.FieldRef != nil:
			vf := env.ValueFrom.FieldRef

			runtimeVal, err := podFieldSelectorRuntimeValue(vf, pod)
			if err != nil {
				return err
			}

			res[env.Name] = runtimeVal

			continue loop
		// Handle population from a resource request/limit.
		case env.ValueFrom != nil && env.ValueFrom.ResourceFieldRef != nil:
			// TODO Implement populating resource requests.
			continue loop
		}
	}

	return nil
}

// podFieldSelectorRuntimeValue returns the runtime value of the given
// selector for a pod.
func podFieldSelectorRuntimeValue(fs *corev1.ObjectFieldSelector, pod *corev1.Pod) (string, error) {
	internalFieldPath, _, err := podshelper.ConvertDownwardAPIFieldLabel(fs.APIVersion, fs.FieldPath, "")
	if err != nil {
		return "", err
	}
	switch internalFieldPath {
	case "spec.nodeName":
		return pod.Spec.NodeName, nil
	case "spec.serviceAccountName":
		return pod.Spec.ServiceAccountName, nil

	}
	return fieldpath.ExtractFieldPathAsString(pod, internalFieldPath)
}
