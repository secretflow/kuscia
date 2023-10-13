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

package nlog

type LogConfig struct {
	LogLevel string
	LogPath  string

	// MaxFileSizeMB is the maximum size in megabytes of the log file before it gets
	// rotated. It defaults to 100 megabytes.
	MaxFileSizeMB int

	// MaxFiles is the maximum number of old log files to retain.  The default
	// is to retain all old log files
	MaxFiles int

	Compress bool
}
