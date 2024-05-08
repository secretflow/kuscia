## 说明
本说明文档仅限内部使用，不会带到开源仓库。

## 开发环境搭建
安装 golang

```shell
# mac
wget https://go.dev/dl/go1.19.4.darwin-amd64.tar.gz
sudo tar -C /usr/local -zxvf go1.19.4.darwin-amd64.tar.gz

# linux
wget https://go.dev/dl/go1.19.4.linux-amd64.tar.gz
sudo tar -C /usr/local -zxvf go1.19.4.linux-amd64.tar.gz
```

添加环境变量
```shell
export GOPROXY="https://goproxy.cn,direct"
export GO111MODULE=on
export GOPATH="$HOME/gopath"
export PATH="$PATH:$GOPATH/bin:/usr/local/go/bin"
export GOPRIVATE="gitlab.alipay-inc.com"
```

安装 protoc
```shell
# 更多版本信息：https://github.com/protocolbuffers/protobuf/releases/tag/v21.8
# mac x86_64
PROTOC_ZIP=protoc-21.8-osx-x86_64.zip
# linux x86_64
PROTOC_ZIP=protoc-21.8-linux-x86_64.zip

curl -OL https://github.com/protocolbuffers/protobuf/releases/download/v21.8/$PROTOC_ZIP
unzip -o $PROTOC_ZIP -d /usr/local bin/protoc
unzip -o $PROTOC_ZIP -d /usr/local 'include/*'
rm -f $PROTOC_ZIP
```

git 配置
```shell
git config --global user.name "xxx"
git config --global user.email "xxx@antgroup.com"

# http/https 免密码登录（首次需要密码验证）
git config --global credential.helper store

# 通过 ssh 访问 gitlab，需要加这行配置
git config --global url."git@gitlab.alipay-inc.com:".insteadOf "http://gitlab.alipay-inc.com/"
```
ssh 配置
```shell
# 生成 ssh 公私钥
ssh-keygen -t rsa -b 4096 -f ~/.ssh/id_rsa -N ''

# 将公钥内容填至 AntCode->设置->SSH密钥
cat ~/.ssh/id_rsa.pub
```



## 构建

### 构建环境

1. 本地环境

   参考“开发环境搭建”搭建本地构建环境。

2. docker 环境

   后台启动 kuscia-dev 构建镜像：

   ```
   cd ${your kuscia direcotry}
   docker run -dit --name kuscia-dev-$(whoami) -v "$(pwd)":/home/admin/dev/ -w /home/admin/dev reg.docker.alibaba-inc.com/secretflow/kuscia-dev:0.5 /sbin/init

   docker exec -it kuscia-dev-$(whoami) bash
   cd /home/admin/dev/ && ......
   ```



### crd 构建
```shell
# 生成 crd spec
hack/generate-crds.sh

# 生成 crd client code
hack/update-codegen.sh
```



### proto 构建
```shell
# 生成 golang 代码
hack/proto-to-go.sh
```

## 代码规范

### 参考：
- https://golang.org/doc/effective_go.html
- https://github.com/golang/go/wiki/CodeReviewComments
### 补充：
todo


## 常见问题
#### 1. Goland 报错 Found several packages [syscall, main] in '/usr/local/go/src/syscall;/usr/local/go/src/syscall'
当前 kuscia 框架使用 go1.19.4，如果 Goland 版本较低会报错，需要升级 Goland 版本