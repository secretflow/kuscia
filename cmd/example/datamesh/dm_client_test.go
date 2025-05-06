package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateLocalFsDataSource(t *testing.T) {
	client := &MockFlightClient{}
	datasourceID, err := client.createLocalFsDataSource()
	assert.NotEmpty(t, datasourceID)
	assert.Nil(t, err)
}

func TestCreateDomainData(t *testing.T) {
	client := &MockFlightClient{
		testDataType: "primitives",
	}
	datasourceID := "test-datasource"
	domainDataID, err := client.createDomainData(datasourceID)
	assert.NotEmpty(t, domainDataID)
	assert.Nil(t, err)
}

func TestQueryDataSource(t *testing.T) {
	client := &MockFlightClient{}
	datasourceID := "test-datasource"
	err := client.QueryDataSource(datasourceID)
	assert.Nil(t, err)
}
