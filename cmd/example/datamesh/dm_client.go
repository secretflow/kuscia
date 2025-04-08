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

package main

import (
	"context"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/apache/arrow/go/v13/arrow"
	"github.com/apache/arrow/go/v13/arrow/array"
	"github.com/apache/arrow/go/v13/arrow/csv"
	"github.com/apache/arrow/go/v13/arrow/flight"
	"github.com/apache/arrow/go/v13/arrow/ipc"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/protobuf/proto"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/datamesh/dataserver/utils"
	"github.com/secretflow/kuscia/pkg/kusciaapi/service"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/paths"
	"github.com/secretflow/kuscia/pkg/utils/tls"
	"github.com/secretflow/kuscia/proto/api/v1alpha1"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/datamesh"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/kusciaapi"
)

var (
	metaServerEndpoint = fmt.Sprintf("%s:8071", dataMeshHost)
)

type MockFlightClient struct {
	testDataType      string
	outputCSVFilePath string
	flightClient      flight.Client
	datasourceSvc     service.IDomainDataSourceService
}

type CertsConfig struct {
	serverCertFile string
	serverKeyFile  string

	clientCertFile string
	clientKeyFile  string

	caFile string
}

func (m *MockFlightClient) start(config *CertsConfig) error {
	var (
		err          error
		domainDataID string

		localDatasourceID      string
		ossDatasourceID        string
		mysqlDatasourceID      string
		postgresqlDatasourceID string
	)

	m.flightClient, _ = createFlightClient(metaServerEndpoint, config)
	if localDatasourceID, err = m.createLocalFsDataSource(); err != nil {
		return err
	}

	if ossDatasourceID, err = m.createOSSDataSource(); err != nil {
		return err
	}

	if mysqlDatasourceID, err = m.createMysqlDataSource(); err != nil {
		return err
	}

	if postgresqlDatasourceID, err = m.createPostgresqlDataSource(); err != nil {
		return err
	}
	nlog.Infof("LocalFsID:%s ossID:%s mysqlID:%s postgresqlID:%s", localDatasourceID, ossDatasourceID, mysqlDatasourceID, postgresqlDatasourceID)

	datasourceID := localDatasourceID
	if err := m.QueryDataSource(datasourceID); err != nil {
		nlog.Warnf("Query datasource info fail: %v", err)
		return err
	}

	if domainDataID, err = m.createDomainData(datasourceID); err != nil {
		return err
	}

	if err := m.putData(domainDataID); err != nil {
		return err
	}

	// for a binary file or you want read a structure file as binary file, set useRawData=true
	// if  useRawData=true, the result only contains one column with type = BinaryString
	if err := m.getData(domainDataID, true); err != nil {
		return err
	}

	return m.flightClient.Close()
}
func (m *MockFlightClient) createLocalFsDataSource() (string, error) {
	// CreateDataSource
	createDatasourceReq := &kusciaapi.CreateDomainDataSourceRequest{
		Header:       nil,
		DomainId:     mockDomain,
		DatasourceId: "",
		Type:         "localfs",
		Info: &kusciaapi.DataSourceInfo{
			Localfs: &kusciaapi.LocalDataSourceInfo{
				Path: "./",
			},
		},
	}

	return m.createDataSource(createDatasourceReq)
}

func (m *MockFlightClient) createOSSDataSource() (string, error) {
	// CreateDataSource
	createDatasourceReq := &kusciaapi.CreateDomainDataSourceRequest{
		Header:       nil,
		DomainId:     mockDomain,
		DatasourceId: "",
		Type:         "oss",
		Info: &kusciaapi.DataSourceInfo{
			Oss: &kusciaapi.OssDataSourceInfo{
				Endpoint:        fmt.Sprintf("%s:9000", dataMeshHost),
				Bucket:          "testBucket",
				Prefix:          "/xxx/",
				AccessKeyId:     "ak",
				AccessKeySecret: "sk",
				StorageType:     "minio",
			},
		},
	}
	return m.createDataSource(createDatasourceReq)
}

func (m *MockFlightClient) createMysqlDataSource() (string, error) {
	// CreateDataSource
	createDatasourceReq := &kusciaapi.CreateDomainDataSourceRequest{
		Header:       nil,
		DomainId:     mockDomain,
		DatasourceId: "",
		Type:         "mysql",
		Info: &kusciaapi.DataSourceInfo{
			Database: &kusciaapi.DatabaseDataSourceInfo{
				Endpoint: fmt.Sprintf("%s:3306", dataMeshHost),
				User:     "root",
				Password: "passwd",
				Database: "demo",
			},
		},
	}
	return m.createDataSource(createDatasourceReq)
}

func (m *MockFlightClient) createPostgresqlDataSource() (string, error) {
	// CreateDatasource
	createDatasourceReq := &kusciaapi.CreateDomainDataSourceRequest{
		Header:       nil,
		DomainId:     mockDomain,
		DatasourceId: "",
		Type:         "postgresql",
		Info: &kusciaapi.DataSourceInfo{
			Database: &kusciaapi.DatabaseDataSourceInfo{
				Endpoint: fmt.Sprintf("%s:3306", dataMeshHost),
				User:     "root",
				Password: "passwd",
				Database: "demo",
			},
		},
	}
	return m.createDataSource(createDatasourceReq)
}

func (m *MockFlightClient) createDataSource(createDatasourceReq *kusciaapi.CreateDomainDataSourceRequest) (string, error) {
	resp := m.datasourceSvc.CreateDomainDataSource(context.Background(), createDatasourceReq)
	if resp == nil {
		err := fmt.Errorf("create domainDataSource source fail")
		nlog.Warn(err)
		return "", err
	}
	if resp.Status != nil && resp.Status.Code != 0 {
		err := fmt.Errorf("create domainDataSource %s source fail:%v", createDatasourceReq.DatasourceId, resp.Status)
		nlog.Warn(err)
		return "", err
	}
	nlog.Infof("Data-source-id:%s, type is:%s", resp.Data.DatasourceId, createDatasourceReq.Type)
	return resp.Data.DatasourceId, nil
}

func (m *MockFlightClient) QueryDataSource(datasourceID string) error {
	queryDatasourceReq := &datamesh.QueryDomainDataSourceRequest{

		DatasourceId: datasourceID,
	}
	msg, err := proto.Marshal(queryDatasourceReq)
	if err != nil {
		return err
	}

	action := &flight.Action{
		Type: "ActionQueryDomainDataSourceRequest",
		Body: msg,
	}
	resp, err := m.flightClient.DoAction(context.Background(), action)
	if err != nil {
		nlog.Warnf("DoActionQueryDomainDataSourceRequest err: %v", err)
	}
	result, err := resp.Recv()
	if err != nil {
		nlog.Warnf("Receive ActionQueryDomainDataSourceResponse err: %v", err)
		return err
	}

	var queryDatasourceResp datamesh.QueryDomainDataSourceResponse
	if err := proto.Unmarshal(result.Body, &queryDatasourceResp); err != nil {
		nlog.Warnf("Unmarshal ActionQueryDomainDataSourceResponse err: %v", err)
		return err
	}

	dsResp := queryDatasourceResp.Data
	nlog.Infof("DataSourceType is %s, dsInfo:%v", dsResp.Type, dsResp.Info)
	return nil
}

func (m *MockFlightClient) createDomainData(datasourceID string) (string, error) {
	cols, err := common.ArrowSchema2DataMeshColumns(Records[m.testDataType][0].Schema())
	nlog.Infof("DomainData Cols:%v", cols)
	if err != nil {
		return "", err
	}
	createDomainDataReq := &datamesh.CreateDomainDataRequest{
		Header:       nil,
		DomaindataId: fmt.Sprintf("%s-%d", m.testDataType, time.Now().UnixNano()),
		Name:         m.testDataType,
		Type:         "table",
		RelativeUri:  "b.csv", // for mysql table, RelativeUri should be "dbName.tableName"
		DatasourceId: datasourceID,
		Columns:      cols,
		FileFormat:   v1alpha1.FileFormat_CSV,
	}
	msg, err := proto.Marshal(createDomainDataReq)
	if err != nil {
		return "", err
	}

	action := &flight.Action{
		Type: "ActionCreateDomainDataRequest",
		Body: msg,
	}
	resp, err := m.flightClient.DoAction(context.Background(), action)
	if err != nil {
		nlog.Warnf("DoActionCreateDomainDataRequest err: %v", err)
	}
	resp.CloseSend()

	result, err := resp.Recv()
	if err != nil {
		nlog.Warnf("Recv ActionCreateDomainDataResponse err: %v", err)
	}

	var createDomainDataResp datamesh.CreateDomainDataResponse
	if err := proto.Unmarshal(result.Body, &createDomainDataResp); err != nil {
		nlog.Warnf("Unmarshal ActionCreateDomainDataResponse err: %v", err)
		return "", err
	}
	domainDataID := createDomainDataResp.Data.DomaindataId
	return domainDataID, nil
}

func (m *MockFlightClient) getData(domainDataID string, useRawData bool) error {
	cmd := &datamesh.CommandDomainDataQuery{
		DomaindataId: domainDataID,
	}

	if useRawData {
		cmd.ContentType = datamesh.ContentType_RAW
	}
	desc, err := utils.DescForCommand(cmd)
	if err != nil {
		nlog.Warnf(err.Error())
		return err
	}

	flightInfo, err := m.flightClient.GetFlightInfo(context.Background(), desc)
	if err != nil {
		nlog.Warnf("Get flightInfo fail:%v", err)
	}

	dpURI := flightInfo.Endpoint[0].Location[0].Uri
	flightClient, err := createFlightClient(dpURI, nil)
	if err != nil {
		nlog.Warnf("Connect to data proxy fail:%v", err)
		return err
	}

	defer flightClient.Close()
	flightData, err := flightClient.DoGet(context.Background(), flightInfo.Endpoint[0].Ticket)
	if err != nil {
		nlog.Warnf("DoGet fail:%v", err)
		return err
	}
	r, err := flight.NewRecordReader(flightData)
	if err != nil {
		nlog.Warnf("NewRecordReader fail:%v", err)
		return err
	}

	file, err := os.OpenFile(m.outputCSVFilePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		nlog.Warnf("Open file:%s", err)
		return err
	}
	defer file.Close()

	if useRawData {
		return writeBinaryCSV(file, r)
	}
	return writeStructureCSV(file, r)
}

func (m *MockFlightClient) putData(domainDataID string) error {
	cmd := &datamesh.CommandDomainDataUpdate{
		DomaindataId: domainDataID,
		FileWriteOptions: &datamesh.FileWriteOptions{
			Options: &datamesh.FileWriteOptions_CsvOptions{
				CsvOptions: &datamesh.CSVWriteOptions{FieldDelimiter: ","},
			},
		},
	}
	desc, err := utils.DescForCommand(cmd)
	if err != nil {
		nlog.Warnf(err.Error())
		return err
	}

	flightInfo, err := m.flightClient.GetFlightInfo(context.Background(), desc)
	if err != nil {
		nlog.Warnf("Get flightInfo fail:%v", err)
		return err
	}

	dpURI := flightInfo.Endpoint[0].Location[0].Uri
	ticket := flightInfo.Endpoint[0].Ticket
	ticketID := ticket.GetTicket()

	flightClient, err := createFlightClient(dpURI, nil)
	if err != nil {
		nlog.Warnf("Connect to data proxy fail:%v", err)
		return err
	}
	defer flightClient.Close()
	var records []arrow.Record
	if m.testDataType == binaryTestData {
		nlog.Infof("MakeBinaryRecords ")
		records = MakeBinaryRecords()
	} else {
		nlog.Infof("MakePrimitiveRecords ")
		records = MakePrimitiveRecords()
	}

	stream, err := flightClient.DoPut(context.Background())
	if err != nil {
		nlog.Warnf("DoPut Fail:%v", err)
		return err
	}

	if err := stream.SendMsg(&flight.Ticket{
		Ticket: ticketID,
	}); err != nil {
		nlog.Warnf("Send tick to stream Fail:%v", err)
		return err
	}

	wr := flight.NewRecordWriter(stream, ipc.WithSchema(records[0].Schema()))
	defer wr.Close()

	for idx, r := range records {
		if err := wr.WriteWithAppMetadata(r, []byte(fmt.Sprintf("%d_%s", idx,
			ticketID)) /*metadata*/); err != nil {
			nlog.Warnf("Write data to data proxy fail:%v", err)
			return err
		}

	}
	pr, err := stream.Recv()
	if err != nil {
		nlog.Warnf("Recv error when write data to data proxy:%v", err)
		return err
	}

	acked := pr.GetAppMetadata()
	if len(acked) > 0 {
		nlog.Infof("Got metadata:%s", string(acked))
	}
	stream.CloseSend()
	nlog.Infof("Put data succ")
	return nil
}

func writeStructureCSV(writer io.Writer, r *flight.Reader) error {
	w := csv.NewWriter(writer, r.Schema(), csv.WithComma(','))
	idx := 0
	for {
		rec, err := r.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			nlog.Warnf("Read data error: %v", err)
			return err
		}
		nlog.Infof("%v", rec)
		w.Write(rec)
		idx++
	}
	w.Flush()
	nlog.Infof("Client get data succ")
	return nil
}

func writeBinaryCSV(writer io.Writer, r *flight.Reader) error {
	for {
		rec, err := r.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			nlog.Warnf("Read data error: %v", err)
			return err
		}
		col := rec.Column(0)
		arr := col.(*array.Binary)
		for i := 0; i < arr.Len(); i++ {
			writer.Write(arr.Value(i))
		}
	}
	nlog.Infof("Client get data succ")
	return nil
}

func createFlightClient(endpoint string, config *CertsConfig) (flight.Client, error) {
	// remove schema grpc+tcp:
	uriSlice := strings.Split(endpoint, "//")
	if len(uriSlice) == 2 {
		endpoint = uriSlice[1]
	}
	nlog.Infof("Endpoint is %s", endpoint)

	dialOpts := []grpc.DialOption{
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                1 * time.Minute,
			Timeout:             30 * time.Second,
			PermitWithoutStream: true,
		}),
	}
	if config == nil {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		clientTLSConfig, err := tls.BuildClientTLSConfigViaPath(config.caFile, config.clientCertFile, config.clientKeyFile)
		if err != nil {
			nlog.Fatalf("Failed to init server tls config: %v", err)
		}

		dialOpts = append(dialOpts, grpc.WithTransportCredentials(credentials.NewTLS(clientTLSConfig)))
	}

	conn, err := grpc.Dial(endpoint, dialOpts...)
	if err != nil {
		nlog.Errorf("Create grpc conn to %s fail: %v", endpoint, err)
		return nil, err
	}
	return flight.NewClientFromConn(conn, nil), nil
}

func createClientCertificate() (*CertsConfig, error) {
	certsDir := filepath.Join("./", "dp-certs")
	serverKeyFile := filepath.Join(certsDir, "server.key")
	serverCertFile := filepath.Join(certsDir, "server.crt")
	clientKeyFile := filepath.Join(certsDir, "client.key")
	clientCertFile := filepath.Join(certsDir, "client.crt")
	signingCertFile := filepath.Join(certsDir, "ca.crt")
	signingKeyFile := filepath.Join(certsDir, "ca.key")

	if err := paths.EnsureDirectory(certsDir, true); err != nil {
		nlog.Warnf("Cannot create certs dir: %v", err)
	}

	if err := tls.CreateCAFile("testca", signingCertFile, signingKeyFile); err != nil {
		nlog.Warnf("Create ca file fail: %v", err)
		return nil, err
	}
	DomainCACert, err := tls.ParseCert(nil, signingCertFile)
	if err != nil {
		nlog.Warnf("Parser domainCaCert fail:%v, err", err)
		return nil, err
	}

	DomainCAKey, err := tls.ParseKey(nil, signingKeyFile)
	if err != nil {
		return nil, err
	}

	netIPs := []net.IP{
		net.IPv4(127, 0, 0, 1),
	}

	// generate Server Certs
	serverCertOut, err := os.OpenFile(serverCertFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return nil, err
	}
	serverKeyOut, err := os.OpenFile(serverKeyFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return nil, err
	}
	clientCertOut, err := os.OpenFile(clientCertFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return nil, err
	}
	clientKeyOut, err := os.OpenFile(clientKeyFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return nil, err
	}

	cert := &x509.Certificate{
		SerialNumber: big.NewInt(int64(uuid.New().ID())),
		Subject:      pkix.Name{CommonName: netIPs[0].String()},
		IPAddresses:  netIPs,
		DNSNames:     []string{"127.0.0.1", "localhost"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(10, 0, 0),
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	if err := tls.GenerateX509KeyPair(DomainCACert, DomainCAKey, cert, serverCertOut, serverKeyOut); err != nil {
		return nil, err
	}
	serverCertOut.Close()
	serverKeyOut.Close()

	cert = &x509.Certificate{
		SerialNumber: big.NewInt(int64(uuid.New().ID())),
		Subject:      pkix.Name{OrganizationalUnit: []string{"test-pod"}, CommonName: "test-pod"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(10, 0, 0),
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}

	if err := tls.GenerateX509KeyPair(DomainCACert, DomainCAKey, cert, clientCertOut, clientKeyOut); err != nil {
		return nil, err
	}
	clientCertOut.Close()
	clientKeyOut.Close()

	config := &CertsConfig{
		serverCertFile: serverCertFile,
		serverKeyFile:  serverKeyFile,
		clientCertFile: clientCertFile,
		clientKeyFile:  clientKeyFile,
		caFile:         signingCertFile,
	}
	return config, nil
}
