/*
Copyright 2017 The Kubernetes Authors.

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

package signals

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

var onlyOneSignalHandler = make(chan struct{})

// SetupSignalHandler registered for SIGTERM and SIGINT. A stop channel is returned
// which is closed on one of these signals. If a second signal is caught, the program
// is terminated with exit code 1.
func SetupSignalHandler() (stopCh <-chan struct{}) {
	close(onlyOneSignalHandler) // panics when called twice

	stop := make(chan struct{})
	c := make(chan os.Signal, 2)
	signal.Notify(c, shutdownSignals...)
	go func() {
		sig := <-c
		nlog.Infof("Caught signal %v. Shutting down...", sig.String())
		close(stop)
		<-c
		os.Exit(1) // second signal. Exit directly.
	}()

	return stop
}

type kusciaContext struct {
	stopCh <-chan struct{}
}

func (*kusciaContext) Deadline() (deadline time.Time, ok bool) {
	return
}

func (k *kusciaContext) Done() <-chan struct{} {
	return k.stopCh
}

func (k *kusciaContext) Err() error {
	select {
	case <-k.stopCh:
		return fmt.Errorf("%s", "receive shutdown signals")
	default:
		return nil
	}
}

func (*kusciaContext) Value(key any) any {
	return nil
}

func (*kusciaContext) String() string {
	return "Kuscia Context"
}

func NewKusciaContextWithStopCh(stopCh <-chan struct{}) context.Context {
	return &kusciaContext{stopCh: stopCh}
}
