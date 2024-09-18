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

package utils

import (
	"testing"

	"github.com/secretflow/kuscia/pkg/diagnose/common"
)

func TestReporter(t *testing.T) {
	reporter := NewReporter("")
	defer reporter.Render()
	table := reporter.NewTableWriter()
	table.SetTitle("title")
	table.AddHeader([]string{"Name", "Value", "Result"})
	table.AddRow([]string{"Alice", "1000", common.Pass})
	table.AddRow([]string{"Bob", "1000", common.Warning})
	table.AddRow([]string{"Carol", "1000", common.Fail})
}
