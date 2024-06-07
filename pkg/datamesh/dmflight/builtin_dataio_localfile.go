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
	"os"
	"path"

	"github.com/apache/arrow/go/v13/arrow/flight"
	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/datamesh"
)

type BuiltinLocalFileIO struct {
	batchReadSize int
}

func NewBuiltinLocalFileIOChannel() DataMeshDataIOInterface {
	return &BuiltinLocalFileIO{
		batchReadSize: 4096,
	}
}

// DataFlow: RemoteStorage(FileSystem/OSS/...)  --> DataProxy --> Client
func (fio *BuiltinLocalFileIO) Read(ctx context.Context, rc *DataMeshRequestContext, w *flight.Writer) error {
	data, ds, err := rc.GetDomainDataAndSource(ctx)

	if err != nil {
		return err
	}

	filePath := path.Join(ds.Info.Localfs.Path, data.RelativeUri)

	if _, err := os.Stat(filePath); err != nil {
		nlog.Infof("DomainData(%s) file(%s) stat failed with: %s", data.DomaindataId, filePath, err.Error())
		return err
	}

	file, err := os.Open(filePath)
	if err != nil {
		nlog.Warnf("DomainData(%s) opening file(%s) with error: %s", data.DomaindataId, filePath, err.Error())
		return err
	}
	defer file.Close()

	switch rc.GetTransferContentType() {
	case datamesh.ContentType_RAW:
		return DataProxyContentToFlightStreamBinary(data, file, w, fio.batchReadSize)
	case datamesh.ContentType_CSV, datamesh.ContentType_Table:
		return DataProxyContentToFlightStreamCSV(data, file, w)
	default:
		return errors.Errorf("invalidate content-type: %s", rc.GetTransferContentType().String())
	}
}

// DataFlow: Client --> DataProxy --> RemoteStorage(FileSystem/OSS/...)
func (fio *BuiltinLocalFileIO) Write(ctx context.Context, rc *DataMeshRequestContext, reader *flight.Reader) error {
	data, ds, err := rc.GetDomainDataAndSource(ctx)

	if err != nil {
		return err
	}

	filePath := path.Join(ds.Info.Localfs.Path, data.RelativeUri)

	nlog.Infof("DomainData(%s) try save to file(%s)", data.DomaindataId, filePath)

	if _, err := os.Stat(filePath); err == nil {
		nlog.Infof("DomainData(%s) file(%s) file exists, can't upload", data.DomaindataId, filePath)
		return status.Errorf(codes.AlreadyExists, "file exists, can't upload %s", data.RelativeUri)
	}

	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		nlog.Warnf("DomainData(%s) opening file(%s) to write with error: %s", data.DomaindataId, filePath, err.Error())
		return err
	}
	defer file.Close()

	switch rc.GetTransferContentType() {
	case datamesh.ContentType_RAW:
		return FlightStreamToDataProxyContentBinary(data, file, reader)
	case datamesh.ContentType_CSV, datamesh.ContentType_Table:
		return FlightStreamToDataProxyContentCSV(data, file, reader)
	default:
		return errors.Errorf("invalidate content-type: %s", rc.GetTransferContentType().String())
	}
}

func (fio *BuiltinLocalFileIO) GetEndpointURI() string {
	return BuiltinFlightServerEndpointURI
}
