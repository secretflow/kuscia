package main

import (
	"testing"

	"github.com/apache/arrow/go/v13/arrow"
	"github.com/stretchr/testify/assert"
)

func TestMakePrimitiveRecords(t *testing.T) {
	records := MakePrimitiveRecords()
	assert.NotNil(t, records)
	assert.Greater(t, len(records), 0)

	schema := records[0].Schema()
	assert.Equal(t, 3, schema.Fields()[0].Type.ID()) // Check the first field type
}

func TestMakeBinaryRecords(t *testing.T) {
	records := MakeBinaryRecords()
	assert.NotNil(t, records)
	assert.Greater(t, len(records), 0)

	schema := records[0].Schema()
	assert.Equal(t, arrow.BINARY, schema.Field(0).Type.ID())
}
