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

package rfile

import (
	"fmt"

	"github.com/mitchellh/mapstructure"
	"github.com/rosedblabs/rosedb/v2"

	"github.com/secretflow/kuscia/pkg/secretbackend"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

// RFile uses Rosedb to store confidential information, and you can configure Cipher to encrypt it.
type RFile struct {
	conf   Config
	db     *rosedb.DB
	cipher Cipher
}

type Config struct {
	Path   string       `yaml:"path"`
	Cipher CipherConfig `yaml:"cipher"`
}

type CipherConfig struct {
	Type string          `yaml:"type"`
	AES  AESCipherConfig `yaml:"aes"`
}

func NewRFile(configMap map[string]any) (secretbackend.SecretDriver, error) {
	config := Config{}
	if err := mapstructure.Decode(configMap, &config); err != nil {
		return nil, err
	}
	if config.Path == "" {
		return nil, fmt.Errorf("path should not be empty")
	}

	// open a database
	nlog.Infof("Start to open secret backend with config: %+v", config)
	options := rosedb.DefaultOptions
	options.DirPath = config.Path
	db, err := rosedb.Open(options)
	if err != nil {
		return nil, fmt.Errorf("init rosedb failed: %s", err)
	}

	return &RFile{conf: config, db: db}, nil
}

func (r *RFile) Set(confID string, value string) error {
	valueBytes := []byte(value)
	if r.cipher != nil {
		var err error
		valueBytes, err = r.cipher.Decrypt([]byte(value))
		if err != nil {
			return err
		}
	}

	err := r.db.Put([]byte(confID), valueBytes)
	if err != nil {
		return err
	}
	return nil
}

func (r *RFile) Get(confID string) (string, error) {
	return r.GetByParams(confID, nil)
}

func (r *RFile) GetByParams(confID string, params map[string]any) (string, error) {
	// not support params, ignore
	value, err := r.db.Get([]byte(confID))
	if err != nil {
		return "", err
	}

	if r.cipher != nil {
		value, err = r.cipher.Decrypt(value)
		if err != nil {
			return "", err
		}
	}

	return string(value), nil
}

func (r *RFile) Close() error {
	if err := r.db.Close(); err != nil {
		return err
	}
	r.db = nil
	return nil
}

func init() {
	secretbackend.Register("rfile", NewRFile)
}
