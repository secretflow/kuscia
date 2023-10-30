package service

import (
	"context"
	"reflect"
	"testing"

	"github.com/secretflow/kuscia/pkg/confmanager/config"
	_ "github.com/secretflow/kuscia/pkg/secretbackend/mem"
	"github.com/secretflow/kuscia/pkg/web/asserts"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/confmanager"
)

func newMemDriverConfigurationService() (IConfigurationService, error) {
	srv, err := NewConfigurationService(config.ConfManagerConfig{
		BackendConfig: &config.SecretBackendConfig{
			Driver: "mem",
			Params: map[string]any{},
		},
	})
	if err != nil {
		return nil, err
	}
	return srv, nil
}

func TestNewConfigurationService(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "new mem driver configuration service should return no error",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := newMemDriverConfigurationService()
			if (err != nil) != tt.wantErr {
				t.Errorf("NewConfigurationService() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func Test_configurationService_CreateConfiguration(t *testing.T) {
	type args struct {
		ctx     context.Context
		request *confmanager.CreateConfigurationRequest
		cn      string
	}
	tests := []struct {
		name     string
		args     args
		wantCode int32
	}{
		{
			name: "new mem driver configuration service for alice cn set should success",
			args: args{
				ctx: context.Background(),
				request: &confmanager.CreateConfigurationRequest{
					Id:      "alice",
					Content: "alice",
				},
				cn: "alice",
			},
			wantCode: 0,
		},
		{
			name: "new mem driver configuration service for kuscia cn set should success",
			args: args{
				ctx: context.Background(),
				request: &confmanager.CreateConfigurationRequest{
					Id:      "kuscia",
					Content: "kuscia",
				},
				cn: "kuscia",
			},
			wantCode: 0,
		},
	}
	s, err := newMemDriverConfigurationService()
	_ = asserts.IsNil(err, "new mem driver should return no error")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := s.CreateConfiguration(tt.args.ctx, tt.args.request, tt.args.cn); !reflect.DeepEqual(got.Status.Code, tt.wantCode) {
				t.Errorf("CreateConfiguration().Status.Code = %v, wantCode %v", got.Status.Code, tt.wantCode)
			}
		})
	}
}

func Test_configurationService_QueryConfiguration(t *testing.T) {
	type args struct {
		ctx     context.Context
		request *confmanager.QueryConfigurationRequest
		cn      string
	}
	tests := []struct {
		name       string
		args       args
		wantCode   int32
		wantResult map[string]*confmanager.QueryConfigurationResult
	}{
		{
			name: "new mem driver set configuration for alice should success",
			args: args{
				ctx: context.Background(),
				request: &confmanager.QueryConfigurationRequest{
					Ids: []string{"alice", "kuscia"},
				},
				cn: "alice",
			},
			wantCode: 0,
			wantResult: map[string]*confmanager.QueryConfigurationResult{
				"alice": {
					Success: true,
					Content: "alice",
				},
				"kuscia": {
					Success: false,
				},
			},
		},
		{
			name: "new mem driver set configuration for kuscia should success",
			args: args{
				ctx: context.Background(),
				request: &confmanager.QueryConfigurationRequest{
					Ids: []string{"alice", "kuscia"},
				},
				cn: "kuscia",
			},
			wantCode: 0,
			wantResult: map[string]*confmanager.QueryConfigurationResult{
				"alice": {
					Success: true,
					Content: "alice",
				},
				"kuscia": {
					Success: true,
					Content: "kuscia",
				},
			},
		},
	}
	s, err := newMemDriverConfigurationService()
	_ = asserts.IsNil(err, "new mem driver should return no error")
	response := s.CreateConfiguration(context.Background(), &confmanager.CreateConfigurationRequest{
		Id:      "alice",
		Content: "alice",
	}, "alice")
	_ = asserts.IsTrue(response.Status.Code == 0, "new mem driver set configuration for alice should success")
	response = s.CreateConfiguration(context.Background(), &confmanager.CreateConfigurationRequest{
		Id:      "kuscia",
		Content: "kuscia",
	}, "kuscia")
	_ = asserts.IsTrue(response.Status.Code == 0, "new mem driver set configuration for kuscia should success")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.QueryConfiguration(tt.args.ctx, tt.args.request, tt.args.cn)
			if !reflect.DeepEqual(got.Status.Code, tt.wantCode) {
				t.Errorf("QueryConfiguration().Status.Code = %v, want %v", got.Status.Code, tt.wantCode)
			}
			if len(got.Configurations) != len(tt.wantResult) {
				t.Errorf("QueryConfiguration().len = %v, want %v", len(got.Configurations), len(tt.wantResult))
			}
			for id, c := range got.Configurations {
				_ = asserts.IsTrue(c.Success == tt.wantResult[id].Success, "confID=%s success should be equals")
				if c.Success {
					_ = asserts.IsTrue(c.Content == tt.wantResult[id].Content, "confID=%s content should be equals")
				}
			}
		})
	}
}
