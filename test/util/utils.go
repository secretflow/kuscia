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

package util

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/pkg/scheduler/framework"
	st "k8s.io/kubernetes/pkg/scheduler/testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
)

func MakeTaskResource(namespace, name string, min int, creationTime *time.Time) *kusciaapisv1alpha1.TaskResource {
	ti := 10
	tr := &kusciaapisv1alpha1.TaskResource{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec:       kusciaapisv1alpha1.TaskResourceSpec{MinReservedPods: min, ResourceReservedSeconds: ti},
	}
	if creationTime != nil {
		tr.CreationTimestamp = metav1.Time{Time: *creationTime}
	}

	return tr
}

func MakeNodesAndPods(labels, annotations map[string]string, existingPodsNum, allNodesNum int) (existingPods []*corev1.Pod, allNodes []*corev1.Node) {
	type keyVal struct {
		k string
		v string
	}
	var labelPairs []keyVal
	for k, v := range labels {
		labelPairs = append(labelPairs, keyVal{k: k, v: v})
	}

	var annotationPairs []keyVal
	for k, v := range annotations {
		annotationPairs = append(annotationPairs, keyVal{k: k, v: v})
	}

	// build nodes
	for i := 0; i < allNodesNum; i++ {
		res := map[corev1.ResourceName]string{
			corev1.ResourceCPU:  "1",
			corev1.ResourcePods: "20",
		}
		node := st.MakeNode().Name(fmt.Sprintf("node%d", i)).Capacity(res)
		allNodes = append(allNodes, &node.Node)
	}
	// build pods
	for i := 0; i < existingPodsNum; i++ {
		podWrapper := st.MakePod().Name(fmt.Sprintf("pod%d", i)).Node(fmt.Sprintf("node%d", i%allNodesNum))
		// apply labels[0], labels[0,1], ..., labels[all] to each pod in turn
		if len(labelPairs) > 0 {
			for _, p := range labelPairs[:i%len(labelPairs)+1] {
				podWrapper = podWrapper.Label(p.k, p.v)
			}
		}

		if len(annotationPairs) > 0 {
			for _, p := range annotationPairs[:i%len(annotationPairs)+1] {
				podWrapper = podWrapper.Annotation(p.k, p.v)
			}
		}
		existingPods = append(existingPods, podWrapper.Obj())
	}
	return
}

var _ framework.SharedLister = &FakeSharedLister{}

type FakeSharedLister struct {
	nodeInfos                                    []*framework.NodeInfo
	nodeInfoMap                                  map[string]*framework.NodeInfo
	havePodsWithAffinityNodeInfoList             []*framework.NodeInfo
	havePodsWithRequiredAntiAffinityNodeInfoList []*framework.NodeInfo
}

func (f *FakeSharedLister) NodeInfos() framework.NodeInfoLister {
	return f
}

func (f *FakeSharedLister) List() ([]*framework.NodeInfo, error) {
	return f.nodeInfos, nil
}

func (f *FakeSharedLister) HavePodsWithAffinityList() ([]*framework.NodeInfo, error) {
	return f.havePodsWithAffinityNodeInfoList, nil
}

func (f *FakeSharedLister) HavePodsWithRequiredAntiAffinityList() ([]*framework.NodeInfo, error) {
	return f.havePodsWithRequiredAntiAffinityNodeInfoList, nil
}

func (f *FakeSharedLister) Get(nodeName string) (*framework.NodeInfo, error) {
	return f.nodeInfoMap[nodeName], nil
}

func (f *FakeSharedLister) StorageInfos() framework.StorageInfoLister {
	return f
}

func (f *FakeSharedLister) IsPVCUsedByPods(key string) bool {
	return true
}

func CreateNodeInfoMap(pods []*corev1.Pod, nodes []*corev1.Node) map[string]*framework.NodeInfo {
	nodeNameToInfo := make(map[string]*framework.NodeInfo)
	for _, pod := range pods {
		nodeName := pod.Spec.NodeName
		if _, ok := nodeNameToInfo[nodeName]; !ok {
			nodeNameToInfo[nodeName] = framework.NewNodeInfo()
		}
		nodeNameToInfo[nodeName].AddPod(pod)
	}

	for _, node := range nodes {
		if _, ok := nodeNameToInfo[node.Name]; !ok {
			nodeNameToInfo[node.Name] = framework.NewNodeInfo()
		}
		nodeInfo := nodeNameToInfo[node.Name]
		nodeInfo.SetNode(node)
	}
	return nodeNameToInfo
}

func NewFakeSharedLister(pods []*corev1.Pod, nodes []*corev1.Node) framework.SharedLister {
	nodeInfoMap := CreateNodeInfoMap(pods, nodes)
	nodeInfos := make([]*framework.NodeInfo, 0, len(nodeInfoMap))
	havePodsWithAffinityNodeInfoList := make([]*framework.NodeInfo, 0, len(nodeInfoMap))
	havePodsWithRequiredAntiAffinityNodeInfoList := make([]*framework.NodeInfo, 0, len(nodeInfoMap))
	for _, v := range nodeInfoMap {
		nodeInfos = append(nodeInfos, v)
		if len(v.PodsWithAffinity) > 0 {
			havePodsWithAffinityNodeInfoList = append(havePodsWithAffinityNodeInfoList, v)
		}
		if len(v.PodsWithRequiredAntiAffinity) > 0 {
			havePodsWithRequiredAntiAffinityNodeInfoList = append(havePodsWithRequiredAntiAffinityNodeInfoList, v)
		}
	}
	return &FakeSharedLister{
		nodeInfos:                        nodeInfos,
		nodeInfoMap:                      nodeInfoMap,
		havePodsWithAffinityNodeInfoList: havePodsWithAffinityNodeInfoList,
		havePodsWithRequiredAntiAffinityNodeInfoList: havePodsWithRequiredAntiAffinityNodeInfoList,
	}
}

func MakeBase64EncodeCert(t *testing.T) string {
	return base64.StdEncoding.EncodeToString(makeCertificate(t))
}

func MakeCertString(t *testing.T) string {
	return strings.TrimSuffix(string(makeCertificate(t)), "\n")
}

func makeCertificate(t *testing.T) []byte {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)

	certTemplate := x509.Certificate{
		SerialNumber:          big.NewInt(int64(uuid.New().ID())),
		Subject:               pkix.Name{CommonName: "Test"},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		SubjectKeyId:          []byte{1, 2, 3, 4, 6},
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &certTemplate, &certTemplate, &privateKey.PublicKey, privateKey)
	assert.NoError(t, err)
	certBuf := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	return certBuf
}
