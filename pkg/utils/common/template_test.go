package common

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	rbacv1 "k8s.io/api/rbac/v1"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

func TestRanderObject(t *testing.T) {
	workDir := GetWorkDir()
	tmpPath := filepath.Join(workDir, "etc/conf/domain-namespace-res.yaml")
	role := &rbacv1.Role{}
	input := struct {
		DomainID string
	}{
		DomainID: "alice",
	}
	err := RenderRuntimeObject(tmpPath, role, input)
	assert.NilError(t, err)
}

func GetWorkDir() string {
	_, filename, _, _ := runtime.Caller(0)
	nlog.Infof("path is %s", filename)
	return strings.SplitN(filename, "pkg", 2)[0]
}
