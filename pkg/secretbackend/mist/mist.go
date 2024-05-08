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

package mist

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/secretflow/kuscia/pkg/secretbackend"
	"github.com/secretflow/kuscia/pkg/utils/nlog"

	"github.com/mitchellh/mapstructure"
	"gitlab.alipay-inc.com/mist-sdk/mist_sdk_go/mist"
)

const (
	// SecretWrapperContentTypeSecret for short content, value will be string
	SecretWrapperContentTypeSecret = "secret"
	// SecretWrapperContentTypeCompose for long content, value will be mistMultiValue
	SecretWrapperContentTypeCompose = "compose"

	mistAntVipURLForStable = "antvip-pool.stable.alipay.net"
	mistAntVipURLForTest   = "antvip-pool.test.alipay.net"
	mistAntVipURLForPre    = "antvip-pool.cz99s.alipay.com"
	mistAntVipURLForGray   = "antvip-pool.cz87w.alipay.com"
	mistAntVipURLForProd   = "antvip-pool.global.alipay.com"

	mistBkmiURLForTest = "bkmi-read-pool.stable.global.alipay.com"
	mistBkmiURLForProd = "bkmi-read-pool.global.alipay.com"
)

var mistEndpoints = map[string]mistEndpoint{
	"stable": {
		antVipURL: mistAntVipURLForStable,
		BkmiURL:   mistBkmiURLForTest,
	},
	"test": {
		antVipURL: mistAntVipURLForTest,
		BkmiURL:   mistBkmiURLForTest,
	},
	"pre": {
		antVipURL: mistAntVipURLForPre,
		BkmiURL:   mistBkmiURLForProd,
	},
	"gray": {
		antVipURL: mistAntVipURLForGray,
		BkmiURL:   mistBkmiURLForProd,
	},
	"prod": {
		antVipURL: mistAntVipURLForProd,
		BkmiURL:   mistBkmiURLForProd,
	},
}

type Mist struct {
	config Config
	client IMistClient
}

type Config struct {
	AppName string `mapstructure:"appName"`
	Tenant  string `mapstructure:"tenant"`
	Env     string `mapstructure:"env"`
}

type wrapperValue struct {
	Type    string
	Content string // for SecretWrapperContentTypeSecret
	Part    int    // for SecretWrapperContentTypeCompose
}

type mistMultiValue struct {
	MultiPart *int `json:"_kuscia_multipart_,omitempty"`
}

type IMistClient interface {
	GetSecretInfo(secretName string) (string, string, string, string, error)
	GetFactorSecretInfo(secretName string, factor string) (string, string, string, string, error)
}

func NewMistSecretDriver(configMap map[string]any) (secretbackend.SecretDriver, error) {
	config := Config{}
	if err := mapstructure.Decode(configMap, &config); err != nil {
		return nil, err
	}
	mistConfig := mist.NewMistConfig()
	mistConfig.SetAppName(config.AppName)
	mistConfig.SetTenant(config.Tenant)
	mistConfig.SetMode(config.Env)
	endpoint, exist := mistEndpoints[config.Env]
	if !exist {
		return nil, fmt.Errorf("can not find mist endpoint for Env=%s", config.Env)
	}
	mistConfig.SetAntVipUrl(endpoint.antVipURL)
	mistConfig.SetBkmiUrl(endpoint.BkmiURL)

	nlog.Infof("NewMistSecretDriver config=%+v, endpoint=%+v", config, endpoint)

	return &Mist{
		config: config,
		client: mist.NewMistClient(mistConfig),
	}, nil
}

// setMistClient only for package debug
func (m *Mist) setMistClient(c IMistClient) {
	m.client = c
}

func (m *Mist) Set(confID string, value string) error {
	return fmt.Errorf("unsupported for mist set")
}

func (m *Mist) Get(confID string) (string, error) {
	return m.GetByParams(confID, nil)
}

type getByParams struct {
	Factor      string   `mapstructure:"factor"`
	PartFactors []string `mapstructure:"part_factors"`
}

func (m *Mist) GetByParams(confID string, params map[string]any) (string, error) {
	var mistGetParams *getByParams
	if params != nil {
		mistGetParams = &getByParams{}
		if err := mapstructure.Decode(params, mistGetParams); err != nil {
			return "", err
		}
	}
	mistKey := m.idToMistKey(confID)
	var factor = ""
	if mistGetParams != nil {
		factor = mistGetParams.Factor
	}
	secretValue, err := m.getMistValue(mistKey, factor)
	if err != nil {
		return "", err
	}

	switch secretValue.Type {
	case SecretWrapperContentTypeSecret:
		return secretValue.Content, nil
	case SecretWrapperContentTypeCompose:
		if mistGetParams != nil {
			if secretValue.Part != len(mistGetParams.PartFactors) {
				return "", fmt.Errorf("find multipart secret but partCount not equals part factors num: %d != %d",
					secretValue.Part, len(mistGetParams.PartFactors))
			}
		}
		partChannels := make([]<-chan getMistKeyAsyncResult, 0)
		for i := 0; i < secretValue.Part; i++ {
			partFactor := ""
			if mistGetParams != nil {
				partFactor = mistGetParams.PartFactors[i]
			}
			partChannels = append(partChannels, m.getMistKeyAsync(m.idToMistPartKey(confID, i), partFactor))
		}
		var value = strings.Builder{}
		for _, channel := range partChannels {
			partResult := <-channel
			if partResult.err != nil {
				return "", err
			}
			if partResult.value.Type != SecretWrapperContentTypeSecret {
				return "", fmt.Errorf("unexpected value type for mist multipart key: %s", partResult.value.Type)
			}
			value.WriteString(partResult.value.Content)
		}
		return value.String(), nil
	default:
		return "", fmt.Errorf("unexpected value type for mist key")
	}
}

func (m *Mist) Close() error {
	m.client = nil
	return nil
}

func (m *Mist) getMistValue(mistKey string, factor string) (*wrapperValue, error) {
	var secretInfo string
	var err error
	if factor != "" {
		_, secretInfo, _, _, err = m.client.GetFactorSecretInfo(mistKey, factor)
	} else {
		_, secretInfo, _, _, err = m.client.GetSecretInfo(mistKey)
	}
	if err != nil {
		return nil, err
	}

	// try to detect mist multipart count
	if part := m.detectMistMultiPart(secretInfo); part != nil {
		return &wrapperValue{
			Type: SecretWrapperContentTypeCompose,
			Part: *part,
		}, nil
	}

	return &wrapperValue{
		Type:    SecretWrapperContentTypeSecret,
		Content: secretInfo,
	}, nil
}

func (m *Mist) detectMistMultiPart(secretInfo string) *int {
	// try to Unmarshal to mistMultiValue
	var secretValue mistMultiValue
	var err = json.Unmarshal([]byte(secretInfo), &secretValue)
	if err != nil {
		return nil
	}
	if secretValue.MultiPart == nil || *secretValue.MultiPart <= 0 {
		return nil
	}
	return secretValue.MultiPart
}

type getMistKeyAsyncResult struct {
	value *wrapperValue
	err   error
}

func (m *Mist) getMistKeyAsync(mistKey string, factor string) <-chan getMistKeyAsyncResult {
	resultChan := make(chan getMistKeyAsyncResult)
	go func() {
		value, err := m.getMistValue(mistKey, factor)
		resultChan <- getMistKeyAsyncResult{
			value: value,
			err:   err,
		}
	}()

	return resultChan
}

func (m *Mist) idToMistKey(confID string) string {
	return fmt.Sprintf("other_manual_%s_%s", m.config.AppName, confID)
}

func (m *Mist) idToMistPartKey(confID string, part int) string {
	return fmt.Sprintf("other_manual_%s_%s_part_%d", m.config.AppName, confID, part)
}

type mistEndpoint struct {
	antVipURL string
	BkmiURL   string
}

func init() {
	secretbackend.Register("mist", NewMistSecretDriver)
}
