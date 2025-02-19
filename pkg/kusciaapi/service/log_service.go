// Copyright 2024 Ant Group Co., Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package service

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/nxadm/tail"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciaclientset "github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	"github.com/secretflow/kuscia/pkg/kusciaapi/config"
	"github.com/secretflow/kuscia/pkg/kusciaapi/proxy"
	apiutils "github.com/secretflow/kuscia/pkg/kusciaapi/utils"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	tlsutils "github.com/secretflow/kuscia/pkg/utils/tls"
	"github.com/secretflow/kuscia/pkg/web/constants"
	"github.com/secretflow/kuscia/pkg/web/utils"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/errorcode"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/kusciaapi"
)

type ILogService interface {
	QueryTaskLog(ctx context.Context, request *kusciaapi.QueryLogRequest, eventCh chan<- *kusciaapi.QueryLogResponse)
	QueryPodNode(ctx context.Context, request *kusciaapi.QueryPodNodeRequest) *kusciaapi.QueryPodNodeResponse
}

const (
	QueryLogPath  = "/api/v1/log/task/query"
	OutputLineNum = 100
	OutputPeriod  = 5 * time.Second
)

type logService struct {
	kusciaClient kusciaclientset.Interface
	kubeClient   kubernetes.Interface
	conf         *config.KusciaAPIConfig
}

func NewLogService(config *config.KusciaAPIConfig) ILogService {
	switch config.RunMode {
	case common.RunModeLite:
		return &logServiceLite{
			kusciaAPIClient: proxy.NewKusciaAPIClient(""),
			conf:            config,
		}
	default:
		return &logService{
			kusciaClient: config.KusciaClient,
			kubeClient:   config.KubeClient,
			conf:         config,
		}
	}
}

func (s logService) QueryPodNode(ctx context.Context, request *kusciaapi.QueryPodNodeRequest) *kusciaapi.QueryPodNodeResponse {
	task, err := s.kusciaClient.KusciaV1alpha1().KusciaTasks(common.KusciaCrossDomain).Get(ctx, request.TaskId, metav1.GetOptions{})
	if err != nil {
		return &kusciaapi.QueryPodNodeResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrorCode_KusciaAPIErrQueryPodNode, err.Error()),
		}
	}

	podName := buildPodName(request.Domain, request.TaskId, request.ReplicaIdx)
	nodeName, err := getPodNode(task, podName)
	if err != nil {
		return &kusciaapi.QueryPodNodeResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrorCode_KusciaAPIErrQueryPodNode, err.Error()),
		}
	}

	nodeIP, err := s.getNodeIP(ctx, nodeName)
	if err != nil {
		return &kusciaapi.QueryPodNodeResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.ErrorCode_KusciaAPIErrQueryPodNode, fmt.Sprintf("failed to get node ip for node %s, err: %v", nodeName, err)),
		}
	}

	return &kusciaapi.QueryPodNodeResponse{
		Status:   utils.BuildSuccessResponseStatus(),
		NodeName: nodeName,
		NodeIp:   nodeIP,
	}
}

func (s logService) getNodeIP(ctx context.Context, nodeName string) (string, error) {
	node, err := s.kubeClient.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get k8s node, %v", err)
	}
	for _, address := range node.Status.Addresses {
		if address.Type == v1.NodeInternalIP {
			return address.Address, nil
		}
	}
	return "", fmt.Errorf("internal ip not exists in node %s", nodeName)
}

func getPodNode(task *v1alpha1.KusciaTask, podName string) (string, error) {
	if task.Status.Phase == v1alpha1.TaskPending {
		return "", fmt.Errorf("task %s status pending, please try later", task.Name)

	}
	podStatus, ok := task.Status.PodStatuses[podName]
	if !ok {
		return "", fmt.Errorf("task %s hasn't created pod %s yet, please try later", task.Name, podName)
	}
	if podStatus.PodPhase == v1.PodPending {
		return "", fmt.Errorf("task pod %s status pending, please try later", podName)
	}
	return podStatus.NodeName, nil
}

func (s logService) QueryTaskLog(ctx context.Context, request *kusciaapi.QueryLogRequest, eventCh chan<- *kusciaapi.QueryLogResponse) {
	if s.conf.RunMode == common.RunModeMaster {
		eventCh <- &kusciaapi.QueryLogResponse{Status: utils.BuildErrorResponseStatus(errorcode.ErrorCode_KusciaAPIErrMasterAPINotSupport, "kuscia master api not support this interface now")}
		return
	}
	// overrideRequestDomain(s.conf.RunMode, s.conf.DomainID, request)
	domain := s.conf.DomainID
	podName := buildPodName(domain, request.TaskId, request.ReplicaIdx)
	if request.Local {
		nlog.Infof("Perfrom local log query for pod %s", podName)
		if err := localQueryLog(request, domain, s.conf.StdoutPath, eventCh); err != nil {
			eventCh <- &kusciaapi.QueryLogResponse{Status: utils.BuildErrorResponseStatus(errorcode.ErrorCode_KusciaAPIErrQueryLog, fmt.Sprintf("failed to local query, err: %v", err))}
		}
		return
	}
	// get task by id
	nlog.Infof("Get kuscia task by task id")
	task, err := s.kusciaClient.KusciaV1alpha1().KusciaTasks(common.KusciaCrossDomain).Get(ctx, request.TaskId, metav1.GetOptions{})
	if err != nil {
		eventCh <- &kusciaapi.QueryLogResponse{Status: utils.BuildErrorResponseStatus(errorcode.ErrorCode_KusciaAPIErrQueryLog, err.Error())}
		return
	}

	// validate request param
	nlog.Infof("Validate query log request param")
	if err = s.validateQueryRequest(ctx, request, task); err != nil {
		eventCh <- &kusciaapi.QueryLogResponse{Status: utils.BuildErrorResponseStatus(errorcode.ErrorCode_KusciaAPIErrRequestValidate, err.Error())}
		return
	}

	// check task and pod status
	nodeName, err := getPodNode(task, podName)
	if err != nil {
		eventCh <- &kusciaapi.QueryLogResponse{Status: utils.BuildErrorResponseStatus(errorcode.ErrorCode_KusciaAPIErrQueryLog, err.Error())}
		return
	}
	nlog.Infof("Query log get pod node %s, self node name: %s", nodeName, s.conf.NodeName)
	if nodeName == s.conf.NodeName {
		// local query
		nlog.Infof("Perfrom local log query for pod %s", podName)
		if err := localQueryLog(request, domain, s.conf.StdoutPath, eventCh); err != nil {
			eventCh <- &kusciaapi.QueryLogResponse{Status: utils.BuildErrorResponseStatus(errorcode.ErrorCode_KusciaAPIErrQueryLog, fmt.Sprintf("failed to local query, err: %v", err))}
		}
	} else {
		// proxy to another node which owns the pod
		nodeIP, err := s.getNodeIP(ctx, nodeName)
		if err != nil {
			eventCh <- &kusciaapi.QueryLogResponse{Status: utils.BuildErrorResponseStatus(errorcode.ErrorCode_KusciaAPIErrQueryLog, fmt.Sprintf("failed to get node ip for node %s, err: %v", nodeName, err))}
			return
		}
		nlog.Infof("Perfrom proxy log query for pod %s, ip: %s", podName, nodeIP)
		if err := proxyQueryLog(ctx, nodeIP, s.conf, request, eventCh); err != nil {
			eventCh <- &kusciaapi.QueryLogResponse{Status: utils.BuildErrorResponseStatus(errorcode.ErrorCode_KusciaAPIErrQueryLog, fmt.Sprintf("failed to proxy query, err: %v", err))}
		}
	}
}

func (s logService) validateQueryRequest(ctx context.Context, request *kusciaapi.QueryLogRequest, task *v1alpha1.KusciaTask) error {
	// find appimage
	var appImageName string
	var role string
	for _, party := range task.Spec.Parties {
		if party.DomainID == s.conf.DomainID {
			appImageName = party.AppImageRef
			role = party.Role
			break
		}
	}
	if appImageName == "" {
		return fmt.Errorf("can't find the party in this task")
	}

	appImage, err := s.kusciaClient.KusciaV1alpha1().AppImages().Get(ctx, appImageName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("can't find appimage %s for task id %s, err: %v", appImageName, request.TaskId, err)
	}

	// find deploytemplate
	var deployTemplate v1alpha1.DeployTemplate
	if role == "" && len(appImage.Spec.DeployTemplates) > 0 {
		deployTemplate = appImage.Spec.DeployTemplates[0]
	} else {
		for _, dt := range appImage.Spec.DeployTemplates {
			if dt.Role == role {
				deployTemplate = dt
				break
			}
		}
	}

	// check replicate index param
	if request.ReplicaIdx < 0 || request.ReplicaIdx >= *deployTemplate.Replicas {
		return fmt.Errorf("replicate index invalid")
	}

	// check container param
	if request.Container == "" {
		if len(deployTemplate.Spec.Containers) == 1 {
			request.Container = deployTemplate.Spec.Containers[0].Name
		} else {
			return fmt.Errorf("task's deploy container larger than 1, please specify the container name")
		}
	} else {
		found := false
		for _, container := range deployTemplate.Spec.Containers {
			if request.Container == container.Name {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("container name %s invalid", request.Container)
		}
	}
	return nil
}

func localQueryLog(request *kusciaapi.QueryLogRequest, domain string, stdoutPath string, eventCh chan<- *kusciaapi.QueryLogResponse) error {
	podPrefix := fmt.Sprintf("%s_%s-%d", domain, request.TaskId, request.ReplicaIdx)
	podLogDir, err := findNewestDirWithPrefix(stdoutPath, podPrefix)
	if err != nil || podLogDir == "" {
		return fmt.Errorf("can't find pod log directory for %s, err: %v", podPrefix, err)
	}
	nlog.Infof("Newest pod log directory for %s is %s", podPrefix, podLogDir)
	podLogDir = filepath.Join(podLogDir, request.Container)
	logPath, err := findLargestRestartLogPath(podLogDir)
	if err != nil || logPath == "" {
		return fmt.Errorf("can't find pod log path for %s, err: %v", podLogDir, err)
	}
	nlog.Infof("Largest pod log file in %s is %s", podLogDir, logPath)

	return tailFile(logPath, request.Follow, eventCh)
}

func proxyQueryLog(ctx context.Context, nodeIP string, kusciaAPIConfig *config.KusciaAPIConfig, request *kusciaapi.QueryLogRequest, eventCh chan<- *kusciaapi.QueryLogResponse) error {
	tlsConfig := kusciaAPIConfig.TLS
	protocol := kusciaAPIConfig.Protocol
	httpPort := kusciaAPIConfig.HTTPPort
	tokenConfig := kusciaAPIConfig.Token
	var clientTLSConfig *tls.Config
	var err error
	schema := constants.SchemaHTTP
	// init client tls config
	if tlsConfig != nil {
		if protocol == common.TLS {
			clientTLSConfig, err = tlsutils.BuildClientSimpleTLSConfig(tlsConfig.ServerCert)
		} else {
			clientTLSConfig, err = tlsutils.BuildClientTLSConfig(tlsConfig.RootCA, tlsConfig.ServerCert, tlsConfig.ServerKey)
		}
		if err != nil {
			nlog.Errorf("local tls config error: %v", err)
			return err
		}
		schema = constants.SchemaHTTPS
	}

	// token auth
	var token string
	var tokenAuth bool
	if tokenConfig != nil {
		token, err = apiutils.ReadToken(*tokenConfig)
		if err != nil {
			nlog.Error(err.Error())
			return err
		}
		tokenAuth = true
	}

	httpClient := utils.BuildHTTPClient(clientTLSConfig)
	httpURL := fmt.Sprintf("%s://%s:%d%s", schema, nodeIP, httpPort, QueryLogPath)
	byteReq, err := json.Marshal(request)
	if err != nil {
		nlog.Errorf("Send request %+v ,marshal request failed: %s", request.String(), err.Error())
		return err
	}
	req, err := http.NewRequest(http.MethodPost, httpURL, bytes.NewReader(byteReq))
	if err != nil {
		nlog.Errorf("invalid request error: %v", err)
		return err
	}
	req.Header.Set(constants.ContentTypeHeader, constants.HTTPDefaultContentType)
	if tokenAuth {
		req.Header.Set(constants.TokenHeader, token)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		nlog.Errorf("send health request error: %v", err)
		return err
	}
	defer resp.Body.Close()
	// check http status code
	if resp.StatusCode != http.StatusOK {
		nlog.Errorf("Send watch request %+v failed, http code: %d", request.String(), resp.StatusCode)
		return fmt.Errorf("unexpected error, status_code: '%d'", resp.StatusCode)
	}
	// decode response
	decoder := json.NewDecoder(resp.Body)
	// Loop: parse the stream response
	for {
		select {
		case <-ctx.Done():
			nlog.Warnf("The query context has canceled")
			return nil
		default:
			resp := &kusciaapi.QueryLogResponse{}
			err := decoder.Decode(&resp)
			if err != nil {
				if err.Error() == "EOF" {
					nlog.Warnf("The query stream has finished")
					return nil
				}
				nlog.Errorf("Decoding response to JSON failed, error : %s.", err.Error())
				return err
			}
			nlog.Debugf("Query log: %+v", resp)
			// send a response to channel
			eventCh <- resp
		}
	}
}

func findNewestDirWithPrefix(rootDir string, prefix string) (string, error) {
	var newestFolder string
	var newestTime time.Time

	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() && strings.HasPrefix(info.Name(), prefix) {
			if newestFolder == "" || info.ModTime().After(newestTime) {
				newestFolder = path
				newestTime = info.ModTime()
			}
		}
		return nil
	})

	return newestFolder, err
}

func findLargestRestartLogPath(root string) (string, error) {
	var largestFile string
	var largestNumber int
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && strings.HasSuffix(info.Name(), ".log") {
			baseName := strings.TrimSuffix(info.Name(), ".log")
			number, err := strconv.Atoi(baseName)
			if err == nil {
				if largestFile == "" || number > largestNumber {
					largestFile = path
					largestNumber = number
				}
			}
		}
		return nil
	})
	return largestFile, err
}

func tailFile(fileName string, follow bool, eventCh chan<- *kusciaapi.QueryLogResponse) error {
	// read file using tail
	config := tail.Config{
		Follow: follow,
		ReOpen: follow,
		Poll:   true,
	}
	t, err := tail.TailFile(fileName, config)
	if err != nil {
		return err
	}
	defer t.Cleanup()
	var buffer []string
	ticker := time.NewTicker(OutputPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if len(buffer) != 0 {
				eventCh <- &kusciaapi.QueryLogResponse{Status: utils.BuildSuccessResponseStatus(), Log: strings.Join(buffer, "\n")}
				buffer = buffer[:0]
			}
		case line, ok := <-t.Lines:
			if !ok {
				if len(buffer) != 0 {
					eventCh <- &kusciaapi.QueryLogResponse{Status: utils.BuildSuccessResponseStatus(), Log: strings.Join(buffer, "\n")}
				}
				nlog.Info("Tail channel closed, return")
				return nil
			}
			if line.Err != nil {
				nlog.Errorf("Tail line error: %v", line.Err)
				continue
			}
			nlog.Debugf("Tail log %s", line.Text)
			buffer = append(buffer, line.Text)
			if len(buffer) >= OutputLineNum {
				eventCh <- &kusciaapi.QueryLogResponse{Status: utils.BuildSuccessResponseStatus(), Log: strings.Join(buffer, "\n")}
				buffer = buffer[:0]
				ticker.Reset(OutputPeriod)
			}
		}
	}

}

func buildPodName(domain, taskId string, replicaIdx int32) string {
	return fmt.Sprintf("%s/%s-%d", domain, taskId, replicaIdx)
}

// func overrideRequestDomain(runMode string, domainID string, request *kusciaapi.QueryLogRequest) {
// 	if runMode != common.RunModeMaster {
// 		request.Domain = domainID
// 	}
// }
