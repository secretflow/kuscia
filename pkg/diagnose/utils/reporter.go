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
	"fmt"
	"os"

	"github.com/olekukonko/tablewriter"

	"github.com/secretflow/kuscia/pkg/diagnose/common"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

type Reporter struct {
	fileName string
	file     *os.File
	tables   []*Table
}

type Table struct {
	title    string
	fileName string
	file     *os.File
	table    *tablewriter.Table
}

func NewReporter(file string) *Reporter {
	reporter := &Reporter{
		fileName: file,
	}
	if err := reporter.Open(); err != nil {
		reporter.fileName = ""
	}
	return reporter
}

func (r *Reporter) Open() error {
	if r.fileName != "" {
		file, err := os.Create(r.fileName)
		if err != nil {
			nlog.Warnf("Open file failed, use stdout to record, err: %v", err)
			return err
		}
		r.file = file
	}
	return nil
}

func (r *Reporter) NewTableWriter() *Table {
	table := new(Table)
	table.file = r.file
	table.fileName = r.fileName
	if r.fileName != "" {
		if r.file == nil {
			table.table = tablewriter.NewWriter(os.Stdout)
		}
		table.table = tablewriter.NewWriter(r.file)
	} else {
		table.table = tablewriter.NewWriter(os.Stdout)
	}
	r.tables = append(r.tables, table)
	return table
}

func (r *Reporter) WriteLine(name string) error {
	if r.fileName != "" {
		fmt.Fprintln(r.file, name)
	} else {
		fmt.Println(name)
	}
	return nil
}

func (r *Table) WriteLine(name string) error {
	if r.fileName != "" {
		fmt.Fprintln(r.file, name)
	} else {
		fmt.Println(name)
	}
	return nil
}

func (r *Table) SetTitle(title string) {
	r.title = title
}

func (r *Table) AddHeader(element []string) {
	r.table.SetHeader(element)
}

func (r *Table) AddRow(elements []string) {
	if r.fileName != "" {
		r.table.Append(elements)
	} else {
		colors := make([]tablewriter.Colors, 0)
		for _, element := range elements {
			switch element {
			case common.Fail:
				colors = append(colors, tablewriter.Colors{tablewriter.Normal, tablewriter.FgRedColor})
			case common.Pass:
				colors = append(colors, tablewriter.Colors{tablewriter.Normal, tablewriter.FgGreenColor})
			case common.Warning:
				colors = append(colors, tablewriter.Colors{tablewriter.Normal, tablewriter.FgYellowColor})
			default:
				colors = append(colors, tablewriter.Colors{tablewriter.Normal, tablewriter.Normal})
			}
		}
		r.table.Rich(elements, colors)
	}
}

func (r *Table) RenderTable() {
	_ = r.WriteLine(r.title)
	if r.table != nil {
		r.table.Render()
	}
	_ = r.WriteLine("")
}

func (r *Reporter) Close() error {
	defer r.file.Close()
	return nil
}

func (r *Reporter) Render() {
	_ = r.WriteLine("REPORT:")
	for _, t := range r.tables {
		t.RenderTable()
	}
}
