// Copyright 2024 Ant Group Co., Ltd.
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

package netstat

import (
	"context"
	"fmt"
	"strconv"
	"sync"
)

type IsMatch func(context.Context, int) error

func ParallelSearchArray(ctx context.Context, testcases []int, matcher IsMatch) (int, error) {
	var results sync.Map
	var wg sync.WaitGroup
	wg.Add(len(testcases))
	for _, testcase := range testcases {
		go func(param int) {
			defer wg.Done()
			err := matcher(ctx, param)
			var msg string
			if err != nil {
				msg = err.Error()
			}
			results.Store(param, msg)
		}(testcase)
	}
	wg.Wait()
	result := 0
	var errMsg string
	for _, testcase := range testcases {
		msg, ok := results.Load(testcase)
		errMsg = msg.(string)
		if msg == "" && ok {
			result = testcase
		}
	}
	return result, fmt.Errorf("%s", errMsg)
}

func Decimal(value float64) float64 {
	value, _ = strconv.ParseFloat(fmt.Sprintf("%.5f", value), 64)
	return value
}

func MBToByte(value float64) int {
	mul := 1 << 20
	return int(value * float64(mul))
}

func ToSecond(value int) float64 {
	return Decimal(float64(value) / 1000)
}

func ToMbps(value int) float64 {
	div := 1 << 20
	return Decimal(float64(value<<3) / float64(div))
}

func ToMillisecond(value float64) int {
	return int(value * 1000)
}
