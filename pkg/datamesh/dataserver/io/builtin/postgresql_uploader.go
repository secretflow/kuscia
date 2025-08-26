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
	"database/sql/driver"
	"fmt"
	"strconv"
	"strings"

	"github.com/apache/arrow/go/v13/arrow"
	"github.com/apache/arrow/go/v13/arrow/array"
	"github.com/apache/arrow/go/v13/arrow/flight"
	"github.com/huandu/go-sqlbuilder"
	"github.com/lib/pq"
	"github.com/pkg/errors"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/datamesh"
)

type PostgresqlUploader struct {
	ctx       context.Context
	db        *sql.DB
	data      *datamesh.DomainData
	query     *datamesh.CommandDomainDataQuery
	nullValue string
}

func NewPostgresqlUploader(ctx context.Context, db *sql.DB, data *datamesh.DomainData, query *datamesh.CommandDomainDataQuery) *PostgresqlUploader {
	return &PostgresqlUploader{
		ctx:       ctx,
		db:        db,
		data:      data,
		query:     query,
		nullValue: "NULL",
	}
}

func (u *PostgresqlUploader) transformColToStringArr(typ arrow.DataType, col arrow.Array) []driver.Value {
	res := make([]driver.Value, col.Len())
	switch typ.(type) {
	case *arrow.BooleanType:
		arr := col.(*array.Boolean)
		for i := 0; i < arr.Len(); i++ {
			if arr.IsValid(i) {
				res[i] = false
				if arr.Value(i) {
					res[i] = true
				}
			} else {
				res[i] = sql.NullBool{}
			}
		}
	case *arrow.Int8Type:
		arr := col.(*array.Int8)
		for i := 0; i < arr.Len(); i++ {
			if arr.IsValid(i) {
				res[i] = strconv.FormatInt(int64(arr.Value(i)), 10)
			} else {
				res[i] = sql.Null[int8]{}
			}
		}
	case *arrow.Int16Type:
		arr := col.(*array.Int16)
		for i := 0; i < arr.Len(); i++ {
			if arr.IsValid(i) {
				res[i] = strconv.FormatInt(int64(arr.Value(i)), 10)
			} else {
				res[i] = sql.NullInt16{}
			}
		}
	case *arrow.Int32Type:
		arr := col.(*array.Int32)
		for i := 0; i < arr.Len(); i++ {
			if arr.IsValid(i) {
				res[i] = strconv.FormatInt(int64(arr.Value(i)), 10)
			} else {
				res[i] = sql.NullInt32{}
			}
		}
	case *arrow.Int64Type:
		arr := col.(*array.Int64)
		for i := 0; i < arr.Len(); i++ {
			if arr.IsValid(i) {
				res[i] = strconv.FormatInt(int64(arr.Value(i)), 10)
			} else {
				res[i] = sql.NullInt64{}
			}
		}
	case *arrow.Uint8Type:
		arr := col.(*array.Uint8)
		for i := 0; i < arr.Len(); i++ {
			if arr.IsValid(i) {
				res[i] = strconv.FormatUint(uint64(arr.Value(i)), 10)
			} else {
				res[i] = sql.Null[uint8]{}
			}
		}
	case *arrow.Uint16Type:
		arr := col.(*array.Uint16)
		for i := 0; i < arr.Len(); i++ {
			if arr.IsValid(i) {
				res[i] = strconv.FormatUint(uint64(arr.Value(i)), 10)
			} else {
				res[i] = sql.Null[uint16]{}
			}
		}
	case *arrow.Uint32Type:
		arr := col.(*array.Uint32)
		for i := 0; i < arr.Len(); i++ {
			if arr.IsValid(i) {
				res[i] = strconv.FormatUint(uint64(arr.Value(i)), 10)
			} else {
				res[i] = sql.Null[uint32]{}
			}
		}
	case *arrow.Uint64Type:
		arr := col.(*array.Uint64)
		for i := 0; i < arr.Len(); i++ {
			if arr.IsValid(i) {
				res[i] = strconv.FormatUint(uint64(arr.Value(i)), 10)
			} else {
				res[i] = sql.Null[uint64]{}
			}
		}
	case *arrow.Float32Type:
		arr := col.(*array.Float32)
		for i := 0; i < arr.Len(); i++ {
			if arr.IsValid(i) {
				res[i] = strconv.FormatFloat(float64(arr.Value(i)), 'g', -1, 32)
			} else {
				res[i] = sql.Null[float32]{}
			}
		}
	case *arrow.Float64Type:
		arr := col.(*array.Float64)
		for i := 0; i < arr.Len(); i++ {
			if arr.IsValid(i) {
				res[i] = strconv.FormatFloat(float64(arr.Value(i)), 'g', -1, 64)
			} else {
				res[i] = sql.NullFloat64{}
			}
		}
	case *arrow.StringType:
		arr := col.(*array.String)
		for i := 0; i < arr.Len(); i++ {
			if arr.IsValid(i) {
				res[i] = arr.Value(i)
			} else {
				res[i] = sql.NullString{}
			}
		}
	default:
		panic(fmt.Errorf("arrow to Postgresql: field has unsupported data type %s", typ.String()))
	}
	return res
}

func (u *PostgresqlUploader) ArrowDataTypeToPostgresqlType(colType arrow.DataType) (string, error) {

	id := colType.ID()
	switch id {
	case arrow.UINT8:
		return "SMALLINT", nil
	case arrow.INT8:
		return "SMALLINT", nil
	case arrow.UINT16:
		return "INTEGER", nil
	case arrow.INT16:
		return "SMALLINT", nil
	case arrow.UINT32:
		return "BIGINT", nil
	case arrow.INT32:
		return "INTEGER", nil
	case arrow.UINT64:
		return "DECIMAL(20, 0)", nil
	case arrow.INT64:
		return "BIGINT", nil
	case arrow.BOOL:
		return "BOOLEAN", nil
	case arrow.STRING, arrow.BINARY, arrow.LARGE_BINARY:
		return "TEXT", nil
	case arrow.FLOAT32:
		return "REAL", nil
	case arrow.FLOAT64:
		return "DOUBLE PRECISION", nil
	default:
		return "", fmt.Errorf("unsupported arrow data type `%s`", colType)
	}
}

func (u *PostgresqlUploader) PrepareOutputTable(tableName string, columnNames []string, fields []arrow.Field) error {
	// if exist, try to drop it
	_, err := u.db.Exec("DROP TABLE IF EXISTS " + tableName)
	// maybe permission denied, try delete
	if err != nil {
		nlog.Infof("Sql drop with err(%s), try to sql delete", err)
		_, deleteErr := u.db.Exec("DELETE FROM " + tableName)
		// if there is no table, we can try to create
		if sqlErr, ok := deleteErr.(*pq.Error); ok && sqlErr.Code.Name() == "undefined_table" {
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
		postgresqlType, typeErr := u.ArrowDataTypeToPostgresqlType(field.Type)
		if typeErr != nil {
			return fmt.Errorf("arrow to Postgresql: field has unsupported data type `%s`", field.Type)
		}
		ctb.Define(columnNames[idx], postgresqlType)
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

func (u *PostgresqlUploader) FlightStreamToDataProxyContentPostgresql(reader *flight.Reader) (err error) {
	// read data from flight.Reader, and write to postgresql transaction

	// schema field check
	backTickHeaders := make([]string, reader.Schema().NumFields())
	for idx, col := range reader.Schema().Fields() {
		if strings.IndexByte(col.Name, '"') != -1 {
			err = errors.Errorf("invalid column name(%s). For safety reason, backtick is not allowed", col.Name)
			nlog.Error(err)
			return err
		}
		backTickHeaders[idx] = "\"" + col.Name + "\""
	}

	if strings.IndexByte(u.data.RelativeUri, '"') != -1 {
		err = errors.Errorf("invalid table name(%s). For safety reason, backtick is not allowed", u.data.RelativeUri)
		nlog.Error(err)
		return err
	}
	tableName := "\"" + u.data.RelativeUri + "\""
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
		nlog.Errorf("Prepare Postgresql output table failed(%s)", err)
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

	insertSql := sqlbuilder.InsertInto(tableName).Cols(backTickHeaders...).String()
	nlog.Infof("Insert sql without values (%s)", insertSql)

	for reader.Next() {
		var execErr error
		func() {
			record := reader.Record()
			record.Retain()
			defer record.Release()

			numCols := record.NumCols()
			recs := make([][]any, record.NumRows())
			for i := range recs {
				recs[i] = make([]any, numCols)
			}

			for j, col := range record.Columns() {
				rows := u.transformColToStringArr(reader.Schema().Field(j).Type, col)
				for i, row := range rows {
					recs[i][j] = row
				}
			}

			execErr = insertSafeBatch(tx, insertSql, recs)
			if execErr != nil {
				nlog.Errorf("Insert to Postgresql failed(%s)", execErr)
				return
			}

			iCount += int(record.NumRows())
		}()
		if execErr != nil {
			return execErr
		}
	}
	if err := reader.Err(); err != nil {
		// in this case, stmt.Exec are all success, try to commit, rather than fail
		nlog.Warnf("Domaindata(%s) read from arrow flight failed with error: %s. Postgresql upload result may have problems", u.data.DomaindataId, err)
	}
	nlog.Infof("Domaindata(%s) write total row: %d.", u.data.DomaindataId, iCount)
	return nil
}

func insertBatch(tx *sql.Tx, ib *sqlbuilder.InsertBuilder, batch [][]interface{}) error {

	if len(batch) == 0 {
		nlog.Warnf("Domaindata write total row: 0, insertSql: %s", ib)
		return nil
	}

	numRecords, numColumns := len(batch), len(batch[0])
	args := make([]interface{}, 0, numRecords*numColumns)

	for _, row := range batch {
		ib.Values(row...)
		args = append(args, row...)
	}
	ib.SetFlavor(sqlbuilder.PostgreSQL)
	insertSql := ib.String()

	stmt, err := tx.Prepare(insertSql)
	if err != nil {
		nlog.Errorf("Prepare insert failed(%s)", err)
		return err
	}

	// close stmt first
	defer func() {
		stmtErr := stmt.Close()
		if stmtErr != nil {
			nlog.Errorf("Stmt close error")
		}
	}()
	// stmt is used here to process the precompiled SQL, but the current insertion batch is not fixed,
	// and the pre-compilation still needs to be executed every time
	_, err = stmt.Exec(args...)
	return err
}

func insertSafeBatch(tx *sql.Tx, insertSql string, batch [][]interface{}) error {

	numRecords, numColumns := len(batch), len(batch[0])
	maxRows := DEFAULT_MAX_PLACEHOLDER / numColumns

	for i := 0; i < numRecords; i += maxRows {
		endIndex := min(i+maxRows, numRecords)

		if err := insertBatch(tx, sqlbuilder.NewInsertBuilder().SQL(insertSql), batch[i:endIndex]); err != nil {
			return err
		}
	}
	return nil
}
