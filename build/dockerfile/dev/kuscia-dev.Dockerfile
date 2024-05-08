# latest image version: reg.docker.alibaba-inc.com/secretflow/kuscia-dev:0.6

FROM reg.docker.alibaba-inc.com/nueva-stack/dev-base:0.3

LABEL maintainer="caochen.cao@antgroup.com"

ENV TZ=Asia/Shanghai

RUN yum update -y \
    && yum clean all

RUN ln -snf /usr/share/zoneinfo/$TZ /etc/localtime && echo $TZ > /etc/timezone && \
    yum install -y wget sudo less epel-release net-tools iproute git jq tree unzip gcc perl perl-Digest-MD5 git-lfs which && \
    yum clean all && \
    wget -O /usr/bin/ossutil64 http://gosspublic.alicdn.com/ossutil/1.6.0/ossutil64 && \
    chmod 0755 /usr/bin/ossutil64 && \
    wget https://github.com/protocolbuffers/protobuf/releases/download/v21.8/protoc-21.8-linux-x86_64.zip && \
    unzip protoc-21.8-linux-x86_64.zip -d /usr/local && chgrp -R users /usr/local/include/google /usr/local/bin/protoc && cd ../ && \
    wget http://mpc-antbase.oss-cn-hangzhou.aliyuncs.com/influxdb/influxd -O /usr/local/bin/influxd && chmod 755 /usr/local/bin/influxd  && \
    wget https://mpc-antbase.oss-cn-hangzhou.aliyuncs.com/mysql-8.0.28-lite/mysql.tar.gz  && tar -xvf mysql.tar.gz -C  /usr/local && chmod 755 /usr/local/bin/mysqld

RUN wget https://golang.google.cn/dl/go1.19.10.linux-amd64.tar.gz -O - | tar -xz -C /usr/local && \
    echo export GOPROXY=https://goproxy.cn,direct >> ~/.bashrc && \
    echo export GO111MODULE=on >> ~/.bashrc && \
    echo export GOPATH=$HOME/gopath >> ~/.bashrc && \
    echo export PATH=$PATH:/usr/local/go/bin:'$GOPATH'/bin >> ~/.bashrc && \
    echo export GOPRIVATE=gitlab.alipay-inc.com >> ~/.bashrc && \
    source ~/.bashrc && \
    ssh-keyscan -t rsa gitlab.alipay-inc.com >> ~/.ssh/known_hosts && \
    git config --global url."git@gitlab.alipay-inc.com:".insteadOf "http://gitlab.alipay-inc.com/" && \
    go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.28.1 && \
    go install github.com/t-yuki/gocover-cobertura@latest && \
    go install github.com/jstemmer/go-junit-report/v2@latest && \
    go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.50.1