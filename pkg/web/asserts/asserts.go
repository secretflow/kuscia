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

package asserts

import (
	"fmt"
	"reflect"
	"strings"
)

func IsNil(obj interface{}, errorMsg string) (err error) {
	defer func() {
		if e := recover(); e != nil {
			err = fmt.Errorf("panic, err: %v", e)
		}
	}()
	val := reflect.ValueOf(obj)
	if !val.IsNil() {
		return fmt.Errorf(errorMsg)
	}
	return nil
}

func NotNil(obj interface{}, errorMsg string) (err error) {
	defer func() {
		if e := recover(); e != nil {
			err = fmt.Errorf("panic, err: %v", e)
		}
	}()
	val := reflect.ValueOf(obj)
	if val.IsNil() {
		return fmt.Errorf(errorMsg)
	}
	return nil
}

func IsTrue(v bool, errorMsg string) error {
	if !v {
		return fmt.Errorf(errorMsg)
	}
	return nil
}

func True(v bool, errorMsg string) error {
	if !v {
		return fmt.Errorf(errorMsg)
	}
	return nil // v is true  -> ok
}

func False(v bool, errorMsg string) error {
	if v { // v is true ->  return error
		return fmt.Errorf(errorMsg)
	}
	return nil // v is false  -> ok
}

func IsEmpty(inputStr, errorMsg string) error {
	if len(inputStr) != 0 {
		return fmt.Errorf(errorMsg)
	}
	return nil
}

func NotEmpty(inputStr string, errorMsg string) error {
	if len(inputStr) == 0 {
		return fmt.Errorf(errorMsg)
	}
	return nil
}

func Equals(leftStr, rightStr, errorMsg string) error {
	if strings.Compare(leftStr, rightStr) != 0 {
		return fmt.Errorf(errorMsg)
	}
	return nil
}

func NotEquals(leftStr, rightStr, errorMsg string) error {
	if strings.Compare(leftStr, rightStr) == 0 {
		return fmt.Errorf(errorMsg)
	}
	return nil
}
