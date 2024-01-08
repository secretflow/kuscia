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

package port

import (
	"math/rand"
	"time"
)

type FuncCheckPortValid func(port int) bool

func ChoosePortByRandom(segmentList *SegmentList, checkPortValid FuncCheckPortValid, retryCount int) (int, int, bool) {
	retPort := 0
	retPortIdx := 0

	for i := 0; i < retryCount; i++ {
		randPort := rand.Intn(segmentList.Count())

		for idx, segment := range segmentList.segments {
			if randPort < segment.numberPorts() {
				retPort = segment.min + randPort
				retPortIdx = idx
				break
			}
			randPort -= segment.numberPorts()
		}

		if checkPortValid(retPort) {
			return retPort, retPortIdx, true
		}
	}

	return retPort, retPortIdx, false
}

func ChoosePortByIteration(segmentList *SegmentList, checkPortValid FuncCheckPortValid, basePort int, basePortIdx int) (int, int, bool) {
	retPort := basePort
	retPortIdx := basePortIdx

	for i := 0; i < segmentList.Count(); i++ {
		retPort++

		if retPort >= segmentList.segments[retPortIdx].max {
			retPortIdx++
			if retPortIdx >= len(segmentList.segments) {
				retPortIdx = 0
			}

			retPort = segmentList.segments[retPortIdx].min
		}

		if retPort == basePort {
			return retPort, retPortIdx, false
		}

		if checkPortValid(retPort) {
			return retPort, retPortIdx, true
		}
	}

	return retPort, retPortIdx, false
}

func init() {
	rand.Seed(time.Now().UnixNano())
}
