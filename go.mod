module github.com/secretflow/kuscia

go 1.19

require (
	github.com/armon/circbuf v0.0.0-20150827004946-bbbad097214e
	github.com/coredns/caddy v1.1.1
	github.com/coredns/coredns v1.10.0
	github.com/docker/distribution v2.8.1+incompatible
	github.com/envoyproxy/go-control-plane v0.11.1
	github.com/fsnotify/fsnotify v1.6.0
	github.com/gin-gonic/gin v1.8.1
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da
	github.com/golang/mock v1.6.0
	github.com/golang/protobuf v1.5.3
	github.com/google/go-cmp v0.5.9
	github.com/google/uuid v1.3.0
	github.com/miekg/dns v1.1.50
	github.com/mitchellh/go-homedir v1.1.0
	github.com/opencontainers/selinux v1.10.0
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.14.0
	github.com/secretflow/kuscia-envoy v0.0.0-20230705094915-8e153baebabc
	github.com/shirou/gopsutil/v3 v3.22.6
	github.com/spf13/cobra v1.6.1
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.8.3
	github.com/tidwall/match v1.1.1
	gitlab.com/jonas.jasas/condchan v0.0.0-20190210165812-36637ad2b5bc
	go.uber.org/atomic v1.9.0
	go.uber.org/zap v1.24.0
	golang.org/x/net v0.10.0
	google.golang.org/grpc v1.55.0
	google.golang.org/grpc/cmd/protoc-gen-go-grpc v1.3.0
	google.golang.org/protobuf v1.30.0
	gopkg.in/natefinch/lumberjack.v2 v2.0.0
	gopkg.in/yaml.v3 v3.0.1
	gotest.tools/v3 v3.0.3
	k8s.io/api v0.26.6
	k8s.io/apiextensions-apiserver v0.26.6
	k8s.io/apimachinery v0.26.6
	k8s.io/apiserver v0.26.6
	k8s.io/client-go v0.26.6
	k8s.io/code-generator v0.26.6
	k8s.io/component-base v0.26.6
	k8s.io/component-helpers v0.26.6
	k8s.io/cri-api v0.17.3
	k8s.io/klog/v2 v2.80.1
	k8s.io/kubectl v0.0.0
	k8s.io/kubelet v0.26.6
	k8s.io/kubernetes v1.26.6
	k8s.io/utils v0.0.0-20221107191617-1a15be271d1d
	sigs.k8s.io/controller-tools v0.9.2
	sigs.k8s.io/yaml v1.3.0

)

require github.com/lufia/plan9stats v0.0.0-20220517141722-cf486979b281 // indirect

require (
	github.com/fatih/color v1.13.0 // indirect
	github.com/frankban/quicktest v1.14.3 // indirect
	github.com/gobuffalo/flect v0.3.0 // indirect
	github.com/google/cadvisor v0.46.1 // indirect
	github.com/mattn/go-colorable v0.1.12 // indirect
	github.com/pelletier/go-toml/v2 v2.0.6 // indirect
	go.etcd.io/etcd/client/v2 v2.305.6 // indirect
	go.etcd.io/etcd/client/v3 v3.5.6 // indirect
	go.uber.org/multierr v1.8.0 // indirect
)

replace (
	github.com/opencontainers/runc => github.com/opencontainers/runc v1.1.5
	google.golang.org/protobuf => google.golang.org/protobuf v1.28.1
	k8s.io/api => k8s.io/api v0.26.6
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.26.6
	k8s.io/apimachinery => k8s.io/apimachinery v0.26.6
	k8s.io/apiserver => k8s.io/apiserver v0.26.6
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.26.6
	k8s.io/client-go => k8s.io/client-go v0.26.6
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.26.6
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.26.6
	k8s.io/code-generator => k8s.io/code-generator v0.26.6
	k8s.io/component-base => k8s.io/component-base v0.26.6
	k8s.io/component-helpers => k8s.io/component-helpers v0.26.6
	k8s.io/controller-manager => k8s.io/controller-manager v0.26.6
	k8s.io/cri-api => k8s.io/cri-api v0.26.6
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.26.6
	k8s.io/dynamic-resource-allocation => k8s.io/dynamic-resource-allocation v0.26.6
	k8s.io/kms => k8s.io/kms v0.26.6
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.26.6
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.26.6
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.26.6
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.26.6
	k8s.io/kubectl => k8s.io/kubectl v0.26.6
	k8s.io/kubelet => k8s.io/kubelet v0.26.6
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.26.6
	k8s.io/metrics => k8s.io/metrics v0.26.6
	k8s.io/mount-utils => k8s.io/mount-utils v0.26.6
	k8s.io/pod-security-admission => k8s.io/pod-security-admission v0.26.6
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.26.6
)
