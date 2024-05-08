# OPENSOURCE-CLEANUP DELETE_FILE
# latest image version: reg.docker.alibaba-inc.com/nueva-stack/alios7u2-py-base:0.20

FROM openanolis/anolisos:8.4-x86_64 AS build

RUN yum groupinstall -y "Development Tools" && yum install -y wget fuse fuse-devel fuse3 fuse3-devel
RUN wget -O unionfs-fuse.tgz https://github.com/rpodgorny/unionfs-fuse/archive/refs/tags/v2.1.tar.gz && \
    tar xf unionfs-fuse.tgz && cd unionfs-fuse-2.1/ && make && make install


FROM reg.docker.alibaba-inc.com/alipay/7u2-common:202202.0T

LABEL maintainer="zhuangzhuangyan.yz@digital-engine.com"

ENV TZ=Asia/Shanghai

USER root
WORKDIR /home/admin

COPY --from=build /usr/local/bin/unionfs* /usr/local/bin/

# update glibc to 2.30
# https://yuque.antfin-inc.com/baseos/manual/gcc92

RUN yum clean all && yum -y install glibc-devel && \
    yum -y update util-linux bash-completion gnupg2 python perl systemd && \
    yum -y install alios7u-2_30-gcc-9-repo.noarch && \
    yum -y install libdb binutils && \
    yum -y install glibc glibc-langpack-en && \
    yum -y install glibc-devel libstdc++-devel libstdc++-static libstdc++ libasan && \
    yum clean all

RUN ln -snf /usr/share/zoneinfo/$TZ /etc/localtime && echo $TZ > /etc/timezone

RUN yum install -y wget && \
    wget -O /etc/yum.repos.d/CentOS-Base.repo http://mirrors.aliyun.com/repo/Centos-7.repo && \
    wget -O /etc/yum.repos.d/openresty.repo https://openresty.org/package/centos/openresty.repo && \
    yum install --setopt=skip_missing_names_on_install=False -y \
        net-tools-2.0 epel-release tar iproute sudo logrotate \
        tcpdump less rsync tree which \
        zlib-devel libffi-devel openssl-devel fuse fuse-devel fuse3 fuse3-devel fuse-overlayfs perl perl-Digest-MD5 python3 java openresty openresty-opm  && \
    yum clean all

RUN opm get knyar/nginx-lua-prometheus ledgetech/lua-resty-http cdbattags/lua-resty-jwt && \
    wget -O /usr/bin/ossutil64 http://gosspublic.alicdn.com/ossutil/1.6.0/ossutil64 && \
    chmod 0755 /usr/bin/ossutil64

RUN pushd /usr/bin/ && rm -rf python && ln -s python3 python && popd && \
    sed -i 's/\/usr\/bin\/python/\/usr\/bin\/python2.7/g' /usr/bin/yum /usr/libexec/urlgrabber-ext-down && \
    yum install -y python3-devel

RUN pip3 install --upgrade pip && \
    pip3 install cryptography==3.3.2 paramiko scp

RUN yum install -y libxml2-devel

RUN pushd /tmp && \
    ln -s /usr/lib64/liblua-5.1.so /usr/lib64/liblua.so && \
    wget https://mpc-antbase.oss-cn-hangzhou.aliyuncs.com/release/linux/acjail/t-aliyun-acjail-2.0.1.an8.x86_64.rpm && \
    rpm -ivh t-aliyun-acjail-2.0.1.an8.x86_64.rpm && \
    sed -i '/blacklist_ip = {/c\blacklist_ip = blacklist_ip or {}' /usr/local/aliyun_acjail/config.lua && \
    rm -rfv t-aliyun-acjail-2.0.1.an8.x86_64.rpm && popd

RUN pushd /tmp && wget https://bindfs.org/downloads/bindfs-1.15.1.tar.gz --no-check-certificate && \
    tar xf bindfs-1.15.1.tar.gz && cd bindfs-1.15.1 && ./configure && make && make install && \
    cd .. && rm -rfv bindfs* && popd

RUN yum -y remove openssh && rm -rf /etc/runit/sshd && \
    yum -y remove telnet