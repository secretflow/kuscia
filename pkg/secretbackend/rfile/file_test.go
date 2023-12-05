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
	"os"
	"testing"

	"gotest.tools/v3/assert"
)

func tempDirRFileParams() map[string]any {
	path, _ := os.MkdirTemp("", "")
	return map[string]any{
		"path": path,
	}
}

func TestNewRFile(t *testing.T) {
	type args struct {
		configMap map[string]any
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "new rfile driver should success",
			args: args{
				configMap: tempDirRFileParams(),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewRFile(tt.args.configMap)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewRFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestRFile_SetAndGet(t *testing.T) {
	type setArgs struct {
		confID string
		value  string
	}
	type getArgs struct {
		confID string
	}
	tests := []struct {
		name       string
		setArgs    setArgs
		getArgs    getArgs
		wantSetErr bool
		wantGetErr bool
		wantGot    string
	}{
		{
			name: "set and get with exist confID should return success",
			setArgs: setArgs{
				confID: "hello",
				value:  "world",
			},
			getArgs: getArgs{
				"hello",
			},
			wantSetErr: false,
			wantGetErr: false,
			wantGot:    "world",
		},
		{
			name: "set and get with no-exist confID should return success",
			setArgs: setArgs{
				confID: "hello2",
				value:  "world2",
			},
			getArgs: getArgs{
				"hello3",
			},
			wantSetErr: false,
			wantGetErr: true,
			wantGot:    "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			driver, err := NewRFile(tempDirRFileParams())
			assert.Equal(t, err, nil, "NewRFile should success")
			if err := driver.Set(tt.setArgs.confID, tt.setArgs.value); (err != nil) != tt.wantSetErr {
				t.Errorf("Set() error = %v, wantErr %v", err, tt.wantGetErr)
			}
			got, err := driver.Get(tt.getArgs.confID)
			if (err != nil) != tt.wantGetErr {
				t.Errorf("Get() error = %v, wantErr %v", err, tt.wantGetErr)
				return
			}
			if got != tt.wantGot {
				t.Errorf("Get() got = %v, want %v", got, tt.wantGot)
			}
		})
	}
}
