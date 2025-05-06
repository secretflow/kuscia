package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateLocalFsDataSource(t *testing.T) {
	client := &MockFlightClient{
		datasourceSvc: &MockDatasourceService{}, // Mock implementation
	}
	id, err := client.createLocalFsDataSource()
	assert.NotEmpty(t, id)
	assert.NoError(t, err)
}

func TestCreateDomainData(t *testing.T) {
	client := &MockFlightClient{
		datasourceSvc: &MockDatasourceService{}, // Mock implementation
	}
	domainDataID, err := client.createDomainData("test-datasource-id")
	assert.NotEmpty(t, domainDataID)
	assert.NoError(t, err)
}

func TestQueryDataSource(t *testing.T) {
	client := &MockFlightClient{}
	datasourceID := "test-datasource"
	err := client.QueryDataSource(datasourceID)
	assert.Nil(t, err)
}
