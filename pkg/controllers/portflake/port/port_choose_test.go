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
	"testing"

	"gotest.tools/v3/assert"
)

func TestChoosePortByRandom(t *testing.T) {
	tests := []struct {
		segments           *SegmentList
		funcCheckPortValid FuncCheckPortValid
		retryCount         int
		ok                 bool
	}{
		{
			segments: NewSegmentList([]Segment{
				NewSegment(11000, 30000),
				NewSegment(32768, 40000),
			}),
			funcCheckPortValid: func(port int) bool {
				return true
			},
			retryCount: 10,
			ok:         true,
		},
		{
			segments: NewSegmentList([]Segment{
				NewSegment(11000, 30000),
				NewSegment(32768, 40000),
			}),
			funcCheckPortValid: func(port int) bool {
				return false
			},
			retryCount: 10,
			ok:         false,
		},
	}

	for i, tt := range tests {
		_, _, ok := ChoosePortByRandom(tt.segments, tt.funcCheckPortValid, tt.retryCount)
		assert.Equal(t, tt.ok, ok, "ChoosePortByRandom No.%d", i)
	}

}

func TestChoosePortByIteration(t *testing.T) {
	tests := []struct {
		segments           *SegmentList
		funcCheckPortValid FuncCheckPortValid
		basePort           int
		basePortIdx        int
		retPort            int
		retPortIdx         int
		ok                 bool
	}{
		{
			segments: NewSegmentList([]Segment{
				NewSegment(11000, 30000),
				NewSegment(32768, 40000),
			}),
			funcCheckPortValid: func(port int) bool {
				return true
			},
			basePort:    11000,
			basePortIdx: 0,
			retPort:     11001,
			retPortIdx:  0,
			ok:          true,
		},
		{
			segments: NewSegmentList([]Segment{
				NewSegment(11000, 30000),
				NewSegment(32768, 40000),
			}),
			funcCheckPortValid: func(port int) bool {
				if port >= 35000 {
					return true
				} else {
					return false
				}
			},
			basePort:    11000,
			basePortIdx: 0,
			retPort:     35000,
			retPortIdx:  1,
			ok:          true,
		},
		{
			segments: NewSegmentList([]Segment{
				NewSegment(11000, 30000),
				NewSegment(32768, 40000),
			}),
			funcCheckPortValid: func(port int) bool {
				if port == 15000 {
					return true
				} else {
					return false
				}
			},
			basePort:    32768,
			basePortIdx: 1,
			retPort:     15000,
			retPortIdx:  0,
			ok:          true,
		},
		{
			segments: NewSegmentList([]Segment{
				NewSegment(11000, 30000),
				NewSegment(32768, 40000),
			}),
			funcCheckPortValid: func(port int) bool {
				return false
			},
			basePort:    11000,
			basePortIdx: 0,
			retPort:     11000,
			retPortIdx:  0,
			ok:          false,
		},
	}

	for i, tt := range tests {
		retPort, retPortIdx, ok := ChoosePortByIteration(tt.segments, tt.funcCheckPortValid, tt.basePort, tt.basePortIdx)
		assert.Equal(t, tt.ok, ok, "ChoosePortByIteration No.%d", i)
		assert.Equal(t, tt.retPort, retPort, "ChoosePortByIteration No.%d", i)
		assert.Equal(t, tt.retPortIdx, retPortIdx, "ChoosePortByIteration No.%d", i)
	}

}
