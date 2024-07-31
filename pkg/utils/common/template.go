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
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"text/template"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

func RenderConfig(configPathTmpl, configPath string, s interface{}) error {
	configTmpl, err := template.ParseFiles(configPathTmpl)
	if err != nil {
		return err
	}
	var configContent bytes.Buffer
	if err := configTmpl.Execute(&configContent, s); err != nil {
		return err
	}

	f, err := os.Create(configPath)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(configContent.String())
	if err != nil {
		return err
	}
	return nil
}

func RenderRuntimeObject(configPathTmpl string, object runtime.Object, input interface{}) error {
	template, err := template.ParseFiles(configPathTmpl)
	if err != nil {
		return err
	}

	var buffer bytes.Buffer
	if err := template.Execute(&buffer, input); err != nil {
		return err
	}

	if _, _, err = Decode(buffer.Bytes(), nil, object); err != nil {
		return err
	}
	return nil
}

func QueryByFields(value any, query string) (res any) {
	defer func() {
		if r := recover(); r != nil {
			res = nil
		}
	}()

	toString := func(v any) string {
		if _, ok := v.(string); ok {
			return v.(string)
		}

		s, _ := json.Marshal(v)
		return string(s)
	}

	// eg: .v1.v2
	byName := func(value any, field string) any {
		switch reflect.TypeOf(value).Kind() {
		case reflect.Map:
			v := reflect.ValueOf(value).MapIndex(reflect.ValueOf(field))
			if !v.IsValid() || v.IsZero() {
				return nil
			}
			return v.Interface()
		default:
		}
		return nil
	}

	// eg: .v1[0]
	byIdx := func(value any, index string) any {
		switch reflect.TypeOf(value).Kind() {
		case reflect.Array, reflect.Slice:
			idx, err := strconv.Atoi(index)
			if err != nil {
				return nil
			}

			// invalidate index
			arrlength := reflect.ValueOf(value).Len()
			if idx >= arrlength || idx <= -arrlength {
				return nil
			}
			// Support negative indices
			idx = (idx + arrlength) % arrlength
			v := reflect.ValueOf(value).Index(idx)
			if !v.IsValid() || v.IsZero() {
				return nil
			}
			return v.Interface()
		default:
		}
		return nil
	}

	// eg: .v1[key=value]
	byFilter := func(value any, key, match string) any {
		switch reflect.TypeOf(value).Kind() {
		case reflect.Array, reflect.Slice:
			arr := reflect.ValueOf(value)
			for i := 0; i < arr.Len(); i++ {
				item := arr.Index(i).Interface()
				switch reflect.TypeOf(item).Kind() {
				case reflect.Map:
					v := reflect.ValueOf(item).MapIndex(reflect.ValueOf(key))
					if v.IsValid() && !v.IsZero() && toString(v.Interface()) == match {
						return item
					}
				default:
				}

			}
		default:
			nlog.Infof("type=%s", reflect.TypeOf(value).Kind().String())
		}
		return nil

	}

	fields := strings.Split(query, ".")
	for _, field := range fields {
		if value == nil {
			return nil
		}

		if field == "" { // leading or empty variable
			continue
		}

		if strings.Contains(field, "[") {
			// match: val[1] or val[key=value]
			re := regexp.MustCompile(`^(.+)\[(.+)\]$`)
			kv := re.FindStringSubmatch(field)
			if len(kv) != 3 {
				return nil
			}

			// find value
			tmpval := byName(value, kv[1])
			if tmpval == nil {
				return nil
			}

			if strings.Contains(kv[2], "=") {
				matcher := strings.SplitN(kv[2], "=", 2)
				value = byFilter(tmpval, matcher[0], matcher[1])
			} else {
				value = byIdx(tmpval, kv[2])
			}
		} else {
			value = byName(value, field)

		}
	}
	return value
}

var Decode func(data []byte, defaults *schema.GroupVersionKind, into runtime.Object) (runtime.Object, *schema.GroupVersionKind, error)

func init() {
	sch := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(sch)
	_ = apiextv1beta1.AddToScheme(sch)
	_ = appsv1.AddToScheme(sch)
	_ = corev1.AddToScheme(sch)
	Decode = serializer.NewCodecFactory(sch).UniversalDeserializer().Decode
}
