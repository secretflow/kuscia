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
	statuses map[string]LocalNodeStatus
	lock     sync.RWMutex
}

func NewNodeStatusManager() *NodeStatusManager {
	once.Do(func() {
		nlog.Infof("create nsm instance")
		instance = &NodeStatusManager{}
	})
	return instance
}

func (m *NodeStatusManager) ReplaceAll(statuses map[string]LocalNodeStatus) {
	m.statuses = statuses
}

func (m *NodeStatusManager) GetAll() map[string]LocalNodeStatus {
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.statuses
}

func (m *NodeStatusManager) Get(nodeName string) LocalNodeStatus {
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.statuses[nodeName]
}

func (m *NodeStatusManager) UpdateStatus(newStatus LocalNodeStatus, op string) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	switch op {
	case "add":
		m.statuses[newStatus.Name] = newStatus
	case "update":
		m.statuses[newStatus.Name] = newStatus
	case "delete":
		delete(m.statuses, newStatus.Name)
	default:
		return fmt.Errorf("not support type %s", op)
	}
	return nil
}

func (m *NodeStatusManager) AddPodResources(nodeName string, cpu, mem int64) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	if status, exists := m.statuses[nodeName]; exists {
		status.TotalCPURequest += cpu
		status.TotalMemRequest += mem
		m.statuses[nodeName] = status
		return nil
	}
	return fmt.Errorf("node %s not found", nodeName)
}

func (m *NodeStatusManager) RemovePodResources(nodeName string, cpu, mem int64) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	if status, exists := m.statuses[nodeName]; exists {
		status.TotalCPURequest -= cpu
		status.TotalMemRequest -= mem
		m.statuses[nodeName] = status
		return nil
	}
	return fmt.Errorf("node %s not found", nodeName)
}
