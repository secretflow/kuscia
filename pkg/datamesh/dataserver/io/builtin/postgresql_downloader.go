// Copyright 2025 Ant Group Co., Ltd.
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

package builtin

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/apache/arrow/go/v13/arrow"
	"github.com/apache/arrow/go/v13/arrow/array"
	"github.com/apache/arrow/go/v13/arrow/memory"
	"github.com/huandu/go-sqlbuilder"
	"github.com/pkg/errors"
	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/datamesh/dataserver/utils"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/proto/api/v1alpha1"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/datamesh"
)

type PostgresqlDownloader struct {
	ctx            context.Context
	db             *sql.DB
	data           *datamesh.DomainData
	query          *datamesh.CommandDomainDataQuery
	closeNormal    bool
	totalCount     uint
	builder        *array.RecordBuilder
	fieldConverter []func(string)
	err            error
	writeBatchSize int
}

func NewPostgresqlDownloader(ctx context.Context, db *sql.DB, data *datamesh.DomainData, query *datamesh.CommandDomainDataQuery) *PostgresqlDownloader {
	return &PostgresqlDownloader{
		ctx:            ctx,
		db:             db,
		data:           data,
		query:          query,
		closeNormal:    false,
		totalCount:     0,
		builder:        nil,
		fieldConverter: nil,
		writeBatchSize: 1024,
	}
}

// check if d.query fit the domaindata, and return the domaindata column map, and the real query orderedNames
func (d *PostgresqlDownloader) generateQueryColumns() (map[string]*v1alpha1.DataColumn, []string, error) {
	// create domaindata and query column map[columnName->columnType]
	if len(d.data.Columns) == 0 {
		return nil, nil, errors.Errorf("no data column available, terminate reading")
	}
	var domaindataColumnMap map[string]*v1alpha1.DataColumn
	var orderedNames []string

	domaindataColumnMap = make(map[string]*v1alpha1.DataColumn)
	orderedNames = make([]string, 0, len(d.data.Columns))
	for _, column := range d.data.Columns {
		domaindataColumnMap[column.Name] = column
		orderedNames = append(orderedNames, column.Name)
	}

	if len(d.query.Columns) == 0 {
		return domaindataColumnMap, orderedNames, nil
	}

	// use query column priority
	orderedNames = orderedNames[:0]
	for _, column := range d.query.Columns {
		v, ok := domaindataColumnMap[column]
		if !ok {
			return nil, nil, errors.Errorf("query column(%s) is not defined in domaindata(%s)", column, d.data.DomaindataId)
		}
		orderedNames = append(orderedNames, v.Name)
	}

	return domaindataColumnMap, orderedNames, nil
}

func (d *PostgresqlDownloader) checkSQLSupport(columnMap map[string]*v1alpha1.DataColumn, orderedNames []string) error {
	// check backtick, check type (only key in orderedNames)
	for _, val := range orderedNames {
		k, v := val, columnMap[val]
		if strings.IndexByte(k, '"') != -1 {
			return errors.Errorf("invalid column name(%s). For safety reason, backtick is not allowed", k)
		}
		if strings.Contains(v.Type, "date") || strings.Contains(v.Type, "binary") {
			return errors.Errorf("type(%s) is not supported now, please consider change another type", v.Type)
		}
	}
	return nil
}

func (d *PostgresqlDownloader) initFieldConverter(bldr array.Builder) func(string) {
	switch bldr.Type().(type) {
	case *arrow.BooleanType:
		return func(str string) {
			if err := ParseBool(bldr, str); err != nil && d.err == nil {
				d.err = err
			}
		}
	case *arrow.Int8Type:
		return func(str string) {
			if err := ParseInt8(bldr, str); err != nil && d.err == nil {
				d.err = err
			}
		}
	case *arrow.Int16Type:
		return func(str string) {
			if err := ParseInt16(bldr, str); err != nil && d.err == nil {
				d.err = err
			}
		}
	case *arrow.Int32Type:
		return func(str string) {
			if err := ParseInt32(bldr, str); err != nil && d.err == nil {
				d.err = err
			}
		}
	case *arrow.Int64Type:
		return func(str string) {
			if err := ParseInt64(bldr, str); err != nil && d.err == nil {
				d.err = err
			}
		}
	case *arrow.Uint8Type:
		return func(str string) {
			if err := ParseUint8(bldr, str); err != nil && d.err == nil {
				d.err = err
			}
		}
	case *arrow.Uint16Type:
		return func(str string) {
			if err := ParseUint16(bldr, str); err != nil && d.err == nil {
				d.err = err
			}
		}
	case *arrow.Uint32Type:
		return func(str string) {
			if err := ParseUint32(bldr, str); err != nil && d.err == nil {
				d.err = err
			}
		}
	case *arrow.Uint64Type:
		return func(str string) {
			if err := ParseUint64(bldr, str); err != nil && d.err == nil {
				d.err = err
			}
		}
	case *arrow.Float32Type:
		return func(str string) {
			if err := ParseFloat32(bldr, str); err != nil && d.err == nil {
				d.err = err
			}
		}
	case *arrow.Float64Type:
		return func(str string) {
			if err := ParseFloat64(bldr, str); err != nil && d.err == nil {
				d.err = err
			}
		}
	case *arrow.StringType:
		return func(str string) {
			bldr.(*array.StringBuilder).Append(str)
		}

	default:
		panic(fmt.Errorf("postgresql to arrow conversion: unhandled field type %T", bldr.Type()))
	}
}

func (d *PostgresqlDownloader) DataProxyContentToFlightStreamSQL(w utils.RecordWriter) (err error) {
	// transfer postgresql rows to arrow record
	columnMap, orderedNames, err := d.generateQueryColumns()

	if err != nil {
		nlog.Errorf("Generate query columns failed(%s)", err)
		return err
	}
	err = d.checkSQLSupport(columnMap, orderedNames)
	if err != nil {
		nlog.Error(err)
		return err
	}
	if strings.IndexByte(d.data.RelativeUri, '"') != -1 {
		err = errors.Errorf("invalid table name(%s). For safety reason, backtick is not allowed", d.data.RelativeUri)
		nlog.Error(err)
		return err
	}

	fieldNums := len(orderedNames)

	// generate column names with backtick
	backTickColumns := make([]string, 0, fieldNums)

	for _, name := range orderedNames {
		backTickColumns = append(backTickColumns, "\""+name+"\"")
	}
	tableName := "\"" + d.data.RelativeUri + "\""

	// query
	sql := sqlbuilder.Select(backTickColumns...).From(tableName).String()
	rows, err := d.db.Query(sql)
	if err != nil {
		nlog.Errorf("Postgresql query failed(%s)", err)
		return err
	}

	// avoid panic
	defer func() {
		if r := recover(); r != nil {
			nlog.Errorf("Read domaindata(%s) panic %+v", d.data.DomaindataId, r)
			err = fmt.Errorf("Read domaindata(%s) panic: %+v", d.data.DomaindataId, r)
		}
	}()

	rowValues := make([][]byte, fieldNums)
	scanArgs := make([]interface{}, fieldNums)
	for i := range rowValues {
		scanArgs[i] = &rowValues[i]
	}

	var iCount int64
	var record arrow.Record

	// initialize the record builder
	d.fieldConverter = make([]func(val string), fieldNums)
	fieldList := make([]arrow.Field, fieldNums)
	for idx, name := range orderedNames {
		// name doesn't really matter
		fieldList[idx].Name = name
		fieldList[idx].Nullable = !columnMap[name].NotNullable
		// assign type to field, according to the domaindata
		fieldList[idx].Type = common.Convert2ArrowColumnType(columnMap[name].Type)
	}
	schema := arrow.NewSchema(fieldList, nil)
	d.builder = array.NewRecordBuilder(memory.DefaultAllocator, schema)
	// prepare converter
	for idx := range fieldList {
		d.fieldConverter[idx] = d.initFieldConverter(d.builder.Field(idx))
	}

	defer w.Close()
	rowCount := 0
	for rows.Next() {
		rowCount++
		if err = rows.Scan(scanArgs...); err != nil {
			nlog.Errorf("Read domaindata(%s) postgresql data failed, %s", d.data.DomaindataId, err.Error())
			return err
		}

		// read and convert
		for i, value := range rowValues {
			if value != nil {
				d.fieldConverter[i](string(value))
			} else {
				if fieldList[i].Nullable {
					d.builder.Field(i).AppendNull()
				} else {
					err = errors.Errorf("data column %s not allowed null value", fieldList[i].Name)
					nlog.Error(err)
					return err
				}
			}
		}
		err = d.err
		if err != nil {
			nlog.Errorf("Read domaindata(%s) postgresql failed, %s", d.data.DomaindataId, err.Error())
			return err
		}
		// for better performance batch send to flight
		if rowCount < d.writeBatchSize {
			continue
		}
		rowCount = 0

		record = d.builder.NewRecord()
		record.Retain()
		defer record.Release()
		if err = w.Write(record); err != nil {
			nlog.Errorf("Domaindata(%s) to flight stream failed with error %s", d.data.DomaindataId, err.Error())
			return err
		}
		iCount = iCount + record.NumRows()
		nlog.Debugf("Domaindata(%s) send rows=%d", d.data.DomaindataId, iCount)
	}
	// some record left to send
	if rowCount != 0 {
		rowCount = 0
		record = d.builder.NewRecord()
		record.Retain()
		defer record.Release()
		if err = w.Write(record); err != nil {
			nlog.Errorf("Domaindata(%s) to flight stream failed with error %s", d.data.DomaindataId, err.Error())
			return err
		}
		iCount = iCount + record.NumRows()
		nlog.Debugf("Domaindata(%s) send rows=%d", d.data.DomaindataId, iCount)
	}
	nlog.Infof("Domaindata(%s), file(%s) finish read and send, total row: %d.", d.data.DomaindataId, d.data.RelativeUri, iCount)
	return nil
}
