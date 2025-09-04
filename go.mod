module github.com/secretflow/kuscia

go 1.22

require (
	github.com/DATA-DOG/go-sqlmock v1.5.2
	github.com/agiledragon/gomonkey v2.0.2+incompatible
	github.com/agiledragon/gomonkey/v2 v2.11.0
	github.com/apache/arrow/go/v13 v13.0.0
	github.com/armon/circbuf v0.0.0-20150827004946-bbbad097214e
	github.com/aws/aws-sdk-go v1.50.10
	github.com/casbin/casbin/v2 v2.77.2
	github.com/containerd/cgroups/v3 v3.0.3
	github.com/coredns/caddy v1.1.1
	github.com/coredns/coredns v1.11.2
	github.com/docker/distribution v2.8.2+incompatible
	github.com/docker/go-units v0.5.0
	github.com/envoyproxy/go-control-plane v0.12.0
	github.com/fsnotify/fsnotify v1.7.0
	github.com/gin-gonic/gin v1.9.1
	github.com/go-sql-driver/mysql v1.6.0
	github.com/golang-jwt/jwt/v5 v5.2.2
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da
	github.com/golang/mock v1.6.0
	github.com/golang/protobuf v1.5.4
	github.com/google/go-cmp v0.6.0
	github.com/google/go-containerregistry v0.19.1
	github.com/google/uuid v1.6.0
	github.com/huandu/go-sqlbuilder v1.28.1
	github.com/johannesboyne/gofakes3 v0.0.0-20240513200200-99de01ee122d
	github.com/json-iterator/go v1.1.12
	github.com/lib/pq v1.10.9
	github.com/miekg/dns v1.1.58
	github.com/mitchellh/go-homedir v1.1.0
	github.com/moby/sys/mount v0.3.3
	github.com/moby/sys/mountinfo v0.7.1
	github.com/nxadm/tail v1.4.11
	github.com/olekukonko/tablewriter v0.0.5
	github.com/opencontainers/image-spec v1.1.0-rc5
	github.com/opencontainers/runtime-spec v1.1.1-0.20230823135140-4fec88fd00a4
	github.com/opencontainers/selinux v1.11.0
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.19.1
	github.com/samber/lo v1.47.0
	github.com/secretflow/kuscia-envoy v0.0.0-20240402083426-b0884d002f48
	github.com/shirou/gopsutil/v3 v3.22.6
	github.com/spf13/cobra v1.7.0
	github.com/spf13/pflag v1.0.10
	github.com/stathat/consistent v1.0.0
	github.com/stretchr/testify v1.9.0
	github.com/tidwall/match v1.1.1
	gitlab.com/jonas.jasas/condchan v0.0.0-20190210165812-36637ad2b5bc
	go.uber.org/atomic v1.11.0
	go.uber.org/zap v1.24.0
	golang.org/x/net v0.33.0
	golang.org/x/sys v0.28.0
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240116215550-a9fa1716bcac
	google.golang.org/grpc v1.61.0
	google.golang.org/grpc/cmd/protoc-gen-go-grpc v1.3.0
	google.golang.org/protobuf v1.33.0
	gopkg.in/h2non/gock.v1 v1.1.2
	gopkg.in/natefinch/lumberjack.v2 v2.2.1
	gopkg.in/yaml.v2 v2.4.0
	gopkg.in/yaml.v3 v3.0.1
	gotest.tools/v3 v3.5.1
	k8s.io/api v0.28.12
	k8s.io/apiextensions-apiserver v0.28.12
	k8s.io/apimachinery v0.28.12
	k8s.io/apiserver v0.28.12
	k8s.io/client-go v0.28.12
	k8s.io/code-generator v0.28.12
	k8s.io/component-base v0.28.12
	k8s.io/component-helpers v0.28.12
	k8s.io/cri-api v0.28.12
	k8s.io/klog/v2 v2.110.1
	k8s.io/kubectl v0.0.0
	k8s.io/kubelet v0.28.12
	k8s.io/kubernetes v1.28.12
	k8s.io/utils v0.0.0-20230406110748-d93618cff8a2
	sigs.k8s.io/controller-tools v0.9.2
	sigs.k8s.io/yaml v1.3.0
)

require (
	github.com/lufia/plan9stats v0.0.0-20220517141722-cf486979b281 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.4 // indirect
	github.com/power-devops/perfstat v0.0.0-20210106213030-5aafc221ea8c // indirect
	sigs.k8s.io/json v0.0.0-20221116044647-bc3834ca7abd // indirect
)

require (
	cloud.google.com/go/compute v1.23.3 // indirect
	cloud.google.com/go/compute/metadata v0.2.3 // indirect
	github.com/Azure/go-ansiterm v0.0.0-20210617225240-d185dfc1b5a1 // indirect
	github.com/JeffAshton/win_pdh v0.0.0-20161109143554-76bb4ee9f0ab // indirect
	github.com/Knetic/govaluate v3.0.1-0.20171022003610-9aa49832a739+incompatible // indirect
	github.com/MakeNowJust/heredoc v1.0.0 // indirect
	github.com/Microsoft/go-winio v0.6.1 // indirect
	github.com/NYTimes/gziphandler v1.1.1 // indirect
	github.com/antlr/antlr4/runtime/Go/antlr/v4 v4.0.0-20230305170008-8188dc5388df // indirect
	github.com/apparentlymart/go-cidr v1.1.0 // indirect
	github.com/asaskevich/govalidator v0.0.0-20190424111038-f61b66f89f4a // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/blang/semver/v4 v4.0.0 // indirect
	github.com/bytedance/sonic v1.9.1 // indirect
	github.com/cenkalti/backoff/v4 v4.2.1 // indirect
	github.com/census-instrumentation/opencensus-proto v0.4.1 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/chai2010/gettext-go v1.0.2 // indirect
	github.com/checkpoint-restore/go-criu/v5 v5.3.0 // indirect
	github.com/chenzhuoyu/base64x v0.0.0-20221115062448-fe3a3abad311 // indirect
	github.com/cilium/ebpf v0.11.0 // indirect
	github.com/cncf/xds/go v0.0.0-20231109132714-523115ebc101 // indirect
	github.com/containerd/console v1.0.3 // indirect
	github.com/containerd/stargz-snapshotter/estargz v0.14.3 // indirect
	github.com/containerd/ttrpc v1.2.2 // indirect
	github.com/coreos/go-semver v0.3.1 // indirect
	github.com/coreos/go-systemd/v22 v22.5.0 // indirect
	github.com/cyphar/filepath-securejoin v0.2.4 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/daviddengcn/go-colortext v1.0.0 // indirect
	github.com/distribution/reference v0.6.0 // indirect
	github.com/dnstap/golang-dnstap v0.4.0 // indirect
	github.com/docker/cli v27.1.1+incompatible // indirect
	github.com/docker/docker v27.1.1+incompatible // indirect
	github.com/docker/docker-credential-helpers v0.7.0 // indirect
	github.com/emicklei/go-restful/v3 v3.11.0 // indirect
	github.com/envoyproxy/protoc-gen-validate v1.0.2 // indirect
	github.com/euank/go-kmsg-parser v2.0.0+incompatible // indirect
	github.com/evanphx/json-patch v5.6.0+incompatible // indirect
	github.com/exponent-io/jsonpath v0.0.0-20151013193312-d6023ce2651d // indirect
	github.com/farsightsec/golang-framestream v0.3.0 // indirect
	github.com/fatih/camelcase v1.0.0 // indirect
	github.com/fatih/color v1.13.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/flynn/go-shlex v0.0.0-20150515145356-3f9db97f8568 // indirect
	github.com/fvbommel/sortorder v1.1.0 // indirect
	github.com/gabriel-vasile/mimetype v1.4.2 // indirect
	github.com/gin-contrib/sse v0.1.0 // indirect
	github.com/go-errors/errors v1.4.2 // indirect
	github.com/go-logr/logr v1.4.1 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/go-openapi/jsonpointer v0.19.6 // indirect
	github.com/go-openapi/jsonreference v0.20.2 // indirect
	github.com/go-openapi/swag v0.22.3 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-playground/validator/v10 v10.14.0 // indirect
	github.com/go-task/slim-sprig v0.0.0-20230315185526-52ccab3ef572 // indirect
	github.com/gobuffalo/flect v0.3.0 // indirect
	github.com/goccy/go-json v0.10.2 // indirect
	github.com/godbus/dbus/v5 v5.1.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/google/btree v1.1.2 // indirect
	github.com/google/cadvisor v0.47.3 // indirect
	github.com/google/cel-go v0.16.1 // indirect
	github.com/google/flatbuffers v23.1.21+incompatible // indirect
	github.com/google/gnostic-models v0.6.8 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/pprof v0.0.0-20230817174616-7a8ec2ada47b // indirect
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510 // indirect
	github.com/gregjones/httpcache v0.0.0-20180305231024-9cad4c3443a7 // indirect
	github.com/grpc-ecosystem/go-grpc-middleware v1.4.0 // indirect
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.18.0 // indirect
	github.com/grpc-ecosystem/grpc-opentracing v0.0.0-20180507213350-8e809c8a8645 // indirect
	github.com/h2non/parth v0.0.0-20190131123155-b4df798d6542 // indirect
	github.com/huandu/xstrings v1.4.0 // indirect
	github.com/imdario/mergo v0.3.12 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/infobloxopen/go-trees v0.0.0-20200715205103-96a057b8dfb9 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/jonboulle/clockwork v0.2.2 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/karrick/godirwalk v1.17.0 // indirect
	github.com/klauspost/compress v1.17.6 // indirect
	github.com/klauspost/cpuid/v2 v2.2.6 // indirect
	github.com/leodido/go-urn v1.2.4 // indirect
	github.com/liggitt/tabwriter v0.0.0-20181228230101-89fcab3d43de // indirect
	github.com/lithammer/dedent v1.1.0 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.19 // indirect
	github.com/mattn/go-runewidth v0.0.9 // indirect
	github.com/matttproud/golang_protobuf_extensions/v2 v2.0.0 // indirect
	github.com/mistifyio/go-zfs v2.1.2-0.20190413222219-f784269be439+incompatible // indirect
	github.com/mitchellh/go-wordwrap v1.0.1 // indirect
	github.com/moby/docker-image-spec v1.3.1 // indirect
	github.com/moby/spdystream v0.2.0 // indirect
	github.com/moby/term v0.0.0-20221205130635-1aeaba878587 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/monochromegane/go-gitignore v0.0.0-20200626010858-205db1a8cc00 // indirect
	github.com/mrunalp/fileutils v0.5.1 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/mxk/go-flowrate v0.0.0-20140419014527-cca7078d478f // indirect
	github.com/onsi/ginkgo/v2 v2.11.0 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/runc v1.1.14 // indirect
	github.com/opentracing/opentracing-go v1.2.0 // indirect
	github.com/pelletier/go-toml/v2 v2.1.1 // indirect
	github.com/peterbourgon/diskv v2.0.1+incompatible // indirect
	github.com/pierrec/lz4/v4 v4.1.21 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/prometheus/client_model v0.5.0 // indirect
	github.com/prometheus/common v0.48.0 // indirect
	github.com/prometheus/procfs v0.12.0 // indirect
	github.com/quic-go/quic-go v0.40.1 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/ryszard/goskiplist v0.0.0-20150312221310-2dfbae5fcf46 // indirect
	github.com/seccomp/libseccomp-golang v0.10.0 // indirect
	github.com/shabbyrobe/gocovmerge v0.0.0-20190829150210-3e036491d500 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	github.com/stoewer/go-strcase v1.2.0 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	github.com/syndtr/gocapability v0.0.0-20200815063812-42c35b437635 // indirect
	github.com/tidwall/gjson v1.14.4 // indirect
	github.com/tidwall/pretty v1.2.0 // indirect
	github.com/tklauser/go-sysconf v0.3.10 // indirect
	github.com/tklauser/numcpus v0.4.0 // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	github.com/ugorji/go/codec v1.2.11 // indirect
	github.com/vbatts/tar-split v0.11.3 // indirect
	github.com/vishvananda/netlink v1.2.1-beta.2 // indirect
	github.com/vishvananda/netns v0.0.4 // indirect
	github.com/xlab/treeprint v1.2.0 // indirect
	github.com/yusufpapurcu/wmi v1.2.2 // indirect
	github.com/zeebo/xxh3 v1.0.2 // indirect
	go.etcd.io/bbolt v1.3.8 // indirect
	go.etcd.io/etcd/api/v3 v3.5.12 // indirect
	go.etcd.io/etcd/client/pkg/v3 v3.5.11 // indirect
	go.etcd.io/etcd/client/v3 v3.5.11 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.46.1 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.47.0 // indirect
	go.opentelemetry.io/otel v1.22.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.19.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.19.0 // indirect
	go.opentelemetry.io/otel/metric v1.22.0 // indirect
	go.opentelemetry.io/otel/sdk v1.21.0 // indirect
	go.opentelemetry.io/otel/trace v1.22.0 // indirect
	go.opentelemetry.io/proto/otlp v1.0.0 // indirect
	go.starlark.net v0.0.0-20230525235612-a134d8f9ddca // indirect
	go.uber.org/mock v0.4.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/arch v0.3.0 // indirect
	golang.org/x/crypto v0.31.0 // indirect
	golang.org/x/exp v0.0.0-20240506185415-9bf2ced13842 // indirect
	golang.org/x/mod v0.17.0 // indirect
	golang.org/x/oauth2 v0.16.0 // indirect
	golang.org/x/sync v0.10.0 // indirect
	golang.org/x/term v0.27.0 // indirect
	golang.org/x/text v0.21.0 // indirect
	golang.org/x/time v0.5.0 // indirect
	golang.org/x/tools v0.21.1-0.20240508182429-e35e4ccd0d2d // indirect
	golang.org/x/xerrors v0.0.0-20220907171357-04be3eba64a2 // indirect
	google.golang.org/appengine v1.6.8 // indirect
	google.golang.org/genproto v0.0.0-20240102182953-50ed04b92917 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20240102182953-50ed04b92917 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7 // indirect
	k8s.io/cli-runtime v0.28.12 // indirect
	k8s.io/cloud-provider v0.0.0 // indirect
	k8s.io/controller-manager v0.28.12 // indirect
	k8s.io/csi-translation-lib v0.0.0 // indirect
	k8s.io/dynamic-resource-allocation v0.0.0 // indirect
	k8s.io/gengo v0.0.0-20220902162205-c0856e24416d // indirect
	k8s.io/kms v0.28.12 // indirect
	k8s.io/kube-openapi v0.0.0-20230717233707-2695361300d9 // indirect
	k8s.io/kube-scheduler v0.0.0 // indirect
	k8s.io/metrics v0.28.12 // indirect
	k8s.io/mount-utils v0.0.0 // indirect
	sigs.k8s.io/apiserver-network-proxy/konnectivity-client v0.1.2 // indirect
	sigs.k8s.io/kustomize/api v0.13.5-0.20230601165947-6ce0bf390ce3 // indirect
	sigs.k8s.io/kustomize/kustomize/v5 v5.0.4-0.20230601165947-6ce0bf390ce3 // indirect
	sigs.k8s.io/kustomize/kyaml v0.14.3-0.20230601165947-6ce0bf390ce3 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.2.3 // indirect
	stathat.com/c/consistent v1.0.0 // indirect
)

replace (
	github.com/agiledragon/gomonkey => github.com/agiledragon/gomonkey/v2 v2.2.0
	github.com/prometheus/client_golang => github.com/prometheus/client_golang v1.17.0
	github.com/prometheus/common => github.com/prometheus/common v0.45.0
	github.com/quic-go/quic-go => github.com/quic-go/quic-go v0.48.2
	k8s.io/api => k8s.io/api v0.28.12
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.28.12
	k8s.io/apimachinery => k8s.io/apimachinery v0.28.12
	k8s.io/apiserver => k8s.io/apiserver v0.28.12
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.28.12
	k8s.io/client-go => k8s.io/client-go v0.28.12
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.28.12
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.28.12
	k8s.io/code-generator => k8s.io/code-generator v0.28.12
	k8s.io/component-base => k8s.io/component-base v0.28.12
	k8s.io/component-helpers => k8s.io/component-helpers v0.28.12
	k8s.io/controller-manager => k8s.io/controller-manager v0.28.12
	k8s.io/cri-api => k8s.io/cri-api v0.28.12
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.28.12
	k8s.io/dynamic-resource-allocation => k8s.io/dynamic-resource-allocation v0.28.12
	k8s.io/endpointslice => k8s.io/endpointslice v0.28.12
	k8s.io/kms => k8s.io/kms v0.28.12
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.28.12
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.28.12
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.28.12
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.28.12
	k8s.io/kubectl => k8s.io/kubectl v0.28.12
	k8s.io/kubelet => k8s.io/kubelet v0.28.12
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.28.12
	k8s.io/metrics => k8s.io/metrics v0.28.12
	k8s.io/mount-utils => k8s.io/mount-utils v0.28.12
	k8s.io/pod-security-admission => k8s.io/pod-security-admission v0.28.12
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.28.12
)
