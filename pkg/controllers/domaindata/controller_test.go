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

package domaindata

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"

	"github.com/secretflow/kuscia/pkg/controllers"
	"github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	kusciafake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/nlog/zlogwriter"
	"github.com/secretflow/kuscia/pkg/utils/signals"
	"github.com/secretflow/kuscia/pkg/utils/tls"
)

func createCrtString(t *testing.T) (*rsa.PrivateKey, string) {
	rootDir := t.TempDir()
	caCertFile := filepath.Join(rootDir, "ca.crt")
	caKeyFile := filepath.Join(rootDir, "ca.key")
	assert.NoError(t, tls.CreateCAFile("testca", caCertFile, caKeyFile))
	f, err := os.Open(caCertFile)
	assert.NoError(t, err)
	testCrt, err := io.ReadAll(f)
	assert.NoError(t, err)
	priKey, err := tls.ParsePKCS1PrivateKey(caKeyFile)
	assert.NoError(t, err)
	return priKey, base64.StdEncoding.EncodeToString(testCrt)
}

func Test_doValidate(t *testing.T) {
	logger, _ := zlogwriter.New(nil)
	nlog.Setup(nlog.SetWriter(logger))
	kubeClient := kubefake.NewSimpleClientset()
	kusciaClient := kusciafake.NewSimpleClientset()
	ch := make(chan struct{}, 1)
	ctx := signals.NewKusciaContextWithStopCh(ch)
	c := NewController(ctx, controllers.ControllerConfig{
		KubeClient:   kubeClient,
		KusciaClient: kusciaClient,
	})

	go func() {
		prikey, certstr := createCrtString(t)
		kusciaClient.KusciaV1alpha1().Domains().Create(ctx, &v1alpha1.Domain{
			ObjectMeta: v1.ObjectMeta{
				Name: "alice",
			},
			Spec: v1alpha1.DomainSpec{
				Cert: certstr,
			},
		}, v1.CreateOptions{})
		kusciaClient.KusciaV1alpha1().Domains().Create(ctx, &v1alpha1.Domain{
			ObjectMeta: v1.ObjectMeta{
				Name: "bob",
			},
			Spec: v1alpha1.DomainSpec{
				Cert: certstr,
			},
		}, v1.CreateOptions{})
		kusciaClient.KusciaV1alpha1().DomainDatas("alice").Create(ctx, &v1alpha1.DomainData{
			ObjectMeta: v1.ObjectMeta{
				Name:      "dataid",
				Namespace: "alice",
			},
		}, v1.CreateOptions{})
		time.Sleep(100 * time.Millisecond)
		dg := &v1alpha1.DomainDataGrant{
			ObjectMeta: v1.ObjectMeta{
				Namespace: "alice",
				Name:      "testgrant",
				Annotations: map[string]string{
					"test": "alice",
				},
			},
			Spec: v1alpha1.DomainDataGrantSpec{
				Author:       "alice",
				DomainDataID: "dataid",
				GrantDomain:  "bob",
				Limit: &v1alpha1.GrantLimit{
					ExpirationTime: &v1.Time{
						Time: time.Now().Add(150 * time.Second),
					},
				},
			},
		}
		signDomainDataGrant(prikey, &dg.Spec)
		_, err := kusciaClient.KusciaV1alpha1().DomainDataGrants(dg.Namespace).Create(ctx, dg, v1.CreateOptions{})
		assert.NoError(t, err)
		time.Sleep(100 * time.Millisecond)
		pass := false
		for i := 0; i < 10; i++ {
			dg, err = kusciaClient.KusciaV1alpha1().DomainDataGrants(dg.Namespace).Get(ctx, dg.Name, v1.GetOptions{})
			assert.NoError(t, err)
			if dg.Status.Phase == v1alpha1.GrantReady {
				pass = true
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		assert.Equal(t, true, pass)
		// set expire
		dg.Spec.Limit.ExpirationTime.Time = time.Now()
		kusciaClient.KusciaV1alpha1().DomainDataGrants(dg.Namespace).Update(ctx, dg, v1.UpdateOptions{})
		time.Sleep(100 * time.Millisecond)
		dg.Annotations["test"] = "bob"
		kusciaClient.KusciaV1alpha1().DomainDataGrants(dg.Namespace).Update(ctx, dg, v1.UpdateOptions{})
		pass = false
		for i := 0; i < 10; i++ {
			dg, err = kusciaClient.KusciaV1alpha1().DomainDataGrants(dg.Namespace).Get(ctx, dg.Name, v1.GetOptions{})
			assert.NoError(t, err)
			if dg.Status.Phase == v1alpha1.GrantUnavailable {
				pass = true
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		assert.Equal(t, true, pass)
		close(ch)
	}()
	c.Run(4)
}

func signDomainDataGrant(priKey *rsa.PrivateKey, dg *v1alpha1.DomainDataGrantSpec) error {
	dg.Signature = ""
	bs, err := json.Marshal(dg)
	if err != nil {
		return err
	}
	h := sha256.New()
	h.Write(bs)
	digest := h.Sum(nil)
	sign, err := rsa.SignPKCS1v15(rand.Reader, priKey, crypto.SHA256, digest)
	if err != nil {
		return err
	}
	dg.Signature = base64.StdEncoding.EncodeToString(sign)
	return nil
}
