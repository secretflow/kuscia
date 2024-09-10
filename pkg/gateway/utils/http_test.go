package utils

import (
	"net/http"
	"testing"
	"time"

	"gopkg.in/h2non/gock.v1"
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

func TestParseURL(t *testing.T) {
	type args struct {
		url string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		want1   string
		want2   uint32
		want3   string
		wantErr bool
	}{
		{"case 0", args{"https://test.example.com"}, "https", "test.example.com", 443, "", false},
		{"case 1", args{"http://test.example.com"}, "http", "test.example.com", 80, "", false},
		{"case 2", args{"https://127.0.0.1:1080"}, "https", "127.0.0.1", 1080, "", false},
		{"case 3", args{"http://127.0.0.1:1080"}, "http", "127.0.0.1", 1080, "", false},
		{"case 4", args{"https://user-kuscia-master:1080"}, "https", "user-kuscia-master", 1080, "", false},
		{"case 5", args{"http://user-kuscia-master:1080"}, "http", "user-kuscia-master", 1080, "", false},
		{"case 6", args{"https::///test.example.com"}, "", "", 0, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, got2, got3, err := ParseURL(tt.args.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseURL() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("ParseURL() got1 = %v, want %v", got1, tt.want1)
			}
			if got2 != tt.want2 {
				t.Errorf("ParseURL() got2 = %v, want %v", got2, tt.want2)
			}
			if got3 != tt.want3 {
				t.Errorf("ParseURL() got3 = %v, want %v", got3, tt.want3)
			}
		})
	}
}

func TestDoHTTPWithRetry(t *testing.T) {
	type Result struct {
		Value int `json:"value"`
	}

	defer gock.Off()

	gock.New("http://127.0.0.1:80").
		Get("/handshake").
		MatchType("json").
		Reply(200).
		JSON(map[string]int{"value": 100})

	gock.New("http://" + "kuscia-handshake.bob.svc").
		Post("/handshake").
		MatchType("json").
		Reply(200).
		JSON(map[string]int{"value": 200})

	result := &Result{}

	type args struct {
		in  interface{}
		out interface{}
		hp  *HTTPParam
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
		wantVal int
	}{
		{"case 0", args{in: &Result{Value: 0}, out: result, hp: &HTTPParam{Path: "/handshake", Method: http.MethodGet}}, false, 100},
		{"case 1", args{in: &Result{Value: 0}, out: result, hp: &HTTPParam{Path: "/handshake", Method: http.MethodPost}}, true, 100},
		{"case 2", args{in: &Result{Value: 0}, out: result, hp: &HTTPParam{Path: "/handshake", KusciaHost: "kuscia-handshake.bob.svc", Method: http.MethodPost, Transit: true}}, false, 200},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := DoHTTPWithRetry(tt.args.in, tt.args.out, tt.args.hp, time.Second, 3); (err != nil) != tt.wantErr {
				t.Errorf("DoHTTP() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
