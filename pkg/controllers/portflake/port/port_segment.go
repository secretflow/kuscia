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

type Segment struct {
	min, max int // port range: [min, max)
}

func NewSegment(min, max int) Segment {
	return Segment{min: min, max: max}
}

func (s Segment) numberPorts() int {
	return s.max - s.min
}

type SegmentList struct {
	segments []Segment
	count    int
}

func NewSegmentList(segments []Segment) *SegmentList {
	sl := &SegmentList{
		segments: segments,
	}

	for _, s := range segments {
		sl.count += s.max - s.min
	}

	return sl
}

func (sl *SegmentList) Count() int {
	return sl.count
}
