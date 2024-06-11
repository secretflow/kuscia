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
	"bytes"
	"context"
	"errors"
	"fmt"
	"path"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

type OSSUploader struct {
	loggerKey string
	buffer    *bytes.Buffer

	partSizeBytes int

	client    *s3.S3
	bucket    string
	objectKey string

	mutex           sync.Mutex
	isFinished      bool
	uploadLastError error
	isUploadStarted bool
	wg              sync.WaitGroup

	multiPartUploadChan chan []byte
	uploadContext       context.Context
	uploadCancel        context.CancelFunc
}

func NewOSSUploader(ctx context.Context, client *s3.S3, bucket, objectKey string, partSizeBytes int) *OSSUploader {
	initBuf := make([]byte, partSizeBytes)
	ctx, cancel := context.WithCancel(ctx)
	return &OSSUploader{
		client:              client,
		partSizeBytes:       partSizeBytes,
		buffer:              bytes.NewBuffer(initBuf[:0]),
		isFinished:          false,
		bucket:              bucket,
		objectKey:           objectKey,
		multiPartUploadChan: make(chan []byte, 3),
		uploadContext:       ctx,
		uploadCancel:        cancel,
		loggerKey:           path.Join(bucket, objectKey),
	}
}

func (ow *OSSUploader) Write(p []byte) (int, error) {
	if ow.buffer.Len()+len(p) >= ow.partSizeBytes {
		// buffer is full, upload and clean buffer
		if ow.buffer.Len() == 0 {
			// current buffer is not used, so just upload the body
			ow.appendBuffer(p)
		} else {
			if _, err := ow.buffer.Write(p); err != nil {
				nlog.Warnf("Write to buffer failed with %s", err.Error())
				return 0, err
			}

			ow.appendBuffer(ow.buffer.Bytes())
			ow.buffer.Reset()
		}

		return len(p), nil
	}

	// buffer is not full, so cache it
	return ow.buffer.Write(p)
}

func (ow *OSSUploader) appendBuffer(p []byte) {
	buf := make([]byte, len(p))
	copy(buf, p)
	ow.multiPartUploadChan <- buf

	ow.mutex.Lock()
	defer ow.mutex.Unlock()

	if !ow.isUploadStarted {
		ow.isUploadStarted = true
		ow.wg.Add(1)
		go func() {
			defer ow.wg.Done()
			err := ow.multiPartUpload()

			if err != nil {
				ow.mutex.Lock()
				defer ow.mutex.Unlock()
				ow.uploadLastError = err
			}
		}()
	}
}

func (ow *OSSUploader) multiPartUpload() error {
	result, err := ow.client.CreateMultipartUpload(&s3.CreateMultipartUploadInput{
		Bucket: aws.String(ow.bucket),
		Key:    aws.String(ow.objectKey),
	})
	if err != nil {
		nlog.Warnf("[%s] CreateMultipartUpload failed with: %s", ow.loggerKey, err.Error())
		return err
	}

	nlog.Infof("[%s] Multipart UploadID=%s", ow.loggerKey, *result.UploadId)

	uploadID := result.UploadId
	nextPartNumber := int64(1)

	var parts []*s3.CompletedPart
loopForPart:
	for {
		select {
		case buf := <-ow.multiPartUploadChan:
			part, err := ow.uploadToOSS(buf, uploadID, nextPartNumber)
			if err != nil {
				return err
			}
			nextPartNumber++
			parts = append(parts, part)
		case <-ow.uploadContext.Done():
			nlog.Infof("[%s] Got finished signal", ow.loggerKey)
			break loopForPart
		}
	}

loopLeftData:
	for { // upload left data
		select {
		case buf := <-ow.multiPartUploadChan:
			part, err := ow.uploadToOSS(buf, uploadID, nextPartNumber)
			if err != nil {
				return err
			}
			nextPartNumber++
			parts = append(parts, part)
		default:
			break loopLeftData
		}

	}

	nlog.Infof("[%s] Try to complete upload", ow.loggerKey)

	if _, err := ow.client.CompleteMultipartUpload(&s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(ow.bucket),
		Key:      aws.String(ow.objectKey),
		UploadId: uploadID,
		MultipartUpload: &s3.CompletedMultipartUpload{
			Parts: parts,
		},
	}); err != nil {
		nlog.Warnf("[%s] Failed to complete Multipart Upload: %s", ow.loggerKey, err.Error())
		return err
	}

	return nil
}

func (ow *OSSUploader) uploadToOSS(p []byte, uploadID *string, partNumber int64) (*s3.CompletedPart, error) {
	nlog.Infof("[%s] Try to upload part %d, len=%d", ow.loggerKey, partNumber, len(p))
	result, err := ow.client.UploadPart(&s3.UploadPartInput{
		Bucket:     aws.String(ow.bucket),
		Key:        aws.String(ow.objectKey),
		UploadId:   uploadID,
		PartNumber: aws.Int64(partNumber),
		Body:       bytes.NewReader(p),
	})
	if err != nil {
		nlog.Warnf("[%s] Failed to upload part(%s:%d): %s", ow.loggerKey, *uploadID, partNumber, err)

		return nil, err
	}

	return &s3.CompletedPart{
		ETag:       result.ETag,
		PartNumber: aws.Int64(int64(partNumber)),
	}, nil
}

func (ow *OSSUploader) hadFinished() error {
	ow.mutex.Lock()
	defer ow.mutex.Unlock()
	if ow.isFinished {
		return errors.New("had called FinishUpload")
	}
	ow.isFinished = true

	return nil
}

func (ow *OSSUploader) hadUploadError() error {
	ow.mutex.Lock()
	defer ow.mutex.Unlock()

	if ow.uploadLastError != nil {
		return fmt.Errorf("some upload failed with %s", ow.uploadLastError.Error())
	}
	return nil
}

func (ow *OSSUploader) isNeedUploadLeftBuffer() (bool, error) {
	ow.mutex.Lock()
	defer ow.mutex.Unlock()

	if ow.buffer.Len() != 0 {
		if ow.isUploadStarted {
			return true, nil
		}

		nlog.Infof("[%s] Buffer is small than %d bytes, so just upload all", ow.loggerKey, ow.partSizeBytes)
		// try to upload total buffer without multi-party
		reader := bytes.NewReader(ow.buffer.Bytes())
		_, err := ow.client.PutObject(&s3.PutObjectInput{
			Bucket: aws.String(ow.bucket),
			Key:    aws.String(ow.objectKey),
			Body:   reader,
		})

		if err != nil {
			nlog.Warnf("[%s] PutObject failed with %s", ow.loggerKey, err.Error())
			ow.uploadLastError = err
		}

		return false, err

	}

	return false, nil
}

func (ow *OSSUploader) FinishUpload() error {
	if err := ow.hadFinished(); err != nil {
		return err
	}

	started, err := ow.isNeedUploadLeftBuffer()
	if err != nil {
		return err
	}

	if started {
		ow.appendBuffer(ow.buffer.Bytes())
	}

	nlog.Infof("[%s] Signal to finish upload", ow.loggerKey)
	// wait for all upload finished
	ow.uploadCancel()
	ow.wg.Wait()

	if err := ow.hadUploadError(); err != nil {
		nlog.Warnf("[%s] Upload failed with some error, so don't call complete upload", ow.loggerKey)
		return err
	}

	nlog.Infof("[%s] Finish upload", ow.loggerKey)

	return nil
}

func (ow *OSSUploader) Close() error {
	go func() {
		ow.FinishUpload()
	}()

	return nil
}
