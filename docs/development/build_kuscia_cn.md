# 构建命令

## 开发环境搭建

### 开发环境依赖

* Golang: 1.19.4+
* Protoc
* Docker

### 安装 Golang

[Golang 安装教程](https://go.dev/doc/install)

### 安装 Protoc

[Protoc 安装教程](https://github.com/protocolbuffers/protobuf)

### 安装 Docker

[Docker Desktop 安装教程](https://docs.docker.com/desktop/)

[Docker Engine 安装教程](https://docs.docker.com/engine/install/)

## 构建 Kuscia

Kuscia 提供了 Makefile 来构建镜像，你可以通过`make help`命令查看命令帮助，其中 Build 部分提供了构建能力：

```shell
    Usage:
        make <target>

    General
        help             Display this help.

    Development
        manifests        Generate CustomResourceDefinition objects.
        generate         Generate all code that Kuscia needs.
        fmt              Run go fmt against code.
        vet              Run go vet against code.
        test             Run tests.

    Build
        build            Build kuscia binary.
        docs             Build docs.
        image            Build docker image with the manager.
```

### 构建可执行文件

在 Kuscia 项目根目录下：

执行`make build`命令，该命令将会构建出 Kuscia 的可执行文件，构建产物会生成在 ./build/ 目录下。

### 构建 Kuscia-Envoy Image

Kuscia 镜像的构建依赖 Kuscia-Envoy 镜像，Kuscia 提供默认的 [Kuscia-Envoy 镜像](https://hub.docker.com/r/secretflow/kuscia-envoy/tags)。如果你选择使用默认的 Kuscia-Envoy 镜像，那么你可以跳过这一步。

如果你选择自行构建 Kuscia-Envoy 镜像，请在 [Kuscia-Envoy](https://github.com/secretflow/kuscia-envoy) 项目的根目录下执行 `make image` 命令。 而后你可以用 `docker images | grep kuscia-envoy` 来查看 
构建产出的 Kuscia-Envoy 镜像名称。

### 构建 Kuscia Image

在 Kuscia 项目根目录下：

执行`make image`命令，该命令将会使用 Docker 命令构建出 Kuscia 镜像。目前 Kuscia 暂时仅支持构建 linux/amd64 的 Anolis 镜像。

如果你想依赖指定的 Kuscia-Envoy 镜像构建 Kuscia 镜像，你可以通过 `make image KUSCIA_ENVOY_IMAGE=${KUSCIA_ENVOY_IMAGE}` 来指定依赖镜像的名称。

如果你使用的是 arm 架构的 macOS，请修改`build/dockerfile/kuscia-anolis.Dockerfile`文件，将`FROM openanolis/anolisos:8.8`修改为`FROM openanolis/anolisos:8.4-x86_64`，然后再执行`make build`命令。

### 编译文档

在 Kuscia 项目根目录下：

执行`make docs`命令，该命令会生成 Kuscia 文档，生成的文档会放在 `docs/_build/html` 目录，用浏览器打开 `docs/_build/html/index.html` 就可以查看文档。

该命令依赖于 python 环境，并且已经安装了 pip 工具；编译文档前请提前安装，否则会执行错误。