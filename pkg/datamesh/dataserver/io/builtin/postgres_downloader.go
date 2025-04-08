package builtin

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
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

type PostgresDownloader struct {
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

func NewPostgresDownloader(ctx context.Context, db *sql.DB, data *datamesh.DomainData, query *datamesh.CommandDomainDataQuery) *PostgresDownloader {
	return &PostgresDownloader{
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
func (d *PostgresDownloader) generateQueryColumns() (map[string]*v1alpha1.DataColumn, []string, error) {
	// create domaindata and query column map[columnName->columnType]
	if len(d.data.Columns) == 0 {
		return nil, nil, errors.Errorf("No data column available, terminate reading")
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
			return nil, nil, errors.Errorf("Query column(%s) is not defined in domaindata(%s)", column, d.data.DomaindataId)
		}
		orderedNames = append(orderedNames, v.Name)
	}

	return domaindataColumnMap, orderedNames, nil
}

func (d *PostgresDownloader) checkSQLSupport(columnMap map[string]*v1alpha1.DataColumn, orderedNames []string) error {
	// check backtick, check type (only key in orderedNames)
	for _, val := range orderedNames {
		k, v := val, columnMap[val]
		if strings.IndexByte(k, '`') != -1 {
			return errors.Errorf("Invalid column name(%s). For safety reason, backtick is not allowed", k)
		}
		if strings.Contains(v.Type, "date") || strings.Contains(v.Type, "binary") {
			return errors.Errorf("Type(%s) is not supported now, please consider change another type", v.Type)
		}
	}
	return nil
}

// following parse functions are copied and modified from arrow v13@v13.0.0 /csv/reader
func (d *PostgresDownloader) parseBool(field array.Builder, str string) {
	v, err := strconv.ParseBool(str)
	if err != nil {
		d.err = fmt.Errorf("%w: unrecognized boolean: %s", err, str)
		field.AppendNull()
		return
	}

	field.(*array.BooleanBuilder).Append(v)
}

func (d *PostgresDownloader) parseInt8(field array.Builder, str string) {
	v, err := strconv.ParseInt(str, 10, 8)
	if err != nil && d.err == nil {
		d.err = err
		field.AppendNull()
		return
	}

	field.(*array.Int8Builder).Append(int8(v))
}

func (d *PostgresDownloader) parseInt16(field array.Builder, str string) {
	v, err := strconv.ParseInt(str, 10, 16)
	if err != nil && d.err == nil {
		d.err = err
		field.AppendNull()
		return
	}

	field.(*array.Int16Builder).Append(int16(v))
}

func (d *PostgresDownloader) parseInt32(field array.Builder, str string) {
	v, err := strconv.ParseInt(str, 10, 32)
	if err != nil && d.err == nil {
		d.err = err
		field.AppendNull()
		return
	}

	field.(*array.Int32Builder).Append(int32(v))
}

func (d *PostgresDownloader) parseInt64(field array.Builder, str string) {
	v, err := strconv.ParseInt(str, 10, 64)
	if err != nil && d.err == nil {
		d.err = err
		field.AppendNull()
		return
	}

	field.(*array.Int64Builder).Append(v)
}

func (d *PostgresDownloader) parseUint8(field array.Builder, str string) {
	v, err := strconv.ParseUint(str, 10, 8)
	if err != nil && d.err == nil {
		d.err = err
		field.AppendNull()
		return
	}

	field.(*array.Uint8Builder).Append(uint8(v))
}

func (d *PostgresDownloader) parseUint16(field array.Builder, str string) {
	v, err := strconv.ParseUint(str, 10, 16)
	if err != nil && d.err == nil {
		d.err = err
		field.AppendNull()
		return
	}

	field.(*array.Uint16Builder).Append(uint16(v))
}

func (d *PostgresDownloader) parseUint32(field array.Builder, str string) {
	v, err := strconv.ParseUint(str, 10, 32)
	if err != nil && d.err == nil {
		d.err = err
		field.AppendNull()
		return
	}

	field.(*array.Uint32Builder).Append(uint32(v))
}

func (d *PostgresDownloader) parseUint64(field array.Builder, str string) {
	v, err := strconv.ParseUint(str, 10, 64)
	if err != nil && d.err == nil {
		d.err = err
		field.AppendNull()
		return
	}

	field.(*array.Uint64Builder).Append(v)
}

func (d *PostgresDownloader) parseFloat32(field array.Builder, str string) {
	v, err := strconv.ParseFloat(str, 32)
	if err != nil && d.err == nil {
		d.err = err
		field.AppendNull()
		return
	}
	field.(*array.Float32Builder).Append(float32(v))
}

func (d *PostgresDownloader) parseFloat64(field array.Builder, str string) {
	v, err := strconv.ParseFloat(str, 64)
	if err != nil && d.err == nil {
		d.err = err
		field.AppendNull()
		return
	}
	field.(*array.Float64Builder).Append(v)
}

func (d *PostgresDownloader) initFieldConverter(bldr array.Builder) func(string) {
	switch bldr.Type().(type) {
	case *arrow.BooleanType:
		return func(str string) {
			d.parseBool(bldr, str)
		}
	case *arrow.Int8Type:
		return func(str string) {
			d.parseInt8(bldr, str)
		}
	case *arrow.Int16Type:
		return func(str string) {
			d.parseInt16(bldr, str)
		}
	case *arrow.Int32Type:
		return func(str string) {
			d.parseInt32(bldr, str)
		}
	case *arrow.Int64Type:
		return func(str string) {
			d.parseInt64(bldr, str)
		}
	case *arrow.Uint8Type:
		return func(str string) {
			d.parseUint8(bldr, str)
		}
	case *arrow.Uint16Type:
		return func(str string) {
			d.parseUint16(bldr, str)
		}
	case *arrow.Uint32Type:
		return func(str string) {
			d.parseUint32(bldr, str)
		}
	case *arrow.Uint64Type:
		return func(str string) {
			d.parseUint64(bldr, str)
		}
	case *arrow.Float32Type:
		return func(str string) {
			d.parseFloat32(bldr, str)
		}
	case *arrow.Float64Type:
		return func(str string) {
			d.parseFloat64(bldr, str)
		}
	case *arrow.StringType:
		return func(str string) {
			bldr.(*array.StringBuilder).Append(str)
		}

	default:
		panic(fmt.Errorf("postgresql to arrow conversion: unhandled field type %T", bldr.Type()))
	}
}

func (d *PostgresDownloader) DataProxyContentToFlightStreamSQL(w utils.RecordWriter) (err error) {
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
	if strings.IndexByte(d.data.RelativeUri, '`') != -1 {
		err = errors.Errorf("Invalid table name(%s). For safety reason, backtick is not allowed", d.data.RelativeUri)
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
	nlog.Info("posgresql", sql)
	rows, err := d.db.Query(sql)
	if err != nil {
		nlog.Errorf("postgresql query failed(%s)", err)
		return err
	}

	// avoid panic
	defer func() {
		if r := recover(); r != nil {
			nlog.Errorf("Read domaindata(%s) panic %+v", d.data.DomaindataId, r)
			err = fmt.Errorf("read domaindata(%s) panic: %+v", d.data.DomaindataId, r)
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
