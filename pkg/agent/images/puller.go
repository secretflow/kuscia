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

package images

import (
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
	"k8s.io/kubernetes/pkg/credentialprovider"

	pkgcontainer "github.com/secretflow/kuscia/pkg/agent/container"
)

type pullResult struct {
	imageRef string
	err      error
}

type imagePuller interface {
	pullImage(pkgcontainer.ImageSpec, *credentialprovider.AuthConfig, chan<- pullResult, *runtimeapi.PodSandboxConfig)
}

var _, _ imagePuller = &parallelImagePuller{}, &serialImagePuller{}

type parallelImagePuller struct {
	imageService pkgcontainer.ImageService
}

func newParallelImagePuller(imageService pkgcontainer.ImageService) imagePuller {
	return &parallelImagePuller{imageService}
}

func (pip *parallelImagePuller) pullImage(spec pkgcontainer.ImageSpec, auth *credentialprovider.AuthConfig, pullChan chan<- pullResult, podSandboxConfig *runtimeapi.PodSandboxConfig) {
	go func() {
		imageRef, err := pip.imageService.PullImage(spec, auth, podSandboxConfig)
		pullChan <- pullResult{
			imageRef: imageRef,
			err:      err,
		}
	}()
}

// Maximum number of image pull requests than can be queued.
const maxImagePullRequests = 10

type serialImagePuller struct {
	imageService pkgcontainer.ImageService
	pullRequests chan *imagePullRequest
}

func newSerialImagePuller(imageService pkgcontainer.ImageService) imagePuller {
	imagePuller := &serialImagePuller{imageService, make(chan *imagePullRequest, maxImagePullRequests)}
	go wait.Until(imagePuller.processImagePullRequests, time.Second, wait.NeverStop)
	return imagePuller
}

type imagePullRequest struct {
	spec             pkgcontainer.ImageSpec
	auth             *credentialprovider.AuthConfig
	pullChan         chan<- pullResult
	podSandboxConfig *runtimeapi.PodSandboxConfig
}

func (sip *serialImagePuller) pullImage(spec pkgcontainer.ImageSpec, auth *credentialprovider.AuthConfig, pullChan chan<- pullResult, podSandboxConfig *runtimeapi.PodSandboxConfig) {
	sip.pullRequests <- &imagePullRequest{
		spec:             spec,
		auth:             auth,
		pullChan:         pullChan,
		podSandboxConfig: podSandboxConfig,
	}
}

func (sip *serialImagePuller) processImagePullRequests() {
	for pullRequest := range sip.pullRequests {
		imageRef, err := sip.imageService.PullImage(pullRequest.spec, pullRequest.auth, pullRequest.podSandboxConfig)
		pullRequest.pullChan <- pullResult{
			imageRef: imageRef,
			err:      err,
		}
	}
}
