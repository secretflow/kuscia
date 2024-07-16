package kusciainit

import (
	"path"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/secretflow/kuscia/cmd/kuscia/confloader"
	"github.com/secretflow/kuscia/pkg/utils/paths"
	"github.com/secretflow/kuscia/pkg/utils/tls"
)

func TestKusciaInitCommand_ConfigConvert_Master(t *testing.T) {
	t.Parallel()
	domainKeyFile := path.Join(t.TempDir(), "domain.key")
	assert.Nil(t, tls.GeneratePrivateKeyToFile(domainKeyFile))
	domainKeyData, err := loadDomainKeyData(domainKeyFile)
	assert.Nil(t, err)

	input := InitConfig{
		Mode:                  "Master",
		DomainID:              "kuscia-system",
		DomainKeyFile:         domainKeyFile,
		LogLevel:              "INFO",
		Protocol:              "NOTLS",
		EnableWorkloadApprove: true,
	}

	dst := confloader.MasterKusciaConfig{
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
	}

	var exceptValue interface{}
	exceptValue = dst

	assert.EqualValues(t, exceptValue, input.convert2KusciaConfig())
}

func TestKusciaInitCommand_ConfigConvert_Lite(t *testing.T) {
	t.Parallel()
	domainKeyFile := path.Join(t.TempDir(), "domain.key")
	assert.Nil(t, tls.GeneratePrivateKeyToFile(domainKeyFile))
	domainKeyData, err := loadDomainKeyData(domainKeyFile)
	assert.Nil(t, err)

	input := InitConfig{
		Mode:                  "Lite",
		DomainID:              "alice",
		LogLevel:              "INFO",
		DomainKeyFile:         domainKeyFile,
		LiteDeployToken:       "test",
		MasterEndpoint:        "https://master.svc:1080",
		Runtime:               "runc",
		Protocol:              "NOTLS",
		EnableWorkloadApprove: false,
	}

	dst := confloader.LiteKusciaConfig{
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
	}

	var exceptValue interface{}
	exceptValue = dst

	assert.EqualValues(t, exceptValue, input.convert2KusciaConfig())
}

func TestKusciaInitCommand_ConfigConvert_Autonomy(t *testing.T) {
	t.Parallel()
	domainKeyFile := path.Join(t.TempDir(), "domain.key")
	assert.Nil(t, tls.GeneratePrivateKeyToFile(domainKeyFile))
	domainKeyData, err := loadDomainKeyData(domainKeyFile)
	assert.Nil(t, err)

	input := InitConfig{
		Mode:          "Autonomy",
		DomainID:      "alice",
		LogLevel:      "INFO",
		DomainKeyFile: domainKeyFile,
		Runtime:       "runc",
		Protocol:      "NOTLS",
	}

	dst := confloader.AutomonyKusciaConfig{
		CommonConfig: confloader.CommonConfig{
			Mode:          "Autonomy",
			DomainID:      "alice",
			DomainKeyData: domainKeyData,
			LogLevel:      "INFO",
			Protocol:      "NOTLS",
		},
		Runtime: "runc",
	}

	var exceptValue interface{}
	exceptValue = dst

	assert.EqualValues(t, exceptValue, input.convert2KusciaConfig())
}

func TestLoadDomainKeyData_FileExists(t *testing.T) {
	t.Parallel()
	validKeyFile := path.Join(t.TempDir(), "domain.key")
	assert.Nil(t, tls.GeneratePrivateKeyToFile(validKeyFile))

	_, err := loadDomainKeyData(validKeyFile)
	assert.Nil(t, err)
}

func TestLoadDomainKeyData_FileNotExists(t *testing.T) {
	t.Parallel()

	_, err := loadDomainKeyData(path.Join(t.TempDir(), "empty.key"))
	assert.NotNil(t, err)
}

func TestLoadDomainKeyData_InvalidateFileExists(t *testing.T) {
	t.Parallel()
	invalidKeyFile := path.Join(t.TempDir(), "invalid-domain.key")
	assert.Nil(t, paths.WriteFile(invalidKeyFile, []byte("test")))

	_, err := loadDomainKeyData(invalidKeyFile)
	assert.NotNil(t, err)
}

func TestLoadDomainKeyData_EmptyFile(t *testing.T) {
	t.Parallel()

	_, err := loadDomainKeyData("")
	assert.Nil(t, err)
}
