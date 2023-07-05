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

package common

import (
	"fmt"
	"reflect"

	k8sv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	pbv1alpha1 "github.com/secretflow/kuscia/proto/api/v1alpha1"
)

func Convert2PbPartition(partition *k8sv1alpha1.Partition) (pbPartition *pbv1alpha1.Partition) {
	if partition == nil {
		return nil
	}
	cols := make([]*pbv1alpha1.DataColumn, len(partition.Fields))
	for i, v := range partition.Fields {
		cols[i] = &pbv1alpha1.DataColumn{
			Name:    v.Name,
			Type:    v.Type,
			Comment: v.Comment,
		}
	}
	return &pbv1alpha1.Partition{
		Type:   partition.Type,
		Fields: cols,
	}
}

func Convert2PbColumn(cols []k8sv1alpha1.DataColumn) []*pbv1alpha1.DataColumn {
	colsV1 := make([]*pbv1alpha1.DataColumn, len(cols))
	for i, v := range cols {
		colsV1[i] = &pbv1alpha1.DataColumn{
			Name:    v.Name,
			Type:    v.Type,
			Comment: v.Comment,
		}
	}
	return colsV1
}

func Convert2KubePartition(partition *pbv1alpha1.Partition) *k8sv1alpha1.Partition {
	if partition == nil {
		return nil
	}
	cols := make([]k8sv1alpha1.DataColumn, len(partition.Fields))
	for i, v := range partition.Fields {
		cols[i] = k8sv1alpha1.DataColumn{
			Name:    v.Name,
			Type:    v.Type,
			Comment: v.Comment,
		}
	}
	return &k8sv1alpha1.Partition{
		Type:   partition.Type,
		Fields: cols,
	}
}

func Convert2KubeColumn(cols []*pbv1alpha1.DataColumn) []k8sv1alpha1.DataColumn {
	colsV1 := make([]k8sv1alpha1.DataColumn, len(cols))
	for i, v := range cols {
		colsV1[i] = k8sv1alpha1.DataColumn{
			Name:    v.Name,
			Type:    v.Type,
			Comment: v.Comment,
		}
	}
	return colsV1
}

func CopySameMemberTypeStruct(dst, src interface{}) error {
	dVal := reflect.ValueOf(dst).Elem()
	sVal := reflect.ValueOf(src).Elem()

	if dVal.NumField() != sVal.NumField() {
		return fmt.Errorf("number of fields in dst and src struct do not match")
	}

	for i := 0; i < dVal.NumField(); i++ {
		dField := dVal.Field(i)
		sField := sVal.Field(i)

		if dField.Type() != sField.Type() {
			return fmt.Errorf("type of field %v does not match", dVal.Type().Field(i).Name)
		}

		dField.Set(sField)
	}

	return nil
}
