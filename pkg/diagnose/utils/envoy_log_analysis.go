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
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	pcommon "github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/diagnose/common"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/diagnose"
)

type LogEntry struct {
	LogDir      string
	LogPattern  string
	TargetDate  *time.Time
	SearchValue string
	RootDir     string
}

type logLineInfoEntry struct {
	RemoteAddress             string `json:"remote_address"`
	StartTime                 string `json:"start_time"`
	KusciaSource              string `json:"kuscia_source"`
	KusciaHost                string `json:"kuscia_host"`
	Method                    string `json:"method"`
	Path                      string `json:"path"`
	Protocol                  string `json:"protocol"`
	TraceID                   string `json:"trace_id"`
	SpanID                    string `json:"span_id"`
	ResponseCode              string `json:"response_code"`
	ResponseFlags             string `json:"response_flags"`
	ContentLength             string `json:"content_length"`
	UpstreamWireBytesReceived string `json:"upstream_wire_bytes_received"`
	Duration                  string `json:"duration"`
	RequestDuration           string `json:"request_duration"`
	ResponseDuration          string `json:"response_duration"`
	ResponseTxDuration        string `json:"response_tx_duration"`
	RequestBody               string `json:"request_body"`
	ResponseBody              string `json:"response_body"`
}

func ParseLogEntry(line, regexStr string) (*diagnose.EnvoyLog, error) {
	regex := regexp.MustCompile(regexStr)
	match := regex.FindStringSubmatch(line)
	if match == nil {
		return nil, fmt.Errorf("log line does not match the expected format")
	}
	names := regex.SubexpNames()
	result := make(map[string]interface{})
	for i, name := range names {
		if i != 0 && name != "" {
			result[name] = match[i]
		}
	}
	jsonData, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("error converting map to JSON: %v", err)
	}
	var logEntry logLineInfoEntry
	err = json.Unmarshal(jsonData, &logEntry)
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling JSON: %v", err)
	}
	startTime := strings.Trim(logEntry.StartTime, "[]")
	entry := &diagnose.EnvoyLog{
		Ip:            logEntry.RemoteAddress,
		Timestamp:     startTime,
		NodeName:      logEntry.KusciaSource,
		ServiceName:   logEntry.KusciaHost,
		HttpMethod:    logEntry.Method,
		InterfaceAddr: logEntry.Path,
		TraceId:       logEntry.TraceID,
		StatusCode:    logEntry.ResponseCode,
		ContentLength: logEntry.ContentLength,
		RequestTime:   logEntry.RequestDuration,
	}
	return entry, nil
}

func GetLogAnalysisResult(taskID, rootDir string, targetDate *time.Time) ([]*diagnose.EnvoyLogInfo, error) {
	var envoyLogInfos []*diagnose.EnvoyLogInfo
	// internal log
	internalLogs, err := getLogEntries(common.EnvoyLogPath, common.InternalTypeLog, taskID, common.InternalRegexStr, rootDir, targetDate)
	if err != nil {
		nlog.Errorf("Get internal log failed, err: %v", err)
		return nil, fmt.Errorf("get internal log failed, err: %v", err)
	}
	if len(internalLogs) > 0 {
		envoyLogInfo := &diagnose.EnvoyLogInfo{
			Type:         common.InternalTypeLog,
			EnvoyLogList: internalLogs,
		}
		envoyLogInfos = append(envoyLogInfos, envoyLogInfo)
	}

	// external log
	externalLogs, err := getLogEntries(common.EnvoyLogPath, common.ExternalTypeLog, taskID, common.ExternalRegexStr, rootDir, targetDate)
	if err != nil {
		nlog.Errorf("Get external log failed, err: %v", err)
		return nil, fmt.Errorf("get external log failed, err: %v", err)
	}
	if len(externalLogs) > 0 {
		envoyLogInfo := &diagnose.EnvoyLogInfo{
			Type:         common.ExternalTypeLog,
			EnvoyLogList: externalLogs,
		}
		envoyLogInfos = append(envoyLogInfos, envoyLogInfo)
	}
	return envoyLogInfos, nil
}

func getLogEntries(logDir, logPattern, searchValue, regexStr, rootDir string, targetDate *time.Time) ([]*diagnose.EnvoyLog, error) {
	entry := &LogEntry{logDir, logPattern, targetDate, searchValue, rootDir}
	logFile, err := entry.getLogFileByDate()
	if err != nil {
		return nil, fmt.Errorf("get log file fail: %v", err)
	}
	return entry.searchInLogFile(logFile, regexStr)
}

func getAllFileNames(rootDir, dirPath string) ([]string, error) {
	var fileNames []string
	if rootDir == "" {
		rootDir = pcommon.DefaultKusciaHomePath()
	}
	err := filepath.Walk(path.Join(rootDir, dirPath), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			fileNames = append(fileNames, info.Name())
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return fileNames, nil
}

func (e *LogEntry) getLogFileByDate() (string, error) {
	logFiles, err := getAllFileNames(e.RootDir, e.LogDir)
	if err != nil {
		return "", fmt.Errorf("get all file name fail: %v", err)
	}
	type LogFile struct {
		filePath  string
		timestamp time.Time
		isLatest  bool
	}

	var parsedFiles []LogFile
	// Regular expressions for log file matching
	var logFileRegex = regexp.MustCompile(fmt.Sprintf(`%s\.log(-(?P<timestamp>\d{8}-\d{2}))?(?P<compressed>\.gz)?$`, e.LogPattern))
	// Parsing the log file list
	for _, file := range logFiles {
		matches := logFileRegex.FindStringSubmatch(file)
		if matches == nil {
			continue
		}

		// Extract timestamp section
		timestamp := ""
		for i, name := range logFileRegex.SubexpNames() {
			if name == "timestamp" {
				timestamp = matches[i]
				break
			}
		}

		if timestamp == "" {
			// Files without timestamps (internal.log) as the latest file
			parsedFiles = append(parsedFiles, LogFile{
				filePath: file,
				isLatest: true,
				// Files without timestamps are marked as up-to-date
				timestamp: time.Time{},
			})
		} else {
			// Parse timestamps
			logTime, err := time.Parse(common.LogTimeFormat, timestamp)
			if err != nil {
				return "", fmt.Errorf("failed to parse timestamp %s: %w", timestamp, err)
			}
			// will convert the CST time to UTC time, so it needs to be subtracted by 8 hours
			parsedFiles = append(parsedFiles, LogFile{
				filePath:  file,
				timestamp: CSTTimeCovertToUTC(logTime),
				isLatest:  false,
			})
		}
	}

	// chronological
	sort.Slice(parsedFiles, func(i, j int) bool {
		// The newest files are always last
		if parsedFiles[i].isLatest {
			return false
		}
		if parsedFiles[j].isLatest {
			return true
		}
		return parsedFiles[i].timestamp.Before(parsedFiles[j].timestamp)
	})
	// Comparison time
	for _, file := range parsedFiles {
		if file.isLatest {
			// If you compare to the most recent file (internal.log), you can directly return the
			return file.filePath, nil
		}

		// Compare timestamps
		if e.TargetDate.Unix() <= file.timestamp.Unix() {
			return file.filePath, nil
		}
	}

	// Defaults to returning the latest file (if not explicitly found)
	return fmt.Sprintf("%s.log", e.LogPattern), nil
}

func (e *LogEntry) searchInLogFile(logFile, regexStr string) ([]*diagnose.EnvoyLog, error) {
	var result []*diagnose.EnvoyLog
	if e.RootDir == "" {
		e.RootDir = pcommon.DefaultKusciaHomePath()
	}
	file, err := os.Open(path.Join(e.RootDir, e.LogDir, logFile))
	if err != nil {
		return nil, fmt.Errorf("open log file fail: %v", err)
	}
	defer file.Close()

	var scanner *bufio.Scanner
	if strings.HasSuffix(logFile, ".gz") {
		// If the file is compressed, use gzip to decompress it.
		var gzipReader *gzip.Reader
		gzipReader, err = gzip.NewReader(file)
		if err != nil {
			return nil, fmt.Errorf("read gzip log file fail: %v", err)
		}
		defer gzipReader.Close()
		scanner = bufio.NewScanner(gzipReader)
	} else {
		// Uncompressed file, direct read
		scanner = bufio.NewScanner(file)
	}

	// Reads line by line and determines if it contains the target string
	for scanner.Scan() {
		line := scanner.Text()
		entry, parseErr := ParseLogEntry(line, regexStr)
		if parseErr != nil {
			nlog.Errorf("Skipping invalid log line: %v", parseErr)
			continue
		}
		if strings.HasPrefix(entry.ServiceName, e.SearchValue) {
			if entry.StatusCode != fmt.Sprintf("%d", http.StatusOK) {
				result = append(result, entry)
			}
		}
	}

	if err = scanner.Err(); err != nil {
		return nil, fmt.Errorf("read log file error: %v", err)
	}

	return result, nil
}

func CSTTimeCovertToUTC(CSTTime time.Time) time.Time {
	return CSTTime.Add(-8 * time.Hour)
}
