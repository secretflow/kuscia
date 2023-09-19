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

package errorcode

type Errs []error

func NoError(errs *Errs) bool {
	if errs == nil || len(*errs) == 0 {
		return true
	}
	return false
}

func (errs *Errs) AppendErr(err error) {
	if err != nil {
		newErrs := append(*errs, err)
		*errs = newErrs
	}
}

func (errs *Errs) Assert(err error) {
	if err != nil {
		newErrs := append(*errs, err)
		*errs = newErrs
		panic(errs)
	}
}

func (errs *Errs) String() string {
	if errs == nil || len(*errs) == 0 {
		return ""
	}

	if len(*errs) == 1 {
		return (*errs)[0].Error()
	}

	ret := ""
	for i, err := range *errs {
		ret += err.Error()
		if i != len(*errs)-1 {
			ret += ";"
		}
	}
	return ret
}
