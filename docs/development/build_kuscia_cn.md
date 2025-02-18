# 构建命令

## 开发环境搭建

### 开发环境依赖

* Golang: 1.19.4+
* Protoc
* Docker

### 安装 Golang

[Golang 安装教程](https://go.dev/doc/install)

### 安装 Protoc

[Protoc 安装教程](https://github.com/protocolbuffers/protobuf/blob/main/README.md#protobuf-compiler-installation)

### 安装 Docker

[Docker Desktop 安装教程](https://docs.docker.com/desktop/)

[Docker Engine 安装教程](https://docs.docker.com/engine/install/)

## 构建 Kuscia

Kuscia 提供了 Makefile 来构建镜像，您可以通过`make help`命令查看命令帮助，其中 Build 部分提供了构建能力：

```shell
Usage:
  make <target>

General
  help              Display this help.

Development
  manifests         Generate CustomResourceDefinition objects.
  generate          Generate all code that Kuscia needs.
  fmt               Run go fmt against code.
  vet               Run go vet against code.
  test              Run tests.

Build
  build             Build kuscia binary.
  docs              Build docs.
  image             Build docker image with the manager.
  integration_test  Run Integration Test
```

### 构建可执行文件

在 Kuscia 项目根目录下：

执行`make build`命令，该命令将会构建出 Kuscia 的可执行文件，构建产物会生成在 ./build/ 目录下。

### 构建 Kuscia-Envoy Image

Kuscia 镜像的构建依赖 Kuscia-Envoy 镜像，Kuscia 提供默认的 [Kuscia-Envoy 镜像](https://hub.docker.com/r/secretflow/kuscia-envoy/tags)。如果您选择使用默认的 Kuscia-Envoy 镜像，那么您可以跳过这一步。

如果您选择自行构建 Kuscia-Envoy 镜像，请在 [Kuscia-Envoy](https://github.com/secretflow/kuscia-envoy) 项目的根目录下执行 `make image` 命令。 而后您可以用 `docker images | grep kuscia-envoy` 来查看
构建产出的 Kuscia-Envoy 镜像名称。

### 构建 Kuscia Image

在 Kuscia 项目根目录下：

执行`make image`命令，该命令将会使用 Docker 命令构建出 Kuscia 镜像。

如果您想依赖指定的 Kuscia-Envoy 镜像构建 Kuscia 镜像，您可以通过 `make image KUSCIA_ENVOY_IMAGE=${KUSCIA_ENVOY_IMAGE}` 来指定依赖镜像的名称。

### 构建 Kuscia-Secretflow Image

在 kuscia/build/dockerfile 目录下：

执行`docker build -f ./kuscia-secretflow.Dockerfile .`命令会构建出 Kuscia-Secretflow 镜像。Kuscia-Secretflow 镜像在 Kuscia 镜像的基础上集成了 Secretflow 镜像。

需要注意的是，仅 `RunP` 模式下需要构建 kuscia-secretflow 镜像。

kuscia-secretflow.Dockerfile 文件里默认的 Kuscia 镜像版本是 latest，Secretflow 版本是 1.7.0b0，如果需要指定其他版本，可以使用如下命令：

此处以 Kuscia 0.14.0b0，Secretflow 1.7.0b0 版本为例

```bash
docker build  --build-arg KUSCIA_IMAGE=secretflow-registry.cn-hangzhou.cr.aliyuncs.com/secretflow/kuscia:0.14.0b0  --build-arg  SF_VERSION=1.7.0b0 -f ./kuscia-secretflow.Dockerfile .
```

### 编译文档

在 Kuscia 项目根目录下：

执行`make docs`命令，该命令会生成 Kuscia 文档，生成的文档会放在 `docs/_build/html` 目录，用浏览器打开 `docs/_build/html/index.html` 就可以查看文档。

该命令依赖于 python 环境，请确保已经安装 python 和 pip。您可以使用如下命令检查：

```shell
python --version

pip --version 或 pip3 --version
```

### 集成测试

#### 对已存在的镜像进行集成测试

Kuscia 的集成测试可以对 Kuscia 镜像进行测试，创建测试目录 test 并获取 Kuscia 集成测试脚本，集成测试脚本会下载到当前目录：

```shell
export KUSCIA_IMAGE={YOUR_KUSCIA_IMAGE}
mkdir -p test
docker pull $KUSCIA_IMAGE && docker run --rm $KUSCIA_IMAGE cat /home/kuscia/scripts/test/integration_test.sh > ./test/integration_test.sh && chmod u+x ./test/integration_test.sh
```

然后执行集成测试，第一个参数用于选择测试集合。

目前支持：\[all，center.base，p2p.base，center.example\]，不填写则默认为 all。

```shell
./test/integration_test.sh all
```

在集成脚本执行的过程中，会自动安装依赖：[grpcurl](https://github.com/fullstorydev/grpcurl/releases) 和 [jq](https://jqlang.github.io/jq/download/) 在宿主机上。

如果宿主机已经安装了并且可以通过 `PATH` 环境变量发现，则不会重复安装。 对于 `x86_64` 架构的 `maxOS` 和 `Linux` 系统，如果您没有安装，会自动安装在 `test/test_run/bin` 目录下。
对于其他系统，您需要手动安装，然后将其配置到 `PATH` 环境变量中，或者放置在 `test/test_run/bin` 目录下。

#### 使用 make 命令

如果您正在参与 Kuscia 的开发工作，您也可以通过 `make integration_test` 来进行测试，该命令会编译您当前的代码并构建 Kuscia 镜像，然后进行集成测试。

#### 新增测试用例

如果您希望为 Kuscia 新增更多的测试用例，您可以在 Kuscia 项目的 `scripts/test/suite/center` 和 `scripts/test/suite/center/p2p` 下添加您的测试用例代码。
您可以参考 `scripts/test/suite/center/basic.sh` 和 `scripts/test/suite/center/example.sh` 来编写您的测试用例。
Kuscia 使用 [shunit2](https://github.com/kward/shunit2) 作为测试框架，安装在 `scripts/test/vendor` 下，您可以使用其中的断言函数。
Kuscia 也准备了一些常用的函数，您可以在 `scripts/test/suite/core` 下找到。

下面是详细步骤：

1. 对于中心化模式，在 `scripts/test/suite/center/` 下新建您的测试用例集文件，对于 P2P 模式，在 `scripts/test/suite/p2p/` 下新建您的测试用例文件。
2. 编写您的测试用例集，确保您的测试用例集文件包含 `. ./test/vendor/shunit2`，具体请参考 [shunit2](https://github.com/kward/shunit2)。
3. 为您的测试用例集文件添加可执行权限：`chmod a+x {YOUR_TEST_SUITE_FILE}`。
4. 在 `scripts/test/integration_test.sh` 文件中注册您的测试用例集。如 `TEST_SUITES["center.example"]="./test/suite/center/example.sh"`。变量 `TEST_SUITES` 的 key 为您的测试用例集的名称。
5. 运行您的测试用例集，如上例：`./test/integration_test.sh center.example`。
