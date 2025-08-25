// Copyright 2025 Ant Group Co., Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package utils

import (
	"compress/gzip"
	"io"
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/secretflow/kuscia/pkg/diagnose/common"
)

func TestParseLogEntry(t *testing.T) {
	// test parse valid log line
	t.Run("test parse valid log line", func(t *testing.T) {
		line := `172.18.0.3 - [30/May/2025:06:36:44 +0000] bob secretflow-task-20250530143614-single-psi-0-spu.alice.svc "POST /org.interconnection.link.ReceiverService/Push HTTP/1.1" bd3f1ba95ebb6375 bd3f1ba95ebb6375 200 - 118783 33 202 - -`
		regexStr := `^(?P<remote_address>[\d\.]+) - (?P<start_time>\[\d{2}/[A-Za-z]{3}/\d{4}:\d{2}:\d{2}:\d{2} [+-]\d{4}\]) (?P<kuscia_source>[^\s]*) (?P<kuscia_host>[^\s]*) "(?P<method>[A-Z]+) (?P<path>[^\s]+) (?P<protocol>[A-Z]+/[\d\.]+)" (?P<trace_id>[^\s]*) (?P<span_id>[^\s]*) (?P<response_code>\d{3}) (?P<response_flags>[^\s]*) (?P<content_length>[^\s]*) (?P<upstream_wire_bytes_received>[^\s]*) (?P<duration>[^\s]*) (?P<request_duration>[^\s]*) (?P<response_duration>[^\s]*)$`

		entry, err := ParseLogEntry(line, regexStr)
		assert.NoError(t, err)
		assert.Equal(t, "172.18.0.3", entry.Ip)
		assert.Equal(t, "30/May/2025:06:36:44 +0000", entry.Timestamp)
		assert.Equal(t, "bob", entry.NodeName)
		assert.Equal(t, "secretflow-task-20250530143614-single-psi-0-spu.alice.svc", entry.ServiceName)
		assert.Equal(t, "POST", entry.HttpMethod)
		assert.Equal(t, "/org.interconnection.link.ReceiverService/Push", entry.InterfaceAddr)
		assert.Equal(t, "bd3f1ba95ebb6375", entry.TraceId)
		assert.Equal(t, "200", entry.StatusCode)
		assert.Equal(t, "118783", entry.ContentLength)
	})

	// test parse invalid log line
	t.Run("test parse invalid log line", func(t *testing.T) {
		line := "invalid log line"
		regexStr := `invalid pattern`

		_, err := ParseLogEntry(line, regexStr)
		assert.Error(t, err)
	})
}

func TestGetLogAnalysisResult(t *testing.T) {
	// test get log analysis result
	t.Run("test get log analysis result", func(t *testing.T) {
		// mkdir home dir
		// touch internal.log external.log
		tempDir := t.TempDir()
		os.Setenv("KUSCIA_HOME", tempDir)
		defer os.Unsetenv("KUSCIA_HOME")

		// mkdir envoy log dir
		logDir := filepath.Join(tempDir, "var/logs/envoy")
		err := os.MkdirAll(logDir, 0755)
		assert.NoError(t, err)

		// mkdir internal log dir
		internalLog := filepath.Join(logDir, "internal.log")
		err = os.WriteFile(internalLog, []byte(`10.88.0.3 - [30/May/2025:06:36:41 +0000] alice secretflow-task-20250530143614-single-psi-0-spu.bob.svc "POST /org.interconnection.link.ReceiverService/Push HTTP/1.1" 2c71159bb887fa32 2c71159bb887fa32 503 - 1424 283 206 0 105 101 - -`), 0644)
		assert.NoError(t, err)

		// mkdir internal log  dir
		internalLogGz := filepath.Join(logDir, "internal.log-20230101-12")
		err = os.WriteFile(internalLogGz, []byte(`10.88.0.3 - [30/May/2025:06:36:41 +0000] alice secretflow-task-20250530143614-single-psi-0-spu.bob.svc "POST /org.interconnection.link.ReceiverService/Push HTTP/1.1" 2c71159bb887fa32 2c71159bb887fa32 504 - 1424 283 206 0 105 101 - -`), 0644)
		assert.NoError(t, err)

		// mkdir external log dir
		externalLog := filepath.Join(logDir, "external.log")
		err = os.WriteFile(externalLog, []byte(`172.18.0.3 - [30/May/2025:06:36:44 +0000] bob secretflow-task-20250530143614-single-psi-0-spu.alice.svc "POST /org.interconnection.link.ReceiverService/Push HTTP/1.1" bd3f1ba95ebb6375 bd3f1ba95ebb6375 505 - 118783 33 202 - -`), 0644)
		assert.NoError(t, err)

		// mkdir external log  dir
		externalLogGz := filepath.Join(logDir, "external.log-20230101-12")
		err = os.WriteFile(externalLogGz, []byte(`172.18.0.3 - [30/May/2025:06:36:44 +0000] bob secretflow-task-20250530143614-single-psi-0-spu.alice.svc "POST /org.interconnection.link.ReceiverService/Push HTTP/1.1" bd3f1ba95ebb6375 bd3f1ba95ebb6375 506 - 118783 33 202 - -`), 0644)
		assert.NoError(t, err)

		// test get log analysis result
		targetDate := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
		result, err := GetLogAnalysisResult("secretflow-task-20250530143614-single-psi", tempDir, &targetDate)
		assert.NoError(t, err)
		assert.Len(t, result, 2)
		assert.Equal(t, "internal", result[0].Type)
		assert.Len(t, result[0].EnvoyLogList, 1)
		assert.Equal(t, "504", result[0].EnvoyLogList[0].StatusCode)
	})

	// test get .gz log analysis result
	t.Run("test get .gz log analysis result", func(t *testing.T) {
		// mkdir home dir
		// touch internal.log external.log
		tempDir := t.TempDir()
		os.Setenv("KUSCIA_HOME", tempDir)
		defer os.Unsetenv("KUSCIA_HOME")

		// mkdir envoy log dir
		logDir := filepath.Join(tempDir, "var/logs/envoy")
		err := os.MkdirAll(logDir, 0755)
		assert.NoError(t, err)

		// mkdir internal log dir
		internalLog := filepath.Join(logDir, "internal.log")
		err = os.WriteFile(internalLog, []byte(`10.88.0.3 - [30/May/2025:06:36:41 +0000] alice secretflow-task-20250530143614-single-psi-0-spu.bob.svc "POST /org.interconnection.link.ReceiverService/Push HTTP/1.1" 2c71159bb887fa32 2c71159bb887fa32 503 - 1424 283 206 0 105 101 - -`), 0644)
		assert.NoError(t, err)

		// mkdir internal log  dir
		internalLogDate := filepath.Join(logDir, "internal.log-20230101-12")
		err = os.WriteFile(internalLogDate, []byte(`10.88.0.3 - [30/May/2025:06:36:41 +0000] alice secretflow-task-20250530143614-single-psi-0-spu.bob.svc "POST /org.interconnection.link.ReceiverService/Push HTTP/1.1" 2c71159bb887fa32 2c71159bb887fa32 504 - 1424 283 206 0 105 101 - -`), 0644)
		assert.NoError(t, err)

		// mkdir internal .gz log  dir
		internalLogGz := filepath.Join(logDir, "internal.log-20230103-12")
		err = os.WriteFile(internalLogGz, []byte(`10.88.0.3 - [30/May/2025:06:36:41 +0000] alice secretflow-task-20250530143614-single-psi-0-spu.bob.svc "POST /org.interconnection.link.ReceiverService/Push HTTP/1.1" 2c71159bb887fa32 2c71159bb887fa32 507 - 1424 283 206 0 105 101 - -`), 0644)
		assert.NoError(t, err)

		err = compressLogFile(internalLogGz, path.Join(logDir, "internal.log-20230103-12.gz"))
		assert.NoError(t, err)

		err = os.Remove(internalLogGz)
		assert.NoError(t, err)

		// mkdir external log dir
		externalLog := filepath.Join(logDir, "external.log")
		err = os.WriteFile(externalLog, []byte(`172.18.0.3 - [30/May/2025:06:36:44 +0000] bob secretflow-task-20250530143614-single-psi-0-spu.alice.svc "POST /org.interconnection.link.ReceiverService/Push HTTP/1.1" bd3f1ba95ebb6375 bd3f1ba95ebb6375 505 - 118783 33 202 - -`), 0644)
		assert.NoError(t, err)

		// mkdir external log  dir
		externalLogDate := filepath.Join(logDir, "external.log-20230101-12")
		err = os.WriteFile(externalLogDate, []byte(`172.18.0.3 - [30/May/2025:06:36:44 +0000] bob secretflow-task-20250530143614-single-psi-0-spu.alice.svc "POST /org.interconnection.link.ReceiverService/Push HTTP/1.1" bd3f1ba95ebb6375 bd3f1ba95ebb6375 506 - 118783 33 202 - -`), 0644)
		assert.NoError(t, err)

		// mkdir external .gz log  dir
		externalLogGz := filepath.Join(logDir, "external.log-20230103-12")
		err = os.WriteFile(externalLogGz, []byte(`172.18.0.3 - [30/May/2025:06:36:44 +0000] bob secretflow-task-20250530143614-single-psi-0-spu.alice.svc "POST /org.interconnection.link.ReceiverService/Push HTTP/1.1" bd3f1ba95ebb6375 bd3f1ba95ebb6375 508 - 118783 33 202 - -`), 0644)
		assert.NoError(t, err)

		err = compressLogFile(externalLogGz, path.Join(logDir, "external.log-20230103-12.gz"))
		assert.NoError(t, err)
		err = os.Remove(externalLogGz)
		assert.NoError(t, err)

		// test get log analysis result
		targetDate := time.Date(2023, 1, 2, 0, 0, 0, 0, time.UTC)
		result, err := GetLogAnalysisResult("secretflow-task-20250530143614-single-psi", tempDir, &targetDate)
		assert.NoError(t, err)
		assert.Len(t, result, 2)
		assert.Equal(t, "internal", result[0].Type)
		assert.Len(t, result[0].EnvoyLogList, 1)
		assert.Equal(t, "507", result[0].EnvoyLogList[0].StatusCode)
	})
}

func compressLogFile(inputFileName, outputFileName string) error {
	// open input file
	inputFile, err := os.Open(inputFileName)
	if err != nil {
		return err
	}
	defer inputFile.Close()

	// create output file
	outputFile, err := os.Create(outputFileName)
	if err != nil {
		return err
	}
	defer outputFile.Close()

	// create gzip.Writer
	gzipWriter := gzip.NewWriter(outputFile)
	defer gzipWriter.Close()

	// write input file to gzip.Writer
	_, err = io.Copy(gzipWriter, inputFile)
	return err
}

func TestLogEntry_getLogFileByDate(t *testing.T) {
	// test get log file by date
	t.Run("test get log file by date", func(t *testing.T) {
		// mkdir home dir
		tempDir := t.TempDir()
		os.Setenv("KUSCIA_HOME", tempDir)
		defer os.Unsetenv("KUSCIA_HOME")

		// mkdir envoy log dir
		logDir := filepath.Join(tempDir, "var/logs/envoy")
		err := os.MkdirAll(logDir, 0755)
		assert.NoError(t, err)

		// mkdir internal log dir
		files := []string{
			"internal.log",
			"internal.log-20230101-00.gz",
			"internal.log-20230102-00.gz",
			"internal.log-20230103-00.gz",
		}
		for _, file := range files {
			err = os.WriteFile(filepath.Join(logDir, file), []byte{}, 0644)
			assert.NoError(t, err)
		}

		// test get log file by date
		targetDate := time.Date(2023, 1, 2, 0, 0, 0, 0, time.UTC)
		entry := &LogEntry{
			LogDir:     "var/logs/envoy",
			LogPattern: "internal",
			TargetDate: &targetDate,
			RootDir:    tempDir,
		}

		file, err := entry.getLogFileByDate()
		assert.NoError(t, err)
		assert.Equal(t, "internal.log-20230103-00.gz", file)
	})

	// test get latest log file
	t.Run("test get latest log file", func(t *testing.T) {
		// mkdir home dir
		tempDir := t.TempDir()
		os.Setenv("KUSCIA_HOME", tempDir)
		defer os.Unsetenv("KUSCIA_HOME")

		// mkdir envoy log dir
		logDir := filepath.Join(tempDir, "var/logs/envoy")
		err := os.MkdirAll(logDir, 0755)
		assert.NoError(t, err)

		// mkdir internal log dir
		err = os.WriteFile(filepath.Join(logDir, "internal.log"), []byte{}, 0644)
		assert.NoError(t, err)

		// test get latest log file
		entry := &LogEntry{
			LogDir:     "var/logs/envoy",
			LogPattern: "internal",
			RootDir:    tempDir,
		}

		file, err := entry.getLogFileByDate()
		assert.NoError(t, err)
		assert.Equal(t, "internal.log", file)
	})
}

func TestLogEntry_searchInLogFile(t *testing.T) {
	// test search in log file
	t.Run("test search in log file", func(t *testing.T) {
		// mkdir home dir
		tempDir := t.TempDir()
		os.Setenv("KUSCIA_HOME", tempDir)
		defer os.Unsetenv("KUSCIA_HOME")

		// test search in log file
		logDir := filepath.Join(tempDir, "var/logs/envoy")
		err := os.MkdirAll(logDir, 0755)
		assert.NoError(t, err)

		// mkdir internal log dir
		logContent := `10.88.0.3 - [30/May/2025:06:36:41 +0000] alice secretflow-task-20250530143614-single-psi-0-spu.bob.svc "POST /org.interconnection.link.ReceiverService/Push HTTP/1.1" 2c71159bb887fa32 2c71159bb887fa32 503 - 1424 283 206 0 105 101 - -
10.88.0.3 - [30/May/2025:06:36:41 +0000] alice secretflow-task-20250530143614-single-psi-0-spu.bob.svc "POST /org.interconnection.link.ReceiverService/Push HTTP/1.1" 2c71159bb887fa32 2c71159bb887fa32 200 - 1424 283 206 0 105 101 - -`

		// mkdir internal log dir
		logFile := filepath.Join(logDir, "internal.log")
		err = os.WriteFile(logFile, []byte(logContent), 0644)
		assert.NoError(t, err)

		// test search in log file
		regexStr := common.InternalRegexStr
		entry := &LogEntry{
			LogDir:      "var/logs/envoy",
			LogPattern:  "internal",
			SearchValue: "secretflow-task-20250530143614-single-psi",
			RootDir:     tempDir,
		}

		result, err := entry.searchInLogFile("internal.log", regexStr)
		assert.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Equal(t, "503", result[0].StatusCode)
	})

}

func TestGetAllFileNames(t *testing.T) {
	// test get all file names
	t.Run("test get all file names", func(t *testing.T) {
		// mkdir home dir
		tempDir := t.TempDir()
		os.Setenv("KUSCIA_HOME", tempDir)
		defer os.Unsetenv("KUSCIA_HOME")

		// mkdir test dir
		testDir := filepath.Join(tempDir, "testdir")
		err := os.MkdirAll(testDir, 0755)
		assert.NoError(t, err)

		files := []string{"file1.txt", "file2.txt", "file3.txt"}
		for _, file := range files {
			err = os.WriteFile(filepath.Join(testDir, file), []byte{}, 0644)
			assert.NoError(t, err)
		}

		// test get all file names
		result, err := getAllFileNames(tempDir, "testdir")
		assert.NoError(t, err)
		assert.Len(t, result, 3)
		assert.Contains(t, result, "file1.txt")
		assert.Contains(t, result, "file2.txt")
		assert.Contains(t, result, "file3.txt")
	})
}
