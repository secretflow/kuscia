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

package builtin

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/apache/arrow/go/v13/arrow"
	"github.com/apache/arrow/go/v13/arrow/array"
	"github.com/apache/arrow/go/v13/arrow/flight"
	"github.com/go-sql-driver/mysql"
	"github.com/huandu/go-sqlbuilder"
	"github.com/pkg/errors"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/datamesh"
)

type MySQLUploader struct {
	ctx       context.Context
	db        *sql.DB
	data      *datamesh.DomainData
	query     *datamesh.CommandDomainDataQuery
	nullValue string
}

// define some mysql error
const (
	ER_NO_SUCH_TABLE uint16 = 1146
)

// max placeholder. this means cols * rols should < 65535
const (
	DEFAULT_MAX_PLACEHOLDER = 65535
)

func NewMySQLUploader(ctx context.Context, db *sql.DB, data *datamesh.DomainData, query *datamesh.CommandDomainDataQuery) *MySQLUploader {
	return &MySQLUploader{
		ctx:       ctx,
		db:        db,
		data:      data,
		query:     query,
		nullValue: "NULL",
	}
}

func (u *MySQLUploader) transformColToStringArr(typ arrow.DataType, col arrow.Array) []string {
	res := make([]string, col.Len())
	switch typ.(type) {
	case *arrow.BooleanType:
		arr := col.(*array.Boolean)
		for i := 0; i < arr.Len(); i++ {
			if arr.IsValid(i) {
				res[i] = "0"
				if arr.Value(i) {
					res[i] = "1"
				}
			} else {
				res[i] = u.nullValue
			}
		}
	case *arrow.Int8Type:
		arr := col.(*array.Int8)
		for i := 0; i < arr.Len(); i++ {
			if arr.IsValid(i) {
				res[i] = strconv.FormatInt(int64(arr.Value(i)), 10)
			} else {
				res[i] = u.nullValue
			}
		}
	case *arrow.Int16Type:
		arr := col.(*array.Int16)
		for i := 0; i < arr.Len(); i++ {
			if arr.IsValid(i) {
				res[i] = strconv.FormatInt(int64(arr.Value(i)), 10)
			} else {
				res[i] = u.nullValue
			}
		}
	case *arrow.Int32Type:
		arr := col.(*array.Int32)
		for i := 0; i < arr.Len(); i++ {
			if arr.IsValid(i) {
				res[i] = strconv.FormatInt(int64(arr.Value(i)), 10)
			} else {
				res[i] = u.nullValue
			}
		}
	case *arrow.Int64Type:
		arr := col.(*array.Int64)
		for i := 0; i < arr.Len(); i++ {
			if arr.IsValid(i) {
				res[i] = strconv.FormatInt(int64(arr.Value(i)), 10)
			} else {
				res[i] = u.nullValue
			}
		}
	case *arrow.Uint8Type:
		arr := col.(*array.Uint8)
		for i := 0; i < arr.Len(); i++ {
			if arr.IsValid(i) {
				res[i] = strconv.FormatUint(uint64(arr.Value(i)), 10)
			} else {
				res[i] = u.nullValue
			}
		}
	case *arrow.Uint16Type:
		arr := col.(*array.Uint16)
		for i := 0; i < arr.Len(); i++ {
			if arr.IsValid(i) {
				res[i] = strconv.FormatUint(uint64(arr.Value(i)), 10)
			} else {
				res[i] = u.nullValue
			}
		}
	case *arrow.Uint32Type:
		arr := col.(*array.Uint32)
		for i := 0; i < arr.Len(); i++ {
			if arr.IsValid(i) {
				res[i] = strconv.FormatUint(uint64(arr.Value(i)), 10)
			} else {
				res[i] = u.nullValue
			}
		}
	case *arrow.Uint64Type:
		arr := col.(*array.Uint64)
		for i := 0; i < arr.Len(); i++ {
			if arr.IsValid(i) {
				res[i] = strconv.FormatUint(uint64(arr.Value(i)), 10)
			} else {
				res[i] = u.nullValue
			}
		}
	case *arrow.Float32Type:
		arr := col.(*array.Float32)
		for i := 0; i < arr.Len(); i++ {
			if arr.IsValid(i) {
				res[i] = strconv.FormatFloat(float64(arr.Value(i)), 'g', -1, 32)
			} else {
				res[i] = u.nullValue
			}
		}
	case *arrow.Float64Type:
		arr := col.(*array.Float64)
		for i := 0; i < arr.Len(); i++ {
			if arr.IsValid(i) {
				res[i] = strconv.FormatFloat(float64(arr.Value(i)), 'g', -1, 64)
			} else {
				res[i] = u.nullValue
			}
		}
	case *arrow.StringType:
		arr := col.(*array.String)
		for i := 0; i < arr.Len(); i++ {
			if arr.IsValid(i) {
				res[i] = arr.Value(i)
			} else {
				res[i] = u.nullValue
			}
		}
	default:
		panic(fmt.Errorf("arrow to MySQL: field has unsupported data type %s", typ.String()))
	}
	return res
}

func (u *MySQLUploader) ArrowDataTypeToMySQLType(colType arrow.DataType) string {
	switch colType {
	case arrow.PrimitiveTypes.Uint8:
		return "TINYINT UNSIGNED"
	case arrow.PrimitiveTypes.Int8:
		return "TINYINT SIGNED"
	case arrow.PrimitiveTypes.Uint16:
		return "SMALLINT UNSIGNED"
	case arrow.PrimitiveTypes.Int16:
		return "SMALLINT SIGNED"
	case arrow.PrimitiveTypes.Uint32:
		return "INT UNSIGNED"
	case arrow.PrimitiveTypes.Int32:
		return "INT SIGNED"
	case arrow.PrimitiveTypes.Uint64:
		return "BIGINT UNSIGNED"
	case arrow.PrimitiveTypes.Int64:
		return "BIGINT SIGNED"
	case arrow.FixedWidthTypes.Boolean:
		return "TINYINT(1)"
	case arrow.BinaryTypes.String:
		return "TEXT"
	case arrow.PrimitiveTypes.Float32:
		return "FLOAT"
	case arrow.PrimitiveTypes.Float64:
		return "DOUBLE"
	default:
		panic(fmt.Errorf("create MySQL Table: field has unsupported data type %s", colType))
	}
}

func (u *MySQLUploader) PrepareOutputTable(tableName string, columnNames []string, fields []arrow.Field) error {
	// if exist, try to drop it
	_, err := u.db.Exec("DROP TABLE IF EXISTS " + tableName)
	// maybe permission denied, try delete
	if err != nil {
		nlog.Infof("Sql drop with err(%s), try to sql delete", err)
		_, deleteErr := u.db.Exec("DELETE FROM " + tableName)
		// if there is no table, we can try to create
		if sqlErr, ok := deleteErr.(*mysql.MySQLError); ok && sqlErr.Number == uint16(ER_NO_SUCH_TABLE) {
			nlog.Infof("Table(%s) not exist", tableName)
		} else {
			// table exists but can't delete, or other error
			return deleteErr
		}
	}
	nlog.Infof("Sql clear data success")
	ctb := sqlbuilder.NewCreateTableBuilder()
	ctb.CreateTable(tableName)
	for idx, field := range fields {
		ctb.Define(columnNames[idx], u.ArrowDataTypeToMySQLType(field.Type))
	}
	sql := ctb.String()
	nlog.Infof("Prepare output table sql(%s)", sql)
	_, err = u.db.Exec(sql)
	if err != nil {
		return err
	}
	nlog.Infof("Table create success")
	return nil
}

func (u *MySQLUploader) FlightStreamToDataProxyContentMySQL(reader *flight.Reader) (err error) {
	// read data from flight.Reader, and write to mysql transaction

	// schema field check
	backTickHeaders := make([]string, reader.Schema().NumFields())
	for idx, col := range reader.Schema().Fields() {
		if strings.IndexByte(col.Name, '`') != -1 {
			err = errors.Errorf("invalid column name(%s). For safety reason, backtick is not allowed", col.Name)
			nlog.Error(err)
			return err
		}
		backTickHeaders[idx] = "`" + col.Name + "`"
	}

	if strings.IndexByte(u.data.RelativeUri, '`') != -1 {
		err = errors.Errorf("invalid table name(%s). For safety reason, backtick is not allowed", u.data.RelativeUri)
		nlog.Error(err)
		return err
	}
	tableName := "`" + u.data.RelativeUri + "`"
	iCount := 0

	// in the end, avoid panic
	defer func() {
		if r := recover(); r != nil {
			nlog.Errorf("Write domaindata(%s) panic %+v", u.data.DomaindataId, r)
			err = fmt.Errorf("write domaindata(%s) panic: %+v", u.data.DomaindataId, r)
		}
	}()

	// prepare table
	err = u.PrepareOutputTable(tableName, backTickHeaders, reader.Schema().Fields())
	if err != nil {
		nlog.Errorf("Prepare MySQL output table failed(%s)", err)
		return err
	}

	tx, err := u.db.Begin()
	if err != nil {
		nlog.Errorf("Begin Transaction failed(%s)", err)
		return err
	}

	// then check the error, and commit/rollback the transaction
	defer func() {
		if err == nil {
			nlog.Infof("Upload no error, ready to commit")
			// return err to upper function
			err = tx.Commit()
			if err != nil {
				nlog.Errorf("Commit Error(%s)", err)
			} else {
				nlog.Infof("Transaction commit success")
			}
		} else {
			// keep original error
			rollErr := tx.Rollback()
			nlog.Errorf("Rollback Error(%s)", rollErr)
		}
	}()

	for reader.Next() {
		record := reader.Record()
		record.Retain()
		defer record.Release()
		// read field data from record
		recs := make([][]interface{}, record.NumRows())
		for i := range recs {
			recs[i] = make([]interface{}, record.NumCols())
		}
		for j, col := range record.Columns() {
			rows := u.transformColToStringArr(reader.Schema().Field(j).Type, col)
			for i, row := range rows {
				recs[i][j] = row
			}
		}
		ib := sqlbuilder.InsertInto(tableName).Cols(backTickHeaders...)
		placeholderCount := 0
		// upload to sql
		for _, row := range recs {
			if placeholderCount+len(row) >= DEFAULT_MAX_PLACEHOLDER {
				err = execOnce(ib, tx)
				if err != nil {
					return err
				}
				placeholderCount = 0
				// start a new insert
				ib = sqlbuilder.InsertInto(tableName).Cols(backTickHeaders...)
			}
			ib.Values(row...)
			placeholderCount += len(row)
		}
		if placeholderCount != 0 {
			err = execOnce(ib, tx)
			if err != nil {
				return err
			}
		}
		iCount += int(record.NumRows())
		nlog.Debugf("send rows=%d", iCount)
	}
	if err := reader.Err(); err != nil {
		// in this case, stmt.Exec are all success, try to commit, rather than fail
		nlog.Warnf("Domaindata(%s) read from arrow flight failed with error: %s. MySQL upload result may have problems", u.data.DomaindataId, err)
	}
	nlog.Infof("Domaindata(%s) write total row: %d.", u.data.DomaindataId, iCount)
	return nil
}

func execOnce(ib *sqlbuilder.InsertBuilder, tx *sql.Tx) error {
	sql, args := ib.Build()
	stmt, err := tx.Prepare(sql)
	if err != nil {
		nlog.Errorf("Prepare sql(%s) failed with error: %s", sql, err)
		return err
	}
	_, err = stmt.Exec(args...)
	return err
}
