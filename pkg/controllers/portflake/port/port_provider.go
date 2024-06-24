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

package port

import (
	"fmt"
	"sync"
	"time"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

const (
	DefaultMaxNotVerifiedPortAge = 30
)

type BaseInfo struct {
	owner string // pod name
}

type VerifyInfo struct {
	allocateTime int64
}

type Provider struct {
	namespace     string
	mutex         sync.RWMutex
	ports         map[int]*BaseInfo   // port->BaseInfo
	portsToVerify map[int]*VerifyInfo // port->VerifyInfo
	segments      *SegmentList
}

func newProvider(ns string) *Provider {
	pp := &Provider{
		namespace:     ns,
		ports:         map[int]*BaseInfo{},
		portsToVerify: map[int]*VerifyInfo{},

		segments: NewSegmentList([]Segment{
			NewSegment(20000, 32767),
		}),
	}

	return pp
}

func (pp *Provider) addPort(port int) {
	pp.ports[port] = &BaseInfo{}
	pp.portsToVerify[port] = &VerifyInfo{
		allocateTime: time.Now().Unix(),
	}
}

func (pp *Provider) Allocate(count int) ([]int32, error) {
	nlog.Debugf("Allocate port, count=%v, namespace=%v", count, pp.namespace)
	if count <= 0 {
		return []int32{}, nil
	}

	choosePorts := make(map[int]bool, count)

	checkPortValid := func(port int) bool {
		_, ok := pp.ports[port]
		if ok {
			return false
		}

		_, ok = choosePorts[port]
		if ok {
			return false
		}

		return true
	}

	pp.mutex.Lock()
	defer pp.mutex.Unlock()

	// choose first port
	choosePort, choosePortIdx, ok := ChoosePortByRandom(pp.segments, checkPortValid, 10)
	if !ok {
		choosePort, choosePortIdx, ok = ChoosePortByIteration(pp.segments, checkPortValid, choosePort, choosePortIdx)
		if !ok {
			return nil, fmt.Errorf("no available port to allocate, allocated port count=%v, namespace=%v", len(pp.ports), pp.namespace)
		}
	}
	choosePorts[choosePort] = true

	// choose rest of ports
	for index := 1; index < count; index++ {
		choosePort, choosePortIdx, ok = ChoosePortByIteration(pp.segments, checkPortValid, choosePort, choosePortIdx)
		if !ok {
			return nil, fmt.Errorf("no available port to allocate, allocated port count=%v, namespace=%v", len(pp.ports), pp.namespace)
		}

		choosePorts[choosePort] = true
	}

	var retPorts []int32

	for port := range choosePorts {
		// add port to cache
		pp.addPort(port)

		retPorts = append(retPorts, int32(port))
	}

	nlog.Debugf("Allocate port succeed, ports=%+v, namespace=%v", retPorts, pp.namespace)

	return retPorts, nil
}

func (pp *Provider) addPortIndeed(owner string, port int) error {
	// add or update port in pp.ports
	info, ok := pp.ports[port]
	if !ok {
		info = &BaseInfo{owner: owner}
		pp.ports[port] = info
	} else if info.owner != "" && info.owner != owner {
		return fmt.Errorf("port conflict, current owner=%v, new owner=%v, namespace=%v", info.owner, owner, pp.namespace)
	} else {
		info.owner = owner
	}

	// delete port in pp.portsToVerify
	delete(pp.portsToVerify, port)

	return nil
}

func (pp *Provider) AddIndeed(owner string, ports []int) {
	nlog.Infof("Add ports indeed, owner=%v, ports=%+v, namespace=%v", owner, ports, pp.namespace)

	pp.mutex.Lock()
	defer pp.mutex.Unlock()

	for _, pt := range ports {
		if err := pp.addPortIndeed(owner, pt); err != nil {
			nlog.Errorf("Add port indeed failed: %v", err)
		}
	}
}

func (pp *Provider) deletePortIndeed(owner string, port int) error {
	BaseInfo, ok := pp.ports[port]
	if !ok {
		return fmt.Errorf("not found port info, port=%v, namespace=%v", port, pp.namespace)
	}

	if BaseInfo.owner != owner {
		return fmt.Errorf("unmatched owner, expected=%v, given=%v, namespace=%v", BaseInfo.owner, owner, pp.namespace)
	}

	delete(pp.ports, port)

	return nil
}

func (pp *Provider) DeleteIndeed(owner string, ports []int) {
	nlog.Infof("Delete ports indeed, owner=%v, ports=%+v, namespace=%v", owner, ports, pp.namespace)

	pp.mutex.Lock()
	defer pp.mutex.Unlock()

	for _, pt := range ports {
		if err := pp.deletePortIndeed(owner, pt); err != nil {
			nlog.Errorf("Delete port indeed failed: %v", err)
		}
	}
}

func (pp *Provider) CheckNotVerified() {
	nlog.Debugf("Verify port, namespace=%v", pp.namespace)

	curTime := time.Now().Unix()

	pp.mutex.Lock()
	defer pp.mutex.Unlock()

	for pt, verifyInfo := range pp.portsToVerify {
		if curTime-verifyInfo.allocateTime >= DefaultMaxNotVerifiedPortAge {
			nlog.Infof("Delete port %v since the maximum period of not verified port is exceeded, "+
				"allocate time=%v, namespace=%v", pt, time.Unix(verifyInfo.allocateTime, 0), pp.namespace)
			delete(pp.ports, pt)
			delete(pp.portsToVerify, pt)
		}
	}
}

func (pp *Provider) PortCount() int {
	pp.mutex.RLock()
	defer pp.mutex.RUnlock()

	return len(pp.ports)
}

func (pp *Provider) PortToVerifyCount() int {
	pp.mutex.RLock()
	defer pp.mutex.RUnlock()

	return len(pp.portsToVerify)
}

type ProviderManager struct {
	providers sync.Map // namespace->Provider
}

var pm ProviderManager

func GetPortProvider(namespace string) *Provider {
	portProvider, ok := pm.providers.Load(namespace)
	if ok {
		return portProvider.(*Provider)
	}

	defaultProvider := newProvider(namespace)
	portProvider, _ = pm.providers.LoadOrStore(namespace, defaultProvider)

	return portProvider.(*Provider)
}

func ScanPortProviders() {
	pm.providers.Range(func(key, value interface{}) bool {
		portProvider := value.(*Provider)
		portProvider.CheckNotVerified()
		return true
	})
}

func AllocatePort(namespacedCount map[string]int) (map[string][]int32, error) {
	ret := map[string][]int32{}

	for ns, count := range namespacedCount {
		provider := GetPortProvider(ns)
		ports, err := provider.Allocate(count)
		if err != nil {
			return nil, fmt.Errorf("failed to allocate ports for namespace %q, detail-> %v", ns, err)
		}

		ret[ns] = ports
	}

	return ret, nil
}
