package main

import (
	"fmt"
	"strings"
	//any "github.com/golang/protobuf/ptypes/any"
	admin "github.com/envoyproxy/go-control-plane/envoy/admin/v3"
	//bootstrap "github.com/envoyproxy/go-control-plane/envoy/config/bootstrap/v3"
	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/any"
	"io"
	"io/ioutil"
	"log"
	"net/http"
)

func main() {
	resp, err := http.Get("http://localhost:10000/config_dump?format=json")
	if err != nil {
		log.Fatal(err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
		}
	}(resp.Body)

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	var umrsh = jsonpb.Unmarshaler{}
	umrsh.AllowUnknownFields = true
	bootstrapConfigDump, _ := ptypes.MarshalAny(&admin.BootstrapConfigDump{})

	configDump := &admin.ConfigDump {Configs: []*any.Any{
		bootstrapConfigDump,
		}}
	um := &jsonpb.Unmarshaler{}
	err = um.Unmarshal(strings.NewReader(string(body)), configDump)
	if err != nil {
                log.Fatal(err)
        }
	fmt.Println(proto.MarshalTextString(configDump))
}

