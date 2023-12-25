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
	flight2 "github.com/secretflow/kuscia/pkg/datamesh/flight"
	"github.com/secretflow/kuscia/pkg/datamesh/flight/example"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/paths"
	"github.com/secretflow/kuscia/pkg/utils/tls"
	"github.com/secretflow/kuscia/proto/api/v1alpha1"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/datamesh"
)

var (
	metaServerEndpoint = fmt.Sprintf("%s:8071", dataMeshHost)
)

type MockFlightClient struct {
	testDataType      string
	outputCSVFilePath string
	flightClient      flight.Client
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
		datasourceID string
		domainDataID string

		localDatasourceID string
		ossDatasourceID   string
		mysqlDatasourceID string
	)

	m.flightClient, err = createFlightClient(metaServerEndpoint, config)

	if localDatasourceID, err = m.createLocalFsDataSource(); err != nil {
		return err
	}

	if ossDatasourceID, err = m.createOSSDataSource(); err != nil {
		return err
	}

	if mysqlDatasourceID, err = m.createMysqlDataSource(); err != nil {
		return err
	}
	nlog.Infof("LocalFsID:%s ossID:%s mysqlID:%s", localDatasourceID, ossDatasourceID, mysqlDatasourceID)

	datasourceID = localDatasourceID
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
	createDatasourceReq := &datamesh.ActionCreateDomainDataSourceRequest{
		Request: &datamesh.CreateDomainDataSourceRequest{
			DatasourceId: "",
			Name:         "default-datasource",
			Type:         "localfs",
			Info: &datamesh.DataSourceInfo{
				Localfs: &datamesh.LocalDataSourceInfo{
					Path: "./",
				},
			},
			AccessDirectly: false,
		},
	}
	return m.createDataSource(createDatasourceReq)
}

func (m *MockFlightClient) createOSSDataSource() (string, error) {
	// CreateDataSource
	createDatasourceReq := &datamesh.ActionCreateDomainDataSourceRequest{
		Request: &datamesh.CreateDomainDataSourceRequest{
			DatasourceId: "",
			Name:         "oss-ds",
			Type:         "oss",
			Info: &datamesh.DataSourceInfo{
				Oss: &datamesh.OssDataSourceInfo{
					Endpoint:        fmt.Sprintf("%s:9000", dataMeshHost),
					Bucket:          "testBucket",
					Prefix:          "/xxx/",
					AccessKeyId:     "ak",
					AccessKeySecret: "sk",
					StorageType:     "minio",
				},
			},
			AccessDirectly: false,
		},
	}
	return m.createDataSource(createDatasourceReq)
}

func (m *MockFlightClient) createMysqlDataSource() (string, error) {
	// CreateDataSource
	createDatasourceReq := &datamesh.ActionCreateDomainDataSourceRequest{
		Request: &datamesh.CreateDomainDataSourceRequest{
			DatasourceId: "",
			Name:         "mysql-ds",
			Type:         "mysql",
			Info: &datamesh.DataSourceInfo{
				Database: &datamesh.DatabaseDataSourceInfo{
					Endpoint: fmt.Sprintf("%s:3306", dataMeshHost),
					User:     "root",
					Password: "passwd",
					Database: "demo",
				},
			},
			AccessDirectly: false,
		},
	}
	return m.createDataSource(createDatasourceReq)
}

func (m *MockFlightClient) createDataSource(createDatasourceReq *datamesh.ActionCreateDomainDataSourceRequest) (string, error) {
	msg, err := proto.Marshal(createDatasourceReq)
	if err != nil {
		return "", err
	}

	action := &flight.Action{
		Type: "ActionCreateDomainDataSourceRequest",
		Body: msg,
	}
	resp, err := m.flightClient.DoAction(context.Background(), action)
	if err != nil {
		nlog.Warnf("DoActionCreateDomainDataSourceRequest err: %v", err)
		return "", err
	}
	result, err := resp.Recv()
	if err != nil {
		nlog.Warnf("Receive ActionCreateDomainDataSourceResponse err: %v", err)
	}

	var createDatasourceResp datamesh.ActionCreateDomainDataSourceResponse
	if err := proto.Unmarshal(result.Body, &createDatasourceResp); err != nil {
		nlog.Warnf("Unmarshal ActionCreateDomainDataSourceResponse err: %v", err)
	}
	datasourceID := createDatasourceResp.Response.Data.DatasourceId
	nlog.Infof("data-source-id:%s, type is:%s", datasourceID, createDatasourceReq.Request.Type)
	return datasourceID, nil
}

func (m *MockFlightClient) QueryDataSource(datasourceID string) error {
	queryDatasourceReq := &datamesh.ActionQueryDomainDataSourceRequest{
		Request: &datamesh.QueryDomainDataSourceRequest{
			DatasourceId: datasourceID,
		},
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

	var queryDatasourceResp datamesh.ActionQueryDomainDataSourceResponse
	if err := proto.Unmarshal(result.Body, &queryDatasourceResp); err != nil {
		nlog.Warnf("Unmarshal ActionQueryDomainDataSourceResponse err: %v", err)
		return err
	}

	dsResp := queryDatasourceResp.Response.Data
	nlog.Infof("dsType is %s, dsInfo:%v", dsResp.Type, dsResp.Info)
	return nil
}

func (m *MockFlightClient) createDomainData(datasourceID string) (string, error) {
	cols, err := common.ArrowSchema2DataMeshColumns(example.Records[m.testDataType][0].Schema())
	nlog.Infof("DomainData Cols:%v", cols)
	if err != nil {
		return "", err
	}
	createDomainDataReq := &datamesh.ActionCreateDomainDataRequest{
		Request: &datamesh.CreateDomainDataRequest{
			Header:       nil,
			DomaindataId: fmt.Sprintf("%s-%d", m.testDataType, time.Now().UnixNano()),
			Name:         m.testDataType,
			Type:         "table",
			RelativeUri:  "b.csv", // for mysql table, RelativeUri should be "dbName.tableName"
			DatasourceId: datasourceID,
			Columns:      cols,
			FileFormat:   v1alpha1.FileFormat_CSV,
		},
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

	var createDomainDataResp datamesh.ActionCreateDomainDataResponse
	if err := proto.Unmarshal(result.Body, &createDomainDataResp); err != nil {
		nlog.Warnf("Unmarshal ActionCreateDomainDataResponse err: %v", err)
		return "", err
	}
	domainDataID := createDomainDataResp.Response.Data.DomaindataId
	return domainDataID, nil
}

func (m *MockFlightClient) getData(domainDataID string, useRawData bool) error {
	cmd := &datamesh.CommandDomainDataQuery{
		DomaindataId: domainDataID,
	}

	if useRawData {
		cmd.ContentType = datamesh.ContentType_RAW
	}
	desc, err := flight2.DescForCommand(cmd)
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
	desc, err := flight2.DescForCommand(cmd)
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
	ticket, err := flight2.GetTicketDomainDataQuery(flightInfo.Endpoint[0].Ticket)
	if err != nil {
		nlog.Warnf("GetTicketDomainDataQuery fail:%v", err)
	}
	ticketID := ticket.DomaindataHandle

	putDesc, err := flight2.DescForCommand(ticket)
	if err != nil {
		nlog.Warnf("Generate put desc fail:%v", err)
	}

	flightClient, err := createFlightClient(dpURI, nil)
	if err != nil {
		nlog.Warnf("Connect to data proxy fail:%v", err)
		return err
	}
	defer flightClient.Close()
	var records []arrow.Record
	if m.testDataType == binaryTestData {
		nlog.Infof("MakeBinaryRecords ")
		records = example.MakeBinaryRecords()
	} else {
		nlog.Infof("MakePrimitiveRecords ")
		records = example.MakePrimitiveRecords()
	}

	stream, err := flightClient.DoPut(context.Background())
	if err != nil {
		nlog.Warnf("DoPut Fail:%v", err)
		return err
	}

	wr := flight.NewRecordWriter(stream, ipc.WithSchema(records[0].Schema()))
	wr.SetFlightDescriptor(putDesc)
	defer wr.Close()

	for idx, r := range records {
		if err := wr.WriteWithAppMetadata(r, []byte(fmt.Sprintf("%d_%s", idx,
			ticketID)) /*metadata*/); err != nil {
			nlog.Warnf("write data to data proxy fail:%v", err)
			return err
		}

	}
	pr, err := stream.Recv()
	if err != nil {
		nlog.Warnf("recv error when write data to data proxy:%v", err)
		return err
	}

	acked := pr.GetAppMetadata()
	if len(acked) > 0 {
		nlog.Infof("got metadata:%s", string(acked))
	}
	stream.CloseSend()
	nlog.Infof("put data succ")
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
	nlog.Infof("client get data succ")
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
	nlog.Infof("endpoint is %s", endpoint)

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
		nlog.Errorf("create grpc conn to %s fail: %v", endpoint, err)
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
		nlog.Warnf("cannot create certs dir: %v", err)
	}

	if err := tls.CreateCAFile("testca", signingCertFile, signingKeyFile); err != nil {
		nlog.Warnf("create ca file fail: %v", err)
		return nil, err
	}
	DomainCACert, err := tls.ParseCert(nil, signingCertFile)
	if err != nil {
		nlog.Warnf("parser domainCaCert fail:%v, err", err)
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
