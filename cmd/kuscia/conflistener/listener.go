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

package conflistener

import (
	"context"
	"os"
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"

	"github.com/secretflow/kuscia/cmd/kuscia/confloader"
	"github.com/secretflow/kuscia/cmd/kuscia/modules"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

var (
	updateMutex sync.Mutex
)

func LogLevelHotReloadListener(ctx context.Context, cf string) {

	nlog.Infof("Kuscia config file watcher start, config file: %s.", cf)
	configFileCheck(ctx, cf)
}

func configFileCheck(ctx context.Context, cf string) {

	initWG := sync.WaitGroup{}
	initWG.Add(1)

	go func() {
		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			nlog.Fatalf("Kuscia config file watcher init failed: %v", err)
		}
		defer func(watcher *fsnotify.Watcher) {
			err = watcher.Close()
			if err != nil {
				nlog.Fatalf("Kuscia config file watcher close failed: %v", err)
			}
		}(watcher)

		// filename & file dir
		configFile := filepath.Clean(cf)
		configFileDir, _ := filepath.Split(configFile)
		realConfigFile, err := filepath.EvalSymlinks(cf)
		nlog.Debugf("Kuscia config file watcher real config file dir: %v, configFile: %v, realConfigFile: %v",
			configFileDir, configFile, realConfigFile)

		eventWatcher(ctx, watcher, &initWG, realConfigFile, cf, configFileDir)

	}()

	initWG.Wait()
}

func eventWatcher(ctx context.Context, watcher *fsnotify.Watcher, initWG *sync.WaitGroup, realCf, cf, cfd string) {

	eventsWG := sync.WaitGroup{}
	eventsWG.Add(1)

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:

				if !ok {
					eventsWG.Done()
					return
				}
				currentConfigFile, _ := filepath.EvalSymlinks(cf)

				if (filepath.Clean(event.Name) == cf &&
					(event.Has(fsnotify.Write) ||
						event.Has(fsnotify.Create))) ||
					(currentConfigFile != "" && currentConfigFile != realCf) {
					realCf = currentConfigFile
					if checkFile(realCf) {
						updateLogLevel(ctx, realCf)
					} else {
						nlog.Error("Kuscia config file watcher update failed. ignore...")
					}
				}
			case err, ok := <-watcher.Errors:

				if ok {
					nlog.Errorf("Kuscia config file watcher error: %v", err.Error())
				}

				eventsWG.Done()
				return
			}
		}
	}()

	err := watcher.Add(cfd)
	if err != nil {
		initWG.Done()
		return
	}
	initWG.Done()
	eventsWG.Wait()
}

// checkFile check config file is empty or not
func checkFile(cf string) bool {

	fileContent, err := os.ReadFile(cf)
	if err != nil {
		nlog.Errorf("Read config file %s failed: %v", cf, err)
		return false
	}
	if len(fileContent) > 0 {
		return true
	}

	return false
}

func updateLogLevel(ctx context.Context, cf string) {

	updateMutex.Lock()
	defer updateMutex.Unlock()

	// Check if the file exists and is readable
	if _, err := os.Stat(cf); os.IsNotExist(err) {
		nlog.Infof("Config file %s does not exist, skip update", cf)
	}

	nlog.Debugf("Kuscia config file watcher path: %s", cf)
	config, err := confloader.ReadConfig(cf)
	if err != nil {
		nlog.Errorf("Kuscia config file watcher read config failed: %v", err)
		return
	}

	modules.UpdateCommonConfigs(ctx, config)
}
