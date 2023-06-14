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

package prober

import (
	"net/http"
	"reflect"
	"testing"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestFormatURL(t *testing.T) {
	testCases := []struct {
		scheme string
		host   string
		port   int
		path   string
		result string
	}{
		{"http", "localhost", 93, "", "http://localhost:93"},
		{"https", "localhost", 93, "/path", "https://localhost:93/path"},
		{"http", "localhost", 93, "?foo", "http://localhost:93?foo"},
		{"https", "localhost", 93, "/path?bar", "https://localhost:93/path?bar"},
	}
	for _, test := range testCases {
		url := formatURL(test.scheme, test.host, test.port, test.path)
		if url.String() != test.result {
			t.Errorf("Expected %s, got %s", test.result, url.String())
		}
	}
}

func TestFindPortByName(t *testing.T) {
	container := v1.Container{
		Ports: []v1.ContainerPort{
			{
				Name:          "foo",
				ContainerPort: 8080,
			},
			{
				Name:          "bar",
				ContainerPort: 9000,
			},
		},
	}
	want := 8080
	got, err := findPortByName(container, "foo")
	if got != want || err != nil {
		t.Errorf("Expected %v, got %v, err: %v", want, got, err)
	}
}

func TestGetURLParts(t *testing.T) {
	testCases := []struct {
		probe *v1.HTTPGetAction
		ok    bool
		host  string
		port  int
		path  string
	}{
		{&v1.HTTPGetAction{Host: "", Port: intstr.FromInt(-1), Path: ""}, false, "", -1, ""},
		{&v1.HTTPGetAction{Host: "", Port: intstr.FromString(""), Path: ""}, false, "", -1, ""},
		{&v1.HTTPGetAction{Host: "", Port: intstr.FromString("-1"), Path: ""}, false, "", -1, ""},
		{&v1.HTTPGetAction{Host: "", Port: intstr.FromString("not-found"), Path: ""}, false, "", -1, ""},
		{&v1.HTTPGetAction{Host: "", Port: intstr.FromString("found"), Path: ""}, true, "127.0.0.1", 93, ""},
		{&v1.HTTPGetAction{Host: "", Port: intstr.FromInt(76), Path: ""}, true, "127.0.0.1", 76, ""},
		{&v1.HTTPGetAction{Host: "", Port: intstr.FromString("118"), Path: ""}, true, "127.0.0.1", 118, ""},
		{&v1.HTTPGetAction{Host: "hostname", Port: intstr.FromInt(76), Path: "path"}, true, "hostname", 76, "path"},
	}

	for _, test := range testCases {
		state := v1.PodStatus{PodIP: "127.0.0.1"}
		container := v1.Container{
			Ports: []v1.ContainerPort{{Name: "found", ContainerPort: 93}},
			LivenessProbe: &v1.Probe{
				ProbeHandler: v1.ProbeHandler{
					HTTPGet: test.probe,
				},
			},
		}

		scheme := test.probe.Scheme
		if scheme == "" {
			scheme = v1.URISchemeHTTP
		}
		host := test.probe.Host
		if host == "" {
			host = state.PodIP
		}
		port, err := extractPort(test.probe.Port, container)
		if test.ok && err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		path := test.probe.Path

		if !test.ok && err == nil {
			t.Errorf("Expected error for %+v, got %s%s:%d/%s", test, scheme, host, port, path)
		}
		if test.ok {
			if host != test.host || port != test.port || path != test.path {
				t.Errorf("Expected %s:%d/%s, got %s:%d/%s",
					test.host, test.port, test.path, host, port, path)
			}
		}
	}
}

func TestGetTCPAddrParts(t *testing.T) {
	testCases := []struct {
		probe *v1.TCPSocketAction
		ok    bool
		host  string
		port  int
	}{
		{&v1.TCPSocketAction{Port: intstr.FromInt(-1)}, false, "", -1},
		{&v1.TCPSocketAction{Port: intstr.FromString("")}, false, "", -1},
		{&v1.TCPSocketAction{Port: intstr.FromString("-1")}, false, "", -1},
		{&v1.TCPSocketAction{Port: intstr.FromString("not-found")}, false, "", -1},
		{&v1.TCPSocketAction{Port: intstr.FromString("found")}, true, "1.2.3.4", 93},
		{&v1.TCPSocketAction{Port: intstr.FromInt(76)}, true, "1.2.3.4", 76},
		{&v1.TCPSocketAction{Port: intstr.FromString("118")}, true, "1.2.3.4", 118},
	}

	for _, test := range testCases {
		host := "1.2.3.4"
		container := v1.Container{
			Ports: []v1.ContainerPort{{Name: "found", ContainerPort: 93}},
			LivenessProbe: &v1.Probe{
				ProbeHandler: v1.ProbeHandler{
					TCPSocket: test.probe,
				},
			},
		}
		port, err := extractPort(test.probe.Port, container)
		if !test.ok && err == nil {
			t.Errorf("Expected error for %+v, got %s:%d", test, host, port)
		}
		if test.ok && err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if test.ok {
			if host != test.host || port != test.port {
				t.Errorf("Expected %s:%d, got %s:%d", test.host, test.port, host, port)
			}
		}
	}
}

func TestHTTPHeaders(t *testing.T) {
	testCases := []struct {
		input  []v1.HTTPHeader
		output http.Header
	}{
		{[]v1.HTTPHeader{}, http.Header{}},
		{[]v1.HTTPHeader{
			{Name: "X-Muffins-Or-Cupcakes", Value: "Muffins"},
		}, http.Header{"X-Muffins-Or-Cupcakes": {"Muffins"}}},
		{[]v1.HTTPHeader{
			{Name: "X-Muffins-Or-Cupcakes", Value: "Muffins"},
			{Name: "X-Muffins-Or-Plumcakes", Value: "Muffins!"},
		}, http.Header{"X-Muffins-Or-Cupcakes": {"Muffins"},
			"X-Muffins-Or-Plumcakes": {"Muffins!"}}},
		{[]v1.HTTPHeader{
			{Name: "X-Muffins-Or-Cupcakes", Value: "Muffins"},
			{Name: "X-Muffins-Or-Cupcakes", Value: "Cupcakes, too"},
		}, http.Header{"X-Muffins-Or-Cupcakes": {"Muffins", "Cupcakes, too"}}},
	}
	for _, test := range testCases {
		headers := buildHeader(test.input)
		if !reflect.DeepEqual(test.output, headers) {
			t.Errorf("Expected %#v, got %#v", test.output, headers)
		}
	}
}
