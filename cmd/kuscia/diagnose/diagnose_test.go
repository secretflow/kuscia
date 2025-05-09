package diagnose

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewDiagnoseCommand(t *testing.T) {
	ctx := context.Background()
	cmd := NewDiagnoseCommand(ctx)

	assert.Equal(t, "diagnose", cmd.Use)
	assert.Equal(t, "Diagnose means pre-test Kuscia", cmd.Short)

	subCommands := cmd.Commands()
	assert.Len(t, subCommands, 2)
	assert.Equal(t, "cdr", subCommands[0].Use)
	assert.Equal(t, "network", subCommands[1].Use)
}

func TestNewCDRCommand(t *testing.T) {
	ctx := context.Background()
	cmd := NewCDRCommand(ctx)

	assert.Equal(t, "cdr", cmd.Use)
	assert.Equal(t, "Diagnose the effectiveness of domain route", cmd.Short)

	flags := cmd.Flags()
	assert.NotNil(t, flags.Lookup("speed"))
	assert.NotNil(t, flags.Lookup("rtt"))
	assert.NotNil(t, flags.Lookup("proxy-timeout"))
	assert.NotNil(t, flags.Lookup("size"))
	assert.NotNil(t, flags.Lookup("buffer"))
}

func TestNewNetworkCommand(t *testing.T) {
	ctx := context.Background()
	cmd := NewNeworkCommand(ctx)

	assert.Equal(t, "network", cmd.Use)
	assert.Equal(t, "Diagnose the status of network between domains", cmd.Short)

	flags := cmd.Flags()
	assert.NotNil(t, flags.Lookup("speed"))
	assert.NotNil(t, flags.Lookup("rtt"))
	assert.NotNil(t, flags.Lookup("proxy-timeout"))
	assert.NotNil(t, flags.Lookup("size"))
	assert.NotNil(t, flags.Lookup("buffer"))
	assert.NotNil(t, flags.Lookup("bidirection"))
}
