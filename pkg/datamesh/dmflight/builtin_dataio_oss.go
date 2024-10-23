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

package dmflight

import (
	"context"
	"path"
	"strings"

	"github.com/apache/arrow/go/v13/arrow/flight"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/pkg/errors"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/datamesh"
)

const minio string = "minio"

// BuiltinOssIO defined the oss read & write methond
type BuiltinOssIO struct {
	batchReadSize int
}

func NewBuiltinOssIOChannel() DataMeshDataIOInterface {
	return &BuiltinOssIO{
		batchReadSize: 4096,
	}
}

// new oss client
func (o *BuiltinOssIO) newOssSession(config *datamesh.OssDataSourceInfo) (*s3.S3, error) {
	nlog.Debugf("Open oss remote endpoint(%s), bucket(%s)", config.Endpoint, config.Bucket)
	pathStyle := aws.Bool(false)
	region := ""
	if strings.ToLower(config.StorageType) == minio {
		pathStyle = aws.Bool(true)
		region = minio
	} else {
		region, _ = ParseRegionFromEndpoint(config.Endpoint)
	}

	sess, err := session.NewSession(&aws.Config{
		Credentials:      credentials.NewStaticCredentials(config.AccessKeyId, config.AccessKeySecret, ""),
		Endpoint:         &config.Endpoint,
		Region:           &region,
		S3ForcePathStyle: pathStyle,
	})

	if err != nil {
		nlog.Warnf("Oss(%s) create session failed with error: %s", config.Endpoint, err.Error())
		return nil, err
	}

	return s3.New(sess), nil
}

// DataFlow: RemoteStorage(FileSystem/OSS/...)  --> DataProxy --> Client
func (o *BuiltinOssIO) Read(ctx context.Context, rc *DataMeshRequestContext, w *flight.Writer) error {
	// init bucket & object stream
	dd, ds, err := rc.GetDomainDataAndSource(ctx)
	if err != nil {
		return err
	}

	client, err := o.newOssSession(ds.Info.Oss)
	if err != nil {
		nlog.Errorf("Create oss client error: %s", err.Error())
		return err
	}
	obj, err := client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(ds.Info.Oss.Bucket),
		Key:    aws.String(path.Join(ds.Info.Oss.Prefix, dd.RelativeUri))})
	if err != nil {
		nlog.Error("Oss client get object error: ", err)
		return err
	}
	defer obj.Body.Close()

	switch rc.GetTransferContentType() {
	case datamesh.ContentType_RAW:
		return DataProxyContentToFlightStreamBinary(dd, obj.Body, w, o.batchReadSize)
	case datamesh.ContentType_CSV, datamesh.ContentType_Table:
		return DataProxyContentToFlightStreamCSV(dd, obj.Body, w)
	default:
		return errors.Errorf("invalidate content-type: %s", rc.GetTransferContentType().String())
	}
}

// DataFlow: Client --> DataProxy --> RemoteStorage(FileSystem/OSS/...)
func (o *BuiltinOssIO) Write(ctx context.Context, rc *DataMeshRequestContext, reader *flight.Reader) error {
	dd, ds, err := rc.GetDomainDataAndSource(ctx)

	if err != nil {
		return err
	}

	objectKey := path.Join(ds.Info.Oss.Prefix, dd.RelativeUri)
	nlog.Infof("DomainData(%s) try save to remote oss(%s/%s)", dd.DomaindataId, ds.Info.Oss.Endpoint,
		path.Join(ds.Info.Oss.Bucket, objectKey))

	// init oss client & object stream
	client, err := o.newOssSession(ds.Info.Oss)
	if err != nil {
		nlog.Errorf("Create oss client error: %s", err.Error())
		return err
	}

	/*if ok, err := client.IsObjectExist(objectKey); err == nil && ok {
		nlog.Warnf("Oss remote file exists, can't upload %s", objectKey)
		return status.Errorf(codes.AlreadyExists, "oss remote file exists, can't upload %s", objectKey)
	}*/

	exchanger := NewOSSUploader(ctx, client, ds.Info.Oss.Bucket, objectKey, 5*1024*1024)
	defer exchanger.Close()

	switch rc.GetTransferContentType() {
	case datamesh.ContentType_RAW:
		err = FlightStreamToDataProxyContentBinary(dd, exchanger, reader)
	case datamesh.ContentType_CSV, datamesh.ContentType_Table:
		err = FlightStreamToDataProxyContentCSV(dd, exchanger, reader)
	default:
		return errors.Errorf("invalidate content-type: %s", rc.GetTransferContentType().String())
	}

	if err == nil { // no error, close the writer
		if err = exchanger.FinishUpload(); err != nil {
			nlog.Warnf("Upload to oss failed with %s", err.Error())
		}
	}

	return err
}

func (o *BuiltinOssIO) GetEndpointURI() string {
	return BuiltinFlightServerEndpointURI
}
