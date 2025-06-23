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
		nlog.Infof("first time to create nsm instance")
		instance = &NodeStatusManager{}
	})
	nlog.Infof("get nsm instance")
	return instance
}

func (m *NodeStatusManager) ReplaceAll(statuses []LocalNodeStatus) {
	nlog.Infof("start ReplaceAll m statuses is %v", m.statuses)
	nlog.Infof("start ReplaceAll statuses is %v", statuses)
	m.statuses = statuses
	nlog.Infof("end ReplaceAll m statuses is %v", m.statuses)
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
			nlog.Infof("start m statuses is : %+v", m.statuses[i])
			m.statuses[i] = newStatus
			nlog.Infof("end m statuses is : %+v", m.statuses[i])
			return nil
		}
	}

	// 如果节点不存在则追加新记录
	nlog.Infof("start statuses length: %d", len(m.statuses))
	m.statuses = append(m.statuses, newStatus)
	nlog.Infof("end statuses length: %d", len(m.statuses))
	return nil
}

func (m *NodeStatusManager) AddPodResources(nodeName string, cpu, mem int64) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	for i := range m.statuses {
		if m.statuses[i].Name == nodeName {
			nlog.Infof("start AddPodResources m.statuses %s tcr is %d tmr is %d", m.statuses[i].Name, m.statuses[i].TotalCPURequest, m.statuses[i].TotalMemRequest)
			m.statuses[i].TotalCPURequest += cpu
			m.statuses[i].TotalMemRequest += mem
			nlog.Infof("end m.statuses %s tcr is %d tmr is %d", m.statuses[i].Name, m.statuses[i].TotalCPURequest, m.statuses[i].TotalMemRequest)
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
			nlog.Infof("start RemovePodResources m.statuses %s tcr is %d tmr is %d", m.statuses[i].Name, m.statuses[i].TotalCPURequest, m.statuses[i].TotalMemRequest)
			m.statuses[i].TotalCPURequest -= cpu
			m.statuses[i].TotalMemRequest -= mem
			nlog.Infof("end RemovePodResources m.statuses %s tcr is %d tmr is %d", m.statuses[i].Name, m.statuses[i].TotalCPURequest, m.statuses[i].TotalMemRequest)
			return nil
		}
	}
	return fmt.Errorf("node %s not found", nodeName)
}
