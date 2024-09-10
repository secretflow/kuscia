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
	"fmt"
	"io"

	csvEncoding "encoding/csv"

	"github.com/apache/arrow/go/v13/arrow"
	"github.com/apache/arrow/go/v13/arrow/array"
	"github.com/apache/arrow/go/v13/arrow/csv"
	"github.com/apache/arrow/go/v13/arrow/flight"
	"github.com/apache/arrow/go/v13/arrow/ipc"
	"github.com/apache/arrow/go/v13/arrow/memory"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/secretflow/kuscia/pkg/datamesh/dataserver/utils"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/datamesh"
)

const CSVDefaultNullValue = "NULL"

type DataMeshDataIOInterface interface {
	Read(ctx context.Context, rc *utils.DataMeshRequestContext, w utils.RecordWriter) error

	Write(ctx context.Context, rc *utils.DataMeshRequestContext, stream *flight.Reader) error

	GetEndpointURI() string
}

// DataFlow(Table): RemoteStorage(FileSystem/OSS/...)  --> DataProxy --> Client
func DataProxyContentToFlightStreamCSV(data *datamesh.DomainData, r io.Reader, w utils.RecordWriter) error {
	//generate arrow schema
	colTypes, err := utils.GenerateArrowColumnType(data)
	if err != nil {
		nlog.Errorf("Domaindata(%s) generate arrow schema error: %s", data.GetDomaindataId(), err.Error())
		return status.Errorf(codes.Internal, "generate arrow schema failed with %s", err.Error())
	}
	schema, _ := utils.GenerateArrowSchema(data)
	// use csv reader,ignore first row, first row is headline.
	csvReader := csv.NewInferringReader(r, csv.WithColumnTypes(colTypes), csv.WithHeader(true), csv.WithNullReader(true, CSVDefaultNullValue))
	defer csvReader.Release()
	defer func() {
		if r := recover(); r != nil {
			nlog.Errorf("Read domaindata(%s) panic %+v", data.DomaindataId, r)
			err = fmt.Errorf("read domaindata(%s) panic: %+v", data.DomaindataId, r)
		}
	}()

	var iCount int64
	for csvReader.Next() {
		record := csvReader.Record()
		if iCount <= 0 && !schema.Equal(record.Schema()) {
			if csvWriter, ok := w.(*utils.FlightRecordWriter); ok {
				w = flight.NewRecordWriter(csvWriter.FlightWriter, ipc.WithSchema(record.Schema()))
				nlog.Debugf("Domaindata(%s) input writer is csv writer schema(%s)", data.GetDomaindataId(), record.Schema().String())
			}
		}
		record.Retain()
		if err := w.Write(record); err != nil {
			nlog.Warnf("Domaindata(%s) to flight stream failed with error %s", data.DomaindataId, err.Error())
			return err
		}
		iCount = iCount + record.NumRows()
		nlog.Debugf("Domaindata(%s) send rows=%d", data.DomaindataId, iCount)
	}
	defer w.Close()
	err = csvReader.Err()
	if err != nil {
		nlog.Errorf("Read domaindata(%s) csv failed, %s", data.DomaindataId, err.Error())
		return err
	}
	nlog.Infof("Domaindata(%s), file(%s) finish read and send, total row: %d.", data.DomaindataId, data.RelativeUri, iCount)
	return nil
}

// DataFlow(Binary): RemoteStorage(FileSystem/OSS/...)  --> DataProxy --> Client
func DataProxyContentToFlightStreamBinary(data *datamesh.DomainData, r io.Reader, w utils.RecordWriter, batchReadSize int) error {
	buffer := make([]byte, batchReadSize)
	hasMoreContent := true
	for hasMoreContent {
		bytesRead, err := r.Read(buffer)
		if err != nil {
			if err == io.EOF {
				if bytesRead <= 0 {
					break
				}
				hasMoreContent = false
			} else {
				return err
			}
		}

		builder := array.NewBinaryBuilder(memory.NewGoAllocator(), arrow.BinaryTypes.Binary)
		builder.Append(buffer[:bytesRead])

		c := array.NewRecord(utils.GenerateBinaryDataArrowSchema(), []arrow.Array{builder.NewArray()}, 1)
		if err := w.Write(c); err != nil {
			nlog.Warnf("Domaindata(%s), file(%s) write batch failed with error:%s", data.DomaindataId, data.RelativeUri, err.Error())
			return err
		}
	}
	w.Close()
	nlog.Infof("Domaindata(%s), file(%s) finish read and send", data.DomaindataId, data.RelativeUri)
	return nil
}

// DataFlow(Table): Client --> DataProxy --> RemoteStorage(FileSystem/OSS/...)
func FlightStreamToDataProxyContentCSV(data *datamesh.DomainData, w io.Writer, reader *flight.Reader) error {
	//generate arrow schema
	schema, err := utils.GenerateArrowSchema(data)
	if err != nil {
		nlog.Errorf("Domaindata(%s) generate arrow schema error: %s", data.GetDomaindataId(), err.Error())
		return status.Errorf(codes.Internal, "generate arrow schema failed with %s", err.Error())
	}
	nlog.Infof("Domaindata(%s) writer schema(%s) and reader schema(%s)", data.GetDomaindataId(), schema.String(), reader.Schema().String())
	// use csv reader,ignore first row, first row is headline.
	csvWriter := csv.NewWriter(w, reader.Schema(), csv.WithHeader(true), csv.WithNullWriter(CSVDefaultNullValue))
	iCount := 0
	for reader.Next() {
		record := reader.Record()
		record.Retain()
		if err := csvWriter.Write(record); err != nil {
			nlog.Warnf("Domaindata(%s) write content to remote failed with error: %s", data.GetDomaindataId(), err.Error())
			return err
		}
		iCount++
	}
	if iCount == 0 {
		// manually write header to avoid read header EOF
		headers := make([]string, len(reader.Schema().Fields()))
		for i := range headers {
			headers[i] = reader.Schema().Field(i).Name
		}
		fileWriter := csvEncoding.NewWriter(w)
		if writeErr := fileWriter.Write(headers); writeErr != nil {
			nlog.Warnf("Domaindata(%s) write empty csv header failed: %s", data.GetDomaindataId(), writeErr.Error())
			return writeErr
		}
		fileWriter.Flush()
	}
	nlog.Infof("Domaindata(%s) write total row: %d.", data.GetDomaindataId(), iCount)
	if err := reader.Err(); err != nil {
		nlog.Warnf("Domaindata(%s) read from arrow flight failed with error: %s", data.GetDomaindataId(), err.Error())
	}
	return nil
}

// DataFlow(Binary): Client --> DataProxy --> RemoteStorage(FileSystem/OSS/...)
func FlightStreamToDataProxyContentBinary(data *datamesh.DomainData, w io.Writer, reader *flight.Reader) error {
	for reader.Next() {
		cnt := reader.Record()
		cnt.Retain()
		defer cnt.Release()

		for idx, col := range cnt.Columns() {
			if !arrow.IsBaseBinary(col.DataType().ID()) {
				nlog.Warnf("Input data column(%d) is not binary, type=%s", idx, col.DataType().String())
				return status.Errorf(codes.InvalidArgument, "domaindata(%s) input data is not binary. type=%s", data.GetDomaindataId(), col.DataType().String())
			}

			if b, ok := col.(*array.Binary); ok {
				for i := 0; i < b.Len(); i++ {
					data := b.Value(i)
					if _, err := w.Write(data); err != nil {
						nlog.Warnf("Write buf length=%d, failed with err: %s", len(data), err.Error())
						return err
					}
				}
			}

		}

	}

	return nil
}
