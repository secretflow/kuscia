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

package metrics

import (
	"fmt"
	"os"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/secretflow/kuscia/pkg/web/logs"
)

const (
	defaultProbePath   string = "/probe"
	defaultVersionPath string = "/version"
)

// NewProbeInfo generates a new set of ProbeInfo with a certain subsystem name
func NewProbeInfo(servicename string, version string) *ProbeInfo {
	hostname, err := os.Hostname()
	if err != nil {
		logs.GetLogger().Errorf("Get hostname err: %v", err)
	}

	return &ProbeInfo{
		ServiceName: servicename,
		Host:        hostname,
		Version:     version,
		ProbePath:   defaultProbePath,
		VersionPath: defaultVersionPath,
		Uptime:      time.Now(),
	}
}

// ProbeInfo store the info exported in probe url
type ProbeInfo struct {
	ServiceName string
	Host        string
	Version     string
	ProbePath   string
	VersionPath string
	Uptime      time.Time
}

// Use adds the middleware to a gin engine.
func (p *ProbeInfo) Use(e *gin.Engine) {
	p.RegistPath(e)
}

// RegistPath regist Probe/Version paths to gin
func (p *ProbeInfo) RegistPath(e *gin.Engine) {
	e.GET(p.ProbePath, p.probeHandler)
	e.GET(p.VersionPath, p.versionHandler)
}

// SetProbePath set Probe path
func (p *ProbeInfo) SetProbePath(path string) {
	if path != "" {
		p.ProbePath = path
	}
}

// SetVersionPath set Probe path
func (p *ProbeInfo) SetVersionPath(path string) {
	if path != "" {
		p.VersionPath = path
	}
}

func (p *ProbeInfo) probeHandler(c *gin.Context) {
	c.String(200, fmt.Sprintf("ServiceName: %s, Host: %s, Version: %s, Uptime: %v", p.ServiceName, p.Host, p.Version, p.Uptime))
}

func (p *ProbeInfo) versionHandler(c *gin.Context) {
	c.String(200, p.Version)
}
