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
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAESCipher_EncryptAndDecrypt(t *testing.T) {
	type args struct {
		value []byte
	}
	tests := []struct {
		name    string
		key     string
		args    args
		wantErr bool
	}{
		{
			name: "AESCipher encrypt and decrypt should success",
			key:  "hello",
			args: args{
				value: []byte("helloworld"),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := NewAESCipher(tt.key)
			assert.Equal(t, err, nil, "new aes cipher failed")
			ciphertext, err := c.Encrypt(tt.args.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("Encrypt() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			plaintext, err := c.Decrypt(ciphertext)
			if (err != nil) != tt.wantErr {
				t.Errorf("Decrypt() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(plaintext, tt.args.value) {
				t.Errorf("Decrypt() got = %v, want %v", plaintext, tt.args.value)
			}
		})
	}
}
