package server

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestServerRun(t *testing.T) {
	port := 20006
	server := NewHTTPServerBean(port)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err := server.Run(ctx)
	assert.Nil(t, err)
}
