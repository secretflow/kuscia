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
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/util/wait"
)

func TestOssWriter_New(t *testing.T) {
	t.Parallel()
	writer := NewOSSUploader(context.Background(), nil, "xyz", "test.raw", 2*1024)
	assert.NotNil(t, writer)
	assert.Nil(t, writer.client)
	assert.Equal(t, "xyz", writer.bucket)
	assert.Equal(t, "test.raw", writer.objectKey)
	assert.Equal(t, 2*1024, writer.partSizeBytes)
	assert.False(t, writer.isFinished)
	assert.Nil(t, writer.uploadLastError)
	assert.NotNil(t, writer.buffer)
	assert.Equal(t, 0, writer.buffer.Len())
}

func TestOssWriter_Write_Cache(t *testing.T) {
	t.Parallel()

	ow := NewOSSUploader(context.Background(), nil, "xyz", "test.raw", 2*1024)
	n, err := ow.Write([]byte("abcdefg"))
	assert.NoError(t, err)
	assert.Equal(t, len("abcdefg"), n)
	assert.Nil(t, ow.uploadLastError)
	assert.Equal(t, n, ow.buffer.Len())
	assert.Equal(t, "abcdefg", ow.buffer.String())
	assert.False(t, ow.isFinished)
}

func initMockOssBackend(t *testing.T) (*s3mem.Backend, *session.Session, *httptest.Server) {
	backend := s3mem.New()
	faker := gofakes3.New(backend)
	ts := httptest.NewServer(faker.Server())

	assert.NoError(t, backend.CreateBucket("xyz"))

	region := "minio"
	sess, err := session.NewSession(&aws.Config{
		Credentials:      credentials.NewStaticCredentials("testid", "testkey", ""),
		Endpoint:         &ts.URL,
		Region:           &region,
		S3ForcePathStyle: aws.Bool(true),
	})
	assert.NoError(t, err)
	assert.NotNil(t, sess)

	return backend, sess, ts
}

func TestOssWriter_Write_MultiUpload(t *testing.T) {
	t.Parallel()

	backend, sess, ts := initMockOssBackend(t)
	defer ts.Close()

	ow := NewOSSUploader(context.Background(), s3.New(sess), "xyz", "test.raw", 128)

	// split to two part: 128, 72.
	n, err := ow.Write(make([]byte, 200))
	assert.NoError(t, err)
	assert.Equal(t, 200, n)
	assert.Equal(t, 0, ow.buffer.Len())

	assert.False(t, ow.isFinished)
	assert.True(t, ow.isUploadStarted)
	assert.NoError(t, ow.FinishUpload())
	obj, err := backend.GetObject("xyz", "test.raw", nil)
	assert.NoError(t, err)
	assert.NotNil(t, obj)

	assert.True(t, ow.isFinished)
	assert.True(t, ow.isUploadStarted)
	assert.Equal(t, int64(200), obj.Size)
}

func TestOssWriter_FinishUpload_TotalUpload(t *testing.T) {
	t.Parallel()

	backend, sess, ts := initMockOssBackend(t)
	defer ts.Close()

	ow := NewOSSUploader(context.Background(), s3.New(sess), "xyz", "test.raw", 1024)
	n, err := ow.Write(make([]byte, 100))
	assert.NoError(t, err)
	assert.Equal(t, 100, n)

	assert.False(t, ow.isFinished)
	assert.NoError(t, ow.FinishUpload())
	obj, err := backend.GetObject("xyz", "test.raw", nil)
	assert.NoError(t, err)
	assert.NotNil(t, obj)

	assert.True(t, ow.isFinished)
	assert.False(t, ow.isUploadStarted)
	assert.Equal(t, int64(100), obj.Size)
}

func TestOssWriter_FinishUpload_RetryTimes(t *testing.T) {
	t.Parallel()

	backend, sess, ts := initMockOssBackend(t)
	defer ts.Close()

	ow := NewOSSUploader(context.Background(), s3.New(sess), "xyz", "test.raw", 1024)
	n, err := ow.Write(make([]byte, 100))
	assert.NoError(t, err)
	assert.Equal(t, 100, n)

	assert.False(t, ow.isFinished)
	assert.NoError(t, ow.FinishUpload())
	obj, err := backend.GetObject("xyz", "test.raw", nil)
	assert.NoError(t, err)
	assert.NotNil(t, obj)

	assert.Error(t, ow.FinishUpload())
}

func TestOssWriter_FinishUpload_WithError(t *testing.T) {
	t.Parallel()

	backend, sess, ts := initMockOssBackend(t)
	defer ts.Close()

	ow := NewOSSUploader(context.Background(), s3.New(sess), "no-exists", "test.raw", 1024)
	n, err := ow.Write(make([]byte, 100))
	assert.NoError(t, err)
	assert.Equal(t, 100, n)

	assert.False(t, ow.isFinished)
	assert.Error(t, ow.FinishUpload())
	_, err = backend.GetObject("xyz", "test.raw", nil)
	assert.Error(t, err)

	assert.Error(t, ow.uploadLastError)
}

func TestOssWriter_MultiPartUpload_WithError_1(t *testing.T) {
	t.Parallel()

	backend, sess, ts := initMockOssBackend(t)
	defer ts.Close()

	ow := NewOSSUploader(context.Background(), s3.New(sess), "no-exists", "test.raw", 128)
	n, err := ow.Write(make([]byte, 200))
	assert.NoError(t, err)
	assert.Equal(t, 200, n)

	assert.False(t, ow.isFinished)
	assert.Error(t, ow.FinishUpload())
	_, err = backend.GetObject("xyz", "test.raw", nil)
	assert.Error(t, err)

	assert.Error(t, ow.uploadLastError)
}

func TestOssWriter_MultiPartUpload_WithError_2(t *testing.T) {
	t.Parallel()

	backend, sess, ts := initMockOssBackend(t)
	defer ts.Close()

	ow := NewOSSUploader(context.Background(), s3.New(sess), "xyz", "test.raw", 128)
	n, err := ow.Write(make([]byte, 200))
	assert.NoError(t, err)
	assert.Equal(t, 200, n)

	// wait first part uploaded
	assert.NoError(t, wait.PollImmediate(10*time.Millisecond, time.Second, func() (bool, error) {
		return ow.isUploadStarted, nil
	}))
	time.Sleep(100 * time.Millisecond)

	// hack: change bucket name, force upload failed
	ow.bucket = "not-exists"
	n, err = ow.Write(make([]byte, 200))
	assert.NoError(t, err)
	assert.Equal(t, 200, n)

	assert.False(t, ow.isFinished)
	assert.Error(t, ow.FinishUpload())
	_, err = backend.GetObject("xyz", "test.raw", nil)
	assert.Error(t, err)

	assert.Error(t, ow.uploadLastError)
}

func TestOssWriter_MultiPartUpload_WithError_3(t *testing.T) {
	t.Parallel()

	backend, sess, ts := initMockOssBackend(t)
	defer ts.Close()

	ow := NewOSSUploader(context.Background(), s3.New(sess), "xyz", "test.raw", 128)
	n, err := ow.Write(make([]byte, 200))
	assert.NoError(t, err)
	assert.Equal(t, 200, n)

	// wait first part uploaded
	assert.NoError(t, wait.PollImmediate(10*time.Millisecond, time.Second, func() (bool, error) {
		return ow.isUploadStarted, nil
	}))
	time.Sleep(100 * time.Millisecond)

	// hack: change bucket name, force complete request failed
	ow.bucket = "not-exists"
	assert.False(t, ow.isFinished)
	assert.Error(t, ow.FinishUpload())
	_, err = backend.GetObject("xyz", "test.raw", nil)
	assert.Error(t, err)

	assert.Error(t, ow.uploadLastError)
}

func TestOssWriter_MultiPartUpload_NoLeftData(t *testing.T) {
	t.Parallel()

	backend, sess, ts := initMockOssBackend(t)
	defer ts.Close()

	ow := NewOSSUploader(context.Background(), s3.New(sess), "xyz", "test.raw", 128)
	n, err := ow.Write(make([]byte, 100))
	assert.NoError(t, err)
	assert.Equal(t, 100, n)

	n, err = ow.Write(make([]byte, 28))
	assert.NoError(t, err)
	assert.Equal(t, 28, n)

	assert.False(t, ow.isFinished)
	assert.True(t, ow.isUploadStarted)
	assert.Equal(t, 0, ow.buffer.Len())

	assert.NoError(t, ow.FinishUpload())
	obj, err := backend.GetObject("xyz", "test.raw", nil)
	assert.NoError(t, err)
	assert.NotNil(t, obj)

	assert.True(t, ow.isFinished)
	assert.True(t, ow.isUploadStarted)
	assert.Equal(t, int64(128), obj.Size)
}
