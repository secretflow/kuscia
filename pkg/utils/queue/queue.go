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

package queue

import (
	"context"
	"time"

	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

type queueHandler func(ctx context.Context, key string) error

// EnqueueObjectWithKey is used to enqueue object key.
func EnqueueObjectWithKey(obj interface{}, queue workqueue.Interface) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		nlog.Errorf("Enqueue obj %v is invalid: %v", obj, err.Error())
		return
	}

	queue.Add(key)
	nlog.Debugf("Enqueue key: %q", key)
}

// EnqueueObjectWithKeyName is used to enqueue object key name.
func EnqueueObjectWithKeyName(obj interface{}, queue workqueue.Interface) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		nlog.Errorf("Enqueue obj %q is invalid: %v", obj, err.Error())
		return
	}

	_, keyName, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		nlog.Errorf("Get key %q name failed: %v", key, err.Error())
		return
	}

	queue.Add(keyName)
	nlog.Debugf("Enqueue key %q name: %q", key, keyName)
}

// EnqueueObjectWithKeyNamespace is used to enqueue object key namespace.
func EnqueueObjectWithKeyNamespace(obj interface{}, queue workqueue.Interface) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		nlog.Errorf("Enqueue obj %v is invalid: %v", obj, err.Error())
		return
	}

	keyNamespace, _, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		nlog.Errorf("Get key %q namespace failed: %v", key, err.Error())
		return
	}

	queue.Add(keyNamespace)
	nlog.Debugf("Enqueue key %q namespace: %q", key, keyNamespace)
}

// HandleQueueItem is used to handle queue item with retrying when error happened.
func HandleQueueItem(ctx context.Context, queueID string, q workqueue.RateLimitingInterface, handler queueHandler, maxRetries int) bool {
	obj, shutdown := q.Get()
	if shutdown {
		return false
	}
	run := func(obj interface{}) {
		startTime := time.Now()
		// We call Done here so the work queue knows we have finished processing this item.
		// We also must remember to call Forget if we do not want this work item being re-queued.
		// For example, we do not call Forget if a transient error occurs.
		// Instead, the item is put back on the work queue and attempted again after a back-off period.
		defer q.Done(obj)
		var key string
		var ok bool
		// We expect strings to come off the work queue.
		// These are of the form namespace/name.
		// We do this as the delayed nature of the work queue means the items in the informer cache
		// may actually be more up to date that when the item was initially put onto the workqueue.
		if key, ok = obj.(string); !ok {
			// As the item in the work queue is actually invalid, we call Forget here else we'd go
			// into a loop of attempting to process a work item that is invalid.
			q.Forget(obj)
			nlog.Warnf("Get obj failed: expected string item in work queue id[%v] but got %#v", queueID, obj)
			return
		}

		nlog.Debugf("Start processing item: queue id[%v], key[%v]", queueID, key)
		// Run the handler, passing it the namespace/name string of the Pod resource to be synced.
		if err := handler(ctx, key); err != nil {
			if q.NumRequeues(key) < maxRetries {
				// Put the item back on the work queue to handle any transient errors.
				nlog.Infof("Re-syncing: queue id[%v], retry:[%d] key[%v]: %q, re-queuing (%v)", queueID, q.NumRequeues(key), key, err.Error(), time.Since(startTime))
				q.AddRateLimited(key)
				return
			}
			// We've exceeded the maximum retries, so we must forget the key.
			q.Forget(key)
			nlog.Errorf("Forgetting: queue id[%v], key[%v] (%v), due to maximum retries[%v] reached, last error: %q",
				queueID, key, time.Since(startTime), maxRetries, err.Error())
			return
		}

		// Finally, if no error occurs we Forget this item so it does not get queued again until
		// another change happens.
		q.Forget(obj)

		nlog.Infof("Finish processing item: queue id[%v], key[%v] (%v)", queueID, key, time.Since(startTime))
	}
	select {
	case <-ctx.Done():
		nlog.Warnf("HandleQueueItem quit, because %s", ctx.Err().Error())
		return false
	default:
		run(obj)
	}

	return true
}

// HandleQueueItemWithAlwaysRetry is used to handle queue item with retrying when error happened.
func HandleQueueItemWithAlwaysRetry(ctx context.Context, queueID string, q workqueue.RateLimitingInterface, handler queueHandler) bool {
	obj, shutdown := q.Get()
	if shutdown {
		return false
	}
	run := func(obj interface{}) {
		startTime := time.Now()
		// We call Done here so the work queue knows we have finished processing this item.
		// We also must remember to call Forget if we do not want this work item being re-queued.
		// For example, we do not call Forget if a transient error occurs.
		// Instead, the item is put back on the work queue and attempted again after a back-off period.
		defer q.Done(obj)
		var key string
		var ok bool
		// We expect strings to come off the work queue.
		// These are of the form namespace/name.
		// We do this as the delayed nature of the work queue means the items in the informer cache
		// may actually be more up to date that when the item was initially put onto the workqueue.
		if key, ok = obj.(string); !ok {
			// As the item in the work queue is actually invalid, we call Forget here else we'd go
			// into a loop of attempting to process a work item that is invalid.
			q.Forget(obj)
			nlog.Warnf("Get obj failed: expected string item in work queue id[%v] but got %#v", queueID, obj)
			return
		}

		nlog.Debugf("Start processing item: queue id[%v], key[%v]", queueID, key)
		// Run the handler, passing it the namespace/name string of the Pod resource to be synced.
		if err := handler(ctx, key); err != nil {
			// Put the item back on the work queue to handle any transient errors.
			nlog.Infof("Re-syncing: queue id[%v], key[%v]: %q, re-queuing (%v)", queueID, key, err.Error(), time.Since(startTime))
			q.AddRateLimited(key)
			return
		}

		// Finally, if no error occurs we Forget this item so it does not get queued again until
		// another change happens.
		q.Forget(obj)
		nlog.Infof("Finish processing item: queue id[%v], key[%v] (%v)", queueID, key, time.Since(startTime))
	}
	select {
	case <-ctx.Done():
		nlog.Warnf("HandleQueueItem quit, because %s", ctx.Err().Error())
		return false
	default:
		run(obj)
	}

	return true
}

// HandleQueueItemWithoutRetry is used to handle queue item without retrying.
func HandleQueueItemWithoutRetry(ctx context.Context, queueID string, q workqueue.Interface, handler queueHandler) bool {
	obj, shutdown := q.Get()
	if shutdown {
		return false
	}

	run := func(obj interface{}) {
		startTime := time.Now()
		// We call Done here so the work queue knows we have finished processing this item.
		defer q.Done(obj)

		// We expect strings to come off the work queue.
		// These are of the form namespace/name.
		// We do this as the delayed nature of the work queue means the items in the informer cache
		// may actually be more up to date that when the item was initially put onto the workqueue.
		key, ok := obj.(string)
		if !ok {
			nlog.Warnf("Get obj failed: expected string item in work queue id[%v] but got %#v", queueID, obj)
			return
		}

		nlog.Debugf("Start processing item: queue id[%v], key[%v]", queueID, key)
		// Run the handler, passing it the namespace/name string of the Pod resource to be synced.
		if err := handler(ctx, key); err != nil {
			nlog.Errorf("Handle queue id[%v] key[%v] (%v) failed: %v", queueID, key, time.Since(startTime), err.Error())
		} else {
			nlog.Infof("Finish processing item: queue id[%v], key[%v] (%v)", queueID, key, time.Since(startTime))
		}
	}
	select {
	case <-ctx.Done():
		nlog.Warnf("HandleQueueItem quit, because %s", ctx.Err().Error())
		return false
	default:
		run(obj)
	}

	return true
}
