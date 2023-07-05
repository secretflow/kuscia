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

package kuberuntime

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"k8s.io/apimachinery/pkg/util/sets"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
	"k8s.io/kubernetes/pkg/credentialprovider"

	pkgcontainer "github.com/secretflow/kuscia/pkg/agent/container"
)

func TestPullImage(t *testing.T) {
	ctx := context.Background()
	_, _, fakeManager, err := createTestRuntimeManager()
	assert.NoError(t, err)

	imageRef, err := fakeManager.PullImage(ctx, pkgcontainer.ImageSpec{Image: "busybox"}, nil, nil)
	assert.NoError(t, err)
	assert.Equal(t, "busybox", imageRef)

	images, err := fakeManager.ListImages(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(images))
	assert.Equal(t, images[0].RepoTags, []string{"busybox"})
}

func TestPullImageWithError(t *testing.T) {
	ctx := context.Background()
	_, fakeImageService, fakeManager, err := createTestRuntimeManager()
	assert.NoError(t, err)

	fakeImageService.InjectError("PullImage", fmt.Errorf("test-error"))
	imageRef, err := fakeManager.PullImage(ctx, pkgcontainer.ImageSpec{Image: "busybox"}, nil, nil)
	assert.Error(t, err)
	assert.Equal(t, "", imageRef)

	images, err := fakeManager.ListImages(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(images))
}

func TestListImages(t *testing.T) {
	ctx := context.Background()
	_, fakeImageService, fakeManager, err := createTestRuntimeManager()
	assert.NoError(t, err)

	images := []string{"1111", "2222", "3333"}
	expected := sets.NewString(images...)
	fakeImageService.SetFakeImages(images)

	actualImages, err := fakeManager.ListImages(ctx)
	assert.NoError(t, err)
	actual := sets.NewString()
	for _, i := range actualImages {
		actual.Insert(i.ID)
	}

	assert.Equal(t, expected.List(), actual.List())
}

func TestListImagesWithError(t *testing.T) {
	ctx := context.Background()
	_, fakeImageService, fakeManager, err := createTestRuntimeManager()
	assert.NoError(t, err)

	fakeImageService.InjectError("ListImages", fmt.Errorf("test-failure"))

	actualImages, err := fakeManager.ListImages(ctx)
	assert.Error(t, err)
	assert.Nil(t, actualImages)
}

func TestGetImageRef(t *testing.T) {
	ctx := context.Background()
	_, fakeImageService, fakeManager, err := createTestRuntimeManager()
	assert.NoError(t, err)

	image := "busybox"
	fakeImageService.SetFakeImages([]string{image})
	imageRef, err := fakeManager.GetImageRef(ctx, pkgcontainer.ImageSpec{Image: image})
	assert.NoError(t, err)
	assert.Equal(t, image, imageRef)
}

func TestGetImageRefImageNotAvailableLocally(t *testing.T) {
	ctx := context.Background()
	_, _, fakeManager, err := createTestRuntimeManager()
	assert.NoError(t, err)

	image := "busybox"

	imageRef, err := fakeManager.GetImageRef(ctx, pkgcontainer.ImageSpec{Image: image})
	assert.NoError(t, err)

	imageNotAvailableLocallyRef := ""
	assert.Equal(t, imageNotAvailableLocallyRef, imageRef)
}

func TestGetImageRefWithError(t *testing.T) {
	ctx := context.Background()
	_, fakeImageService, fakeManager, err := createTestRuntimeManager()
	assert.NoError(t, err)

	image := "busybox"

	fakeImageService.InjectError("ImageStatus", fmt.Errorf("test-error"))

	imageRef, err := fakeManager.GetImageRef(ctx, pkgcontainer.ImageSpec{Image: image})
	assert.Error(t, err)
	assert.Equal(t, "", imageRef)
}

func TestRemoveImage(t *testing.T) {
	ctx := context.Background()
	_, fakeImageService, fakeManager, err := createTestRuntimeManager()
	assert.NoError(t, err)

	_, err = fakeManager.PullImage(ctx, pkgcontainer.ImageSpec{Image: "busybox"}, nil, nil)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(fakeImageService.Images))

	err = fakeManager.RemoveImage(ctx, pkgcontainer.ImageSpec{Image: "busybox"})
	assert.NoError(t, err)
	assert.Equal(t, 0, len(fakeImageService.Images))
}

func TestRemoveImageNoOpIfImageNotLocal(t *testing.T) {
	ctx := context.Background()
	_, _, fakeManager, err := createTestRuntimeManager()
	assert.NoError(t, err)

	err = fakeManager.RemoveImage(ctx, pkgcontainer.ImageSpec{Image: "busybox"})
	assert.NoError(t, err)
}

func TestRemoveImageWithError(t *testing.T) {
	ctx := context.Background()
	_, fakeImageService, fakeManager, err := createTestRuntimeManager()
	assert.NoError(t, err)

	_, err = fakeManager.PullImage(ctx, pkgcontainer.ImageSpec{Image: "busybox"}, nil, nil)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(fakeImageService.Images))

	fakeImageService.InjectError("RemoveImage", fmt.Errorf("test-failure"))

	err = fakeManager.RemoveImage(ctx, pkgcontainer.ImageSpec{Image: "busybox"})
	assert.Error(t, err)
	assert.Equal(t, 1, len(fakeImageService.Images))
}

func TestImageStats(t *testing.T) {
	ctx := context.Background()
	_, fakeImageService, fakeManager, err := createTestRuntimeManager()
	assert.NoError(t, err)

	const imageSize = 64
	fakeImageService.SetFakeImageSize(imageSize)
	images := []string{"1111", "2222", "3333"}
	fakeImageService.SetFakeImages(images)

	actualStats, err := fakeManager.ImageStats(ctx)
	assert.NoError(t, err)
	expectedStats := &pkgcontainer.ImageStats{TotalStorageBytes: imageSize * uint64(len(images))}
	assert.Equal(t, expectedStats, actualStats)
}

func TestImageStatsWithError(t *testing.T) {
	ctx := context.Background()
	_, fakeImageService, fakeManager, err := createTestRuntimeManager()
	assert.NoError(t, err)

	fakeImageService.InjectError("ListImages", fmt.Errorf("test-failure"))

	actualImageStats, err := fakeManager.ImageStats(ctx)
	assert.Error(t, err)
	assert.Nil(t, actualImageStats)
}

func TestPullWithSecrets(t *testing.T) {
	ctx := context.Background()
	tests := map[string]struct {
		imageName    string
		auth         *credentialprovider.AuthConfig
		expectedAuth *runtimeapi.AuthConfig
	}{
		"no auth": {
			"ubuntu",
			&credentialprovider.AuthConfig{},
			nil,
		},
		"carried auth": {
			"ubuntu",
			&credentialprovider.AuthConfig{
				Username: "aaa",
				Password: "bbb",
			},
			&runtimeapi.AuthConfig{Username: "aaa", Password: "bbb"},
		},
	}
	for description, test := range tests {
		_, fakeImageService, fakeManager, err := customTestRuntimeManager()
		require.NoError(t, err)

		_, err = fakeManager.PullImage(ctx, pkgcontainer.ImageSpec{Image: test.imageName}, test.auth, nil)
		require.NoError(t, err)
		fakeImageService.AssertImagePulledWithAuth(t, &runtimeapi.ImageSpec{Image: test.imageName, Annotations: make(map[string]string)}, test.expectedAuth, description)
	}
}

func TestPullThenListWithAnnotations(t *testing.T) {
	ctx := context.Background()
	_, _, fakeManager, err := createTestRuntimeManager()
	assert.NoError(t, err)

	imageSpec := pkgcontainer.ImageSpec{
		Image: "12345",
		Annotations: []pkgcontainer.Annotation{
			{Name: "kubernetes.io/runtimehandler", Value: "handler_name"},
		},
	}

	_, err = fakeManager.PullImage(ctx, imageSpec, nil, nil)
	assert.NoError(t, err)

	images, err := fakeManager.ListImages(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(images))
	assert.Equal(t, images[0].Spec, imageSpec)
}
