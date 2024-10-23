// Copyright 2023 Ant Group Co., Ltd.
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

package kii

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	oci "github.com/opencontainers/image-spec/specs-go/v1"
	"k8s.io/kubernetes/pkg/util/parsers"
)

type MountType string

const (
	// Plain unpacks all layers to same directory, override files with same filename
	Plain MountType = "plain"
)

type ImageType string

const (
	ImageTypeStandard = "Standard" // Standard OCI image
	ImageTypeBuiltIn  = "BuiltIn"  // Built in image without layers
)

type ImageName struct {
	Image string
	Repo  string
	Tag   string
}

func NewImageName(image string) (*ImageName, error) {
	repoToPull, tag, _, err := parsers.ParseImageName(image)
	if err != nil {
		return nil, fmt.Errorf("failed to parse image name %q, detail-> %v", image, err)
	}

	return &ImageName{
		Image: image,
		Repo:  repoToPull,
		Tag:   tag,
	}, nil
}

func ParseImageNameFromPath(path string) (*ImageName, error) {
	tag := filepath.Base(path)
	repo := filepath.Dir(path)
	image := fmt.Sprintf("%s:%s", repo, tag)
	return &ImageName{
		Image: image,
		Repo:  repo,
		Tag:   tag,
	}, nil
}

// Manifest provides information about image.
type Manifest struct {
	// Created is the combined date and time at which the image was created,
	// formatted as defined by RFC 3339, section 5.6.
	Created *time.Time `json:"created,omitempty"`

	// Author defines the name and/or email address of the person or entity which created
	// and is responsible for maintaining the image.
	Author string `json:"author,omitempty,omitempty"`

	// Architecture is the CPU architecture which the binaries in this image are built to run on.
	Architecture string `json:"architecture,omitempty"`

	// OS is the name of the operating system which the image is built to run on.
	OS string `json:"os,omitempty"`

	// Config defines the execution parameters which should be used as a base when
	// running a container using the image.
	Config oci.ImageConfig `json:"config,omitempty"`

	// total size of all layer
	Size int64 `json:"size"`

	// Layers is an indexed list of layers referenced by the manifest.
	// order: lowest -> highest
	Layers []string `json:"layers"`
	ID     string   `json:"id"`

	// Type defines the type of the image. Default is Standard.
	Type ImageType `json:"type"`
}

func FormatImageID(id string) string {
	if strings.HasPrefix(id, "sha256:") {
		return id
	}

	return fmt.Sprintf("sha256:%s", id)
}
