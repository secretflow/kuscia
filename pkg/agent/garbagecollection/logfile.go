// Copyright 2024 Ant Group Co., Ltd.
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

package garbagecollection

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"time"

	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"

	"github.com/secretflow/kuscia/pkg/agent/utils/fileutils"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

type LogFileGCService struct {
	LogFileGCConfig
	ctx       context.Context
	cancelCtx context.Context
	cancelFn  context.CancelFunc
}

func (fgc *LogFileGCService) Start() {
	kubeInformerFactory := kubeinformers.NewSharedInformerFactoryWithOptions(
		fgc.KubeClient, 0, kubeinformers.WithNamespace(fgc.Namespace))
	kubeInformerFactory.Start(fgc.ctx.Done())
	kubeInformerFactory.WaitForCacheSync(fgc.ctx.Done())
	nlog.Infof("Start LogFileGCService.")
	go func() {
		// use config.GCInterval as ticker interval
		ticker := time.NewTicker(fgc.GCInterval)
		defer ticker.Stop()
		for {
			select {
			case <-fgc.cancelCtx.Done():
				return
			case <-ticker.C:
				if err := fgc.Once(); err != nil {
					nlog.Warnf("LogFileGCService gc once with error, the last err is:%v", err)
					continue
				}
			}
		}
	}()
}

func (fgc *LogFileGCService) Stop() {
	nlog.Infof("Stop LogFileGCService.")
	fgc.cancelFn()
}

func (fgc *LogFileGCService) Once() (err error) {
	nlog.Infof("gc once start")
	// check all the file/directory under pathList, and delete which is older than gcDuration
	deleteCount := 0
	defer func(num *int) {
		nlog.Infof("gc once end, delete count:%d", *num)
	}(&deleteCount)
	cfg := fgc.LogFileGCConfig
	gcTimePoint := time.Now().Add(-cfg.GCDuration)
	// Step1: list all the sub directories (pod log directory)
	paths, listErr := fileutils.ListDir(fgc.LogFilePath)
	if listErr != nil {
		err = fmt.Errorf("list files failed, err:%v", listErr)
		return
	}
	// Step2: check if directory name match podNamePattern
	podRegex, regErr := regexp.Compile(cfg.PodNamePattern)
	if regErr != nil || podRegex == nil {
		err = fmt.Errorf("pod regex compile failed, err:%v", regErr)
		return
	}
	paths = lo.Filter(paths, func(path string, _ int) bool {
		// use regex pattern PodNamePattern to extract pod id
		podName := podRegex.FindStringSubmatch(path)
		return len(podName) == 2
	})
	// Step3: check path duration
	if cfg.IsDurationCheck {
		paths = lo.Filter(paths, func(path string, _ int) bool {
			return fileutils.IsModifyBefore(path, gcTimePoint)
		})
	}
	// Step4: recursively check sub directories and files duration
	if cfg.IsRecursiveDurationCheck {
		paths = lo.Filter(paths, func(path string, _ int) bool {
			// if path is directory, recursive check
			treePaths, listErr := fileutils.ListTree(path)
			if listErr != nil {
				return false
			}
			for _, treePath := range treePaths {
				if !fileutils.IsModifyBefore(treePath, gcTimePoint) {
					return false
				}
			}
			return true
		})
	}
	// Step5: check if pod is terminal or not exist
	paths = lo.Filter(paths, func(path string, _ int) bool {
		podName := podRegex.FindStringSubmatch(path)[1]
		pod, getErr := fgc.GCConfig.KubeClient.CoreV1().Pods(fgc.GCConfig.Namespace).Get(fgc.ctx, podName, metav1.GetOptions{})
		if k8serrors.IsNotFound(getErr) || (getErr == nil && (pod.Status.Phase == v1.PodFailed || pod.Status.Phase == v1.PodSucceeded)) {
			return true
		} else {
			return false
		}
	})
	// Step6: now we can delete the paths
	for _, path := range paths {
		deleteCount++
		if deleteCount > fgc.GCConfig.MaxDeleteNum {
			break
		}
		nlog.Infof("gc delete path:%s", path)
		rmErr := os.RemoveAll(path)
		if rmErr != nil {
			err = fmt.Errorf("delete path:%s failed, err:%v", path, rmErr)
		}
	}
	return
}

func NewLogFileGCService(ctx context.Context, config LogFileGCConfig) *LogFileGCService {
	serviceCtx, cancelFn := context.WithCancel(ctx)
	return &LogFileGCService{
		LogFileGCConfig: config,
		ctx:             ctx,
		cancelCtx:       serviceCtx,
		cancelFn:        cancelFn,
	}
}
