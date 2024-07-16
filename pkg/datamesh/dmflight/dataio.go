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

package dmflight

import (
	"context"
	"io"
	"strings"

	"github.com/apache/arrow/go/v13/arrow"
	"github.com/apache/arrow/go/v13/arrow/array"
	"github.com/apache/arrow/go/v13/arrow/csv"
	"github.com/apache/arrow/go/v13/arrow/flight"
	"github.com/apache/arrow/go/v13/arrow/memory"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/datamesh"
)

type DataMeshDataIOInterface interface {
	Read(ctx context.Context, rc *DataMeshRequestContext, w *flight.Writer) error

	Write(ctx context.Context, rc *DataMeshRequestContext, stream *flight.Reader) error

	GetEndpointURI() string
}

// DataFlow(Table): RemoteStorage(FileSystem/OSS/...)  --> DataProxy --> Client
func DataProxyContentToFlightStreamCSV(data *datamesh.DomainData, r io.Reader, w *flight.Writer) error {
	//generate arrow schema
	schema, err := GenerateArrowSchema(data)
	if err != nil {
		nlog.Errorf("Domaindata(%s) generate arrow schema error: %s", data.GetDomaindataId(), err.Error())
		return status.Errorf(codes.Internal, "generate arrow schema failed with %s", err.Error())
	}

	// use csv reader
	csvReader := csv.NewReader(r, schema)

	for csvReader.Next() {
		record := csvReader.Record()
		record.Retain()
		if err := w.Write(record); err != nil {
			nlog.Warnf("Domaindata(%s) to flight stream failed with error %s", data.DomaindataId, err.Error())
			return err
		}
		nlog.Debugf("Domaindata(%s) send rows=%d", data.DomaindataId, record.NumRows())
	}

	defer csvReader.Release()

	nlog.Infof("Domaindata(%s), file(%s) finish read and send", data.DomaindataId, data.RelativeUri)

	return nil
}

// DataFlow(Binary): RemoteStorage(FileSystem/OSS/...)  --> DataProxy --> Client
func DataProxyContentToFlightStreamBinary(data *datamesh.DomainData, r io.Reader, w *flight.Writer, batchReadSize int) error {
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

		c := array.NewRecord(GenerateBinaryDataArrowSchema(), []arrow.Array{builder.NewArray()}, 1)
		if err := w.Write(c); err != nil {
			nlog.Warnf("Domaindata(%s), file(%s) write batch failed with error:%s", data.DomaindataId, data.RelativeUri, err.Error())
			return err
		}
	}

	nlog.Infof("Domaindata(%s), file(%s) finish read and sand", data.DomaindataId, data.RelativeUri)

	return nil
}

// DataFlow(Table): Client --> DataProxy --> RemoteStorage(FileSystem/OSS/...)
func FlightStreamToDataProxyContentCSV(data *datamesh.DomainData, w io.Writer, reader *flight.Reader) error {
	for reader.Next() {
		record := reader.Record()
		record.Retain()
		columns := record.Columns()
		strColumns := make([]string, 0)
		for index := 0; index < len(columns); index++ {
			strColumns = append(strColumns, columns[index].ValueStr(0))
		}

		if _, err := w.Write([]byte(strings.Join(strColumns, ","))); err != nil {
			nlog.Warnf("Domaindata(%s) write content to remote failed with error: %s", data.GetDomaindataId(), err.Error())
			return err
		}
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
