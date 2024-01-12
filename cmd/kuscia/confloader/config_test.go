package confloader

import (
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/secretflow/kuscia/pkg/web/asserts"
)

const overwriteRootDir = "/home/test-kuscia"
const overwriteMasterConfig = "rootDir: " + overwriteRootDir

func Test_defaultMasterOverwrite(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		wantRootDir string
	}{
		{
			name:        "default kuscia config overwrite",
			content:     overwriteMasterConfig,
			wantRootDir: overwriteRootDir,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := defaultMaster("/home/kuscia")
			err := yaml.Unmarshal([]byte(tt.content), &got)
			_ = asserts.IsNil(err, "unmarshal yaml should success")
			if got.RootDir != tt.wantRootDir {
				t.Errorf("rootDir should be overwrite")
			}
		})
	}
}
