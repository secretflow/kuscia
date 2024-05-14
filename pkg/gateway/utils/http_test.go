package utils

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestProbeEndpoint(t *testing.T) {

	type args struct {
		requestUrl string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "endpoint is 'https://test.example.com' should fail",
			args: args{
				requestUrl: "https://test.example.com",
			},
			wantErr: true,
		},
		{
			name: "endpoint is 'http://test.example.com' should fail",
			args: args{
				requestUrl: "http://test.example.com",
			},
			wantErr: true,
		},
		{
			name: "endpoint is 'https://127.0.0.1:1080' should fail",
			args: args{
				requestUrl: "https://127.0.0.1:1080",
			},
			wantErr: true,
		},
		{
			name: "endpoint is 'http://127.0.0.1:1080' should fail",
			args: args{
				requestUrl: "http://127.0.0.1:1080",
			},
			wantErr: true,
		},
		{
			name: "endpoint: 'user-kuscia-master:1080' is not valid",
			args: args{
				requestUrl: "user-kuscia-master:1080",
			},
			wantErr: true,
		},
		{
			name: "endpoint: 'valid url' is not valid",
			args: args{
				requestUrl: "valid url",
			},
			wantErr: true,
		},
		{
			name: "endpoint: 'https::///test.example.com' is not valid",
			args: args{
				requestUrl: "https::///test.example.com",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ProbePeerEndpoint(tt.args.requestUrl)
			assert.Equal(t, tt.wantErr, err != nil)
		})
	}

}
