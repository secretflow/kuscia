package kusciainit

import (
	"path"
	"reflect"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/secretflow/kuscia/cmd/kuscia/confloader"
	"github.com/secretflow/kuscia/pkg/utils/paths"
	"github.com/secretflow/kuscia/pkg/utils/tls"
)

func TestKusciaInitCommand(t *testing.T) {
	domainKeyFile := path.Join(t.TempDir(), "domain.key")
	assert.NilError(t, tls.GeneratePrivateKeyToFile(domainKeyFile))
	domainKeyData, err := loadDomainKeyData(domainKeyFile)
	assert.NilError(t, err)
	testCases := []struct {
		name         string
		initConfig   InitConfig
		kusciaConfig interface{}
	}{
		{
			name: "Master kuscia config",
			initConfig: InitConfig{
				Mode:                  "Master",
				DomainID:              "kuscia-system",
				DomainKeyFile:         domainKeyFile,
				LogLevel:              "INFO",
				Protocol:              "NOTLS",
				EnableWorkloadApprove: true,
			},
			kusciaConfig: confloader.MasterKusciaConfig{
				CommonConfig: confloader.CommonConfig{
					Mode:          "Master",
					DomainID:      "kuscia-system",
					DomainKeyData: domainKeyData,
					LogLevel:      "INFO",
					Protocol:      "NOTLS",
				},
				AdvancedConfig: confloader.AdvancedConfig{
					EnableWorkloadApprove: true,
				},
			},
		},
		{
			name: "Lite kuscia config",
			initConfig: InitConfig{
				Mode:                  "Lite",
				DomainID:              "alice",
				LogLevel:              "INFO",
				DomainKeyFile:         domainKeyFile,
				LiteDeployToken:       "test",
				MasterEndpoint:        "https://master.svc:1080",
				Runtime:               "runc",
				Protocol:              "NOTLS",
				EnableWorkloadApprove: false,
			},
			kusciaConfig: confloader.LiteKusciaConfig{
				CommonConfig: confloader.CommonConfig{
					Mode:          "Lite",
					DomainID:      "alice",
					DomainKeyData: domainKeyData,
					LogLevel:      "INFO",
					Protocol:      "NOTLS",
				},
				LiteDeployToken: "test",
				MasterEndpoint:  "https://master.svc:1080",
				Runtime:         "runc",
				AdvancedConfig: confloader.AdvancedConfig{
					EnableWorkloadApprove: false,
				},
			},
		},
		{
			name: "Autonomy kuscia config",
			initConfig: InitConfig{
				Mode:          "Autonomy",
				DomainID:      "alice",
				LogLevel:      "INFO",
				DomainKeyFile: domainKeyFile,
				Runtime:       "runc",
				Protocol:      "NOTLS",
			},
			kusciaConfig: confloader.AutomonyKusciaConfig{
				CommonConfig: confloader.CommonConfig{
					Mode:          "Autonomy",
					DomainID:      "alice",
					DomainKeyData: domainKeyData,
					LogLevel:      "INFO",
					Protocol:      "NOTLS",
				},
				Runtime: "runc",
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			kusciaConfig := testCase.initConfig.convert2KusciaConfig()
			assert.Equal(t, reflect.DeepEqual(kusciaConfig, testCase.kusciaConfig), true)
		})
	}
}

func TestLoadDomainKeyData(t *testing.T) {
	validkeyFile := path.Join(t.TempDir(), "domain.key")
	assert.NilError(t, tls.GeneratePrivateKeyToFile(validkeyFile))
	invalidKeyFile := path.Join(t.TempDir(), "invalid-domain.key")
	assert.NilError(t, paths.WriteFile(invalidKeyFile, []byte("test")))

	testCases := []struct {
		name          string
		domainKeyFile string
		wantErr       bool
	}{
		{
			name:          "valid domain key file exists",
			domainKeyFile: validkeyFile,
			wantErr:       false,
		},
		{
			name:          "domain key file not exists",
			domainKeyFile: path.Join(t.TempDir(), "empty.key"),
			wantErr:       true,
		},
		{
			name:          "invalid domain key file exists",
			domainKeyFile: invalidKeyFile,
			wantErr:       true,
		},
		{
			name:          "domain key file is empty",
			domainKeyFile: "",
			wantErr:       false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := loadDomainKeyData(testCase.domainKeyFile)
			if !testCase.wantErr {
				assert.NilError(t, err)
			} else {
				assert.Error(t, err, err.Error())
			}
		})
	}

}
