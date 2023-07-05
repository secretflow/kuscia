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

package prober

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"golang.org/x/net/context"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"
	"k8s.io/kubernetes/pkg/kubelet/events"
	"k8s.io/kubernetes/pkg/probe"
	grpcprobe "k8s.io/kubernetes/pkg/probe/grpc"
	httpprobe "k8s.io/kubernetes/pkg/probe/http"
	tcpprobe "k8s.io/kubernetes/pkg/probe/tcp"

	pkgcontainer "github.com/secretflow/kuscia/pkg/agent/container"
	"github.com/secretflow/kuscia/pkg/utils/nlog"

	"github.com/secretflow/kuscia/pkg/agent/prober/results"
	"github.com/secretflow/kuscia/pkg/agent/utils/format"
)

const maxProbeRetries = 3

// Prober helps to check the liveness/readiness/startup of a container.
type prober struct {
	// probe types needs different httpprobe instances so they don't
	// share a connection pool which can cause collisions to the
	// same host:port and transient failures. See #49740.
	http httpprobe.Prober
	tcp  tcpprobe.Prober
	grpc grpcprobe.Prober

	recorder record.EventRecorder
}

// NewProber creates a Prober, it takes a command runner and
// several container info managers.
func newProber(
	recorder record.EventRecorder) *prober {

	const followNonLocalRedirects = false
	return &prober{
		http:     httpprobe.New(followNonLocalRedirects),
		tcp:      tcpprobe.New(),
		grpc:     grpcprobe.New(),
		recorder: recorder,
	}
}

// recordContainerEvent should be used by the prober for all container related events.
func (pb *prober) recordContainerEvent(pod *v1.Pod, container *v1.Container, eventType, reason, message string, args ...interface{}) {
	ref, err := pkgcontainer.GenerateContainerRef(pod, container)
	if err != nil {
		nlog.Errorf("Can't make a ref to pod %q and container %q", format.Pod(pod), container.Name)
		return
	}
	pb.recorder.Eventf(ref, eventType, reason, message, args...)
}

// probe probes the container.
func (pb *prober) probe(ctx context.Context, probeType probeType, pod *v1.Pod, status v1.PodStatus, container v1.Container, containerID pkgcontainer.CtrID) (results.Result, error) {
	var probeSpec *v1.Probe
	switch probeType {
	case readiness:
		probeSpec = container.ReadinessProbe
	case liveness:
		probeSpec = container.LivenessProbe
	case startup:
		probeSpec = container.StartupProbe
	default:
		return results.Failure, fmt.Errorf("unknown probe type: %q", probeType)
	}

	if probeSpec == nil {
		nlog.Debugf("Probe is nil, probeType=%v, pod=%v, containerName=%v", probeType, format.Pod(pod), container.Name)
		return results.Success, nil
	}

	result, output, err := pb.runProbeWithRetries(ctx, probeType, probeSpec, pod, status, container, containerID, maxProbeRetries)
	if err != nil || (result != probe.Success && result != probe.Warning) {
		// Probe failed in one way or another.
		if err != nil {
			nlog.Errorf("Probe errored, probeType=%v, pod=%v, containerName=%v", probeType, format.Pod(pod), container.Name)
			pb.recordContainerEvent(pod, &container, v1.EventTypeWarning, events.ContainerUnhealthy, "%s probe errored: %v", probeType, err)
		} else { // result != probe.Success
			nlog.Errorf("Probe failed, probeType=%v, pod=%v, containerName=%v, probeResult=%v, output=%v", probeType, format.Pod(pod), container.Name, result, output)
			pb.recordContainerEvent(pod, &container, v1.EventTypeWarning, events.ContainerUnhealthy, "%s probe failed: %s", probeType, output)
		}
		return results.Failure, err
	}
	if result == probe.Warning {
		pb.recordContainerEvent(pod, &container, v1.EventTypeWarning, events.ContainerProbeWarning, "%s probe warning: %s", probeType, output)
		nlog.Warnf("Probe succeeded with a warning, probeType=%v, pod=%v, containerName=%v, output=%v", probeType, format.Pod(pod), container.Name, output)
	} else {
		nlog.Infof("Probe succeeded, probeType=%v, pod=%v, containerName=%v", probeType, format.Pod(pod), container.Name)
	}
	return results.Success, nil
}

// runProbeWithRetries tries to probe the container in a finite loop, it returns the last result
// if it never succeeds.
func (pb *prober) runProbeWithRetries(ctx context.Context, probeType probeType, p *v1.Probe, pod *v1.Pod, status v1.PodStatus, container v1.Container, containerID pkgcontainer.CtrID, retries int) (probe.Result, string, error) {
	var err error
	var result probe.Result
	var output string
	for i := 0; i < retries; i++ {
		result, output, err = pb.runProbe(ctx, probeType, p, pod, status, container, containerID)
		if err == nil {
			return result, output, nil
		}
	}
	return result, output, err
}

// buildHeaderMap takes a list of HTTPHeader <name, value> string
// pairs and returns a populated string->[]string http.Header map.
func buildHeader(headerList []v1.HTTPHeader) http.Header {
	headers := make(http.Header)
	for _, header := range headerList {
		headers[header.Name] = append(headers[header.Name], header.Value)
	}
	return headers
}

func (pb *prober) runProbe(ctx context.Context, probeType probeType, p *v1.Probe, pod *v1.Pod, status v1.PodStatus, container v1.Container, containerID pkgcontainer.CtrID) (probe.Result, string, error) {
	timeout := time.Duration(p.TimeoutSeconds) * time.Second
	if p.HTTPGet != nil {
		req, err := httpprobe.NewRequestForHTTPGetAction(p.HTTPGet, &container, status.PodIP, "probe")
		if err != nil {
			return probe.Unknown, "", err
		}
		nlog.Debugf("HTTP-Probe, scheme=%v, host=%v, port=%v, path=%v, timeout=%v, headers=%+v", req.URL.Scheme, req.URL.Hostname(), req.URL.Port(), req.URL.Path, timeout, p.HTTPGet.HTTPHeaders)
		return pb.http.Probe(req, timeout)
	}

	if p.TCPSocket != nil {
		port, err := extractPort(p.TCPSocket.Port, container)
		if err != nil {
			return probe.Unknown, "", err
		}
		host := p.TCPSocket.Host
		if host == "" {
			host = status.PodIP
		}
		nlog.Debugf("TCP-Probe, host=%v, port=%v, timeout=%v", host, port, timeout)
		return pb.tcp.Probe(host, port, timeout)
	}

	if p.GRPC != nil {
		host := status.PodIP
		service := ""
		if p.GRPC.Service != nil {
			service = *p.GRPC.Service
		}
		nlog.Debugf("GRPC-Probe, host=%v, service=%v, port=%v, timeout=%v", host, service, p.GRPC.Port, timeout)
		return pb.grpc.Probe(host, service, int(p.GRPC.Port), timeout)
	}

	nlog.Warnf("Failed to find probe builder for container %q", container.Name)
	return probe.Unknown, "", fmt.Errorf("missing probe handler for %s:%s", format.Pod(pod), container.Name)
}

func extractPort(param intstr.IntOrString, container v1.Container) (int, error) {
	port := -1
	var err error
	switch param.Type {
	case intstr.Int:
		port = param.IntValue()
	case intstr.String:
		if port, err = findPortByName(container, param.StrVal); err != nil {
			// Last ditch effort - maybe it was an int stored as string?
			if port, err = strconv.Atoi(param.StrVal); err != nil {
				return port, err
			}
		}
	default:
		return port, fmt.Errorf("intOrString had no kind: %+v", param)
	}
	if port > 0 && port < 65536 {
		return port, nil
	}
	return port, fmt.Errorf("invalid port number: %v", port)
}

// findPortByName is a helper function to look up a port in a container by name.
func findPortByName(container v1.Container, portName string) (int, error) {
	for _, port := range container.Ports {
		if port.Name == portName {
			return int(port.ContainerPort), nil
		}
	}
	return 0, fmt.Errorf("port %s not found", portName)
}

// formatURL formats a URL from args.  For testability.
func formatURL(scheme string, host string, port int, path string) *url.URL {
	u, err := url.Parse(path)
	// Something is busted with the path, but it's too late to reject it. Pass it along as is.
	if err != nil {
		u = &url.URL{
			Path: path,
		}
	}
	u.Scheme = scheme
	u.Host = net.JoinHostPort(host, strconv.Itoa(port))
	return u
}
