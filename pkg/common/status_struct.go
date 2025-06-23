package common

import (
	"fmt"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

var instance *NodeStatusManager
var once sync.Once

type LocalNodeStatus struct {
	Name               string      `json:"name"`
	DomainName         string      `json:"domainName"`
	Status             string      `json:"status"`
	LastHeartbeatTime  metav1.Time `json:"lastHeartbeatTime,omitempty"`
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
	UnreadyReason      string      `json:"unreadyReason,omitempty"`
	TotalCPURequest    int64       `json:"totalCPURequest"`
	TotalMemRequest    int64       `json:"totalMemRequest"`
}

type NodeStatusManager struct {
	statuses []LocalNodeStatus
	lock     sync.RWMutex
}

func NewNodeStatusManager() *NodeStatusManager {
	once.Do(func() {
		nlog.Infof("create nsm instance")
		instance = &NodeStatusManager{}
	})
	return instance
}

func (m *NodeStatusManager) ReplaceAll(statuses []LocalNodeStatus) {
	m.statuses = statuses
}

func (m *NodeStatusManager) GetAll() []LocalNodeStatus {
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.statuses
}

func (m *NodeStatusManager) UpdateStatus(newStatus LocalNodeStatus) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	for i, status := range m.statuses {
		if status.Name == newStatus.Name {
			nlog.Debugf("start m statuses is : %+v", m.statuses[i])
			m.statuses[i] = newStatus
			nlog.Debugf("end m statuses is : %+v", m.statuses[i])
			return nil
		}
	}

	m.statuses = append(m.statuses, newStatus)
	return nil
}

func (m *NodeStatusManager) AddPodResources(nodeName string, cpu, mem int64) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	for i := range m.statuses {
		if m.statuses[i].Name == nodeName {
			nlog.Debugf("start AddPodResources m.statuses %s tcr is %d tmr is %d", m.statuses[i].Name, m.statuses[i].TotalCPURequest, m.statuses[i].TotalMemRequest)
			m.statuses[i].TotalCPURequest += cpu
			m.statuses[i].TotalMemRequest += mem
			nlog.Debugf("end m.statuses %s tcr is %d tmr is %d", m.statuses[i].Name, m.statuses[i].TotalCPURequest, m.statuses[i].TotalMemRequest)
			return nil
		}
	}
	return fmt.Errorf("node %s not found", nodeName)
}

func (m *NodeStatusManager) RemovePodResources(nodeName string, cpu, mem int64) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	for i := range m.statuses {
		if m.statuses[i].Name == nodeName {
			nlog.Debugf("start RemovePodResources m.statuses %s tcr is %d tmr is %d", m.statuses[i].Name, m.statuses[i].TotalCPURequest, m.statuses[i].TotalMemRequest)
			m.statuses[i].TotalCPURequest -= cpu
			m.statuses[i].TotalMemRequest -= mem
			nlog.Debugf("end RemovePodResources m.statuses %s tcr is %d tmr is %d", m.statuses[i].Name, m.statuses[i].TotalCPURequest, m.statuses[i].TotalMemRequest)
			return nil
		}
	}
	return fmt.Errorf("node %s not found", nodeName)
}
