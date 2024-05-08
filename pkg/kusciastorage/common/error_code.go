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

package common

const (
	ErrorCodeForSuccess int32 = 200
)

const (
	ErrorCodeForServerInternalErr   int32 = 71001001 + iota //71001001
	ErrorCodeForRequestIsInvalid                            //71001002
	ErrorCodeForRefRequestCreate                            //71001004
	ErrorCodeForNormalRequestCreate                         //71001005
	ErrorCodeForDeleteResource                              //71001006
	ErrorCodeForQueryResource                               //71001007
)

var errorCodeToMsg = map[int32]string{
	ErrorCodeForSuccess:             "success",
	ErrorCodeForServerInternalErr:   "server internal error",
	ErrorCodeForRequestIsInvalid:    "request is invalid",
	ErrorCodeForRefRequestCreate:    "failed to create resource for reference request",
	ErrorCodeForNormalRequestCreate: "failed to create resource for normal request",
	ErrorCodeForDeleteResource:      "failed to delete resource",
	ErrorCodeForQueryResource:       "failed to query resource",
}

// GetMsg is used to get error message.
func GetMsg(code int32) string {
	return errorCodeToMsg[code]
}
