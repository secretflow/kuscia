#!/bin/bash
#
# Copyright 2023 Ant Group Co., Ltd.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#   http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

usage="$(basename "$0") PARTY_ID"
PARTY_ID=$1
OTHER_PARTY_ID=$2
OTHER_PARTY_IP=$3

if [[ ${PARTY_ID} == "" ]]; then
    echo "missing argument: $usage"
    exit 1
fi

export LANG=en_US.UTF-8

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)
echo "${ROOT}"

FATA_DIR=${ROOT}/fate_cluster_install_1.11.1_release/allInone

function init_deploy_conf() {
    . ${FATA_DIR}/conf/setup.conf

    mkdir -p "${FATA_DIR}/"{logs,temp}
    mkdir -p ${FATA_DIR}/../init/conf

    local variables="roles_num=${#roles[@]} host_id=${host_id} guest_id=${guest_id} host_ip=${host_ip} guest_ip=${guest_ip} host_mysql_ip=${host_mysql_ip} pbase=${pbase} pname=${pname} lbase=${lbase} version=${version} user=app group=apps mysql_port=${mysql_port}  host_mysql_pass=${host_mysql_pass} eggroll_dbname=${eggroll_dbname} fate_flow_dbname=${fate_flow_dbname} mysql_admin_pass=${mysql_admin_pass} redis_pass=${redis_pass} rollsite_port=${rollsite_port} clustermanager_port=${clustermanager_port} nodemanager_port=${nodemanager_port} fateflow_grpc_port=${fateflow_grpc_port} fateflow_http_port=${fateflow_http_port} fateboard_port=${fateboard_port}"
    local tpl=$( cat ${FATA_DIR}/templates/init-setup.conf )
    printf "$variables\ncat << EOF\n$tpl\nEOF" | sh > ${FATA_DIR}/../init/conf/setup.conf
    sh ${FATA_DIR}/../init/init.sh

    cd ${FATA_DIR}/../
    mkdir -p ${ROOT}/install;
}

function deploy_host() {
    local variables="pbase=${ROOT} pname=$pname role=host"
    local tpl=$( cat ${FATA_DIR}/templates/do.sh.jinja )
    printf "$variables\ncat << EOF\n$tpl\nEOF" | sh > ${FATA_DIR}/temp/do-host.sh
    cat ${FATA_DIR}/temp/do-host.sh | sh
    echo "deploy host fate ${host_ip} over" >> ${FATA_DIR}/logs/deploy.log
    date >> ${FATA_DIR}/logs/time 
}

function deploy_mysql() {
    local variables="pbase=${ROOT} role=host"
    local tpl=$( cat ${FATA_DIR}/templates/do-mysql.sh.jinja )
    printf "$variables\ncat << EOF\n$tpl\nEOF" | sh > ${FATA_DIR}/temp/do-mysql-host.sh
    cat ${FATA_DIR}/temp/do-mysql-host.sh | sh
    echo "deploy host mysql ${host_mysql_ip} over" >> ${FATA_DIR}/logs/deploy.log
    date >> ${FATA_DIR}/logs/time
}

function upload_data() { 
    local done_log_file=${FATA_DIR}/logs/deploy.log
    while [[ ! -f $done_log_file ]]
    do
        echo "wait to upload data, sleep 10"
        sleep 10
    done

    local done_log=$( cat $done_log_file | grep fate )
    while [[ -z "$done_log" ]]
    do
        done_log=$( cat $done_log_file | grep fate )
        echo "wait to upload data, sleep 10"
        sleep 10
    done

    sh ${ROOT}/scripts/upload_fate_data.sh $pname
}

pushd ${FATA_DIR} || exit

# config fate info and deploy cluster
# doc: https://github.com/FederatedAI/FATE/blob/master/deploy/cluster-deploy/doc/fate_on_eggroll/fate-allinone_deployment_guide.zh.md
cat ${ROOT}/etc/setup.conf > ./conf/setup.conf
HOST_IP=$(hostname -I)
sed -i "s/{{.HOST_IP}}/${HOST_IP}/g;s/{{.PARTY_ID}}/${PARTY_ID}/g;s/{{.OTHER_PARTY_ID}}/${OTHER_PARTY_ID}/g;s/{{.OTHER_PARTY_IP}}/${OTHER_PARTY_IP}/g" ./conf/setup.conf
if [[ ${OTHER_PARTY_IP} == "" ]]; then
    sed -i "s/{{.ROLES}}/( "host" )/g" ./conf/setup.conf
else
    sed -i "s/{{.ROLES}}/( "host" "guest" )/g" ./conf/setup.conf
fi

init_deploy_conf

# deploy mysql
if (( ${#dbmodules[@]} != 0 )); then
    for module in ${dbmodules[*]}
    do
    mv ${module}-install ${ROOT}/install
    done
    deploy_mysql > ${FATA_DIR}/logs/deploy-mysql-host.log 2>&1 &
fi

# deploy host
if (( ${#basemodules[@]} != 0 )); then
    for module in ${basemodules[*]}
    do
    mv ${module}-install ${ROOT}/install
    done
    deploy_host > ${FATA_DIR}/logs/deploy-host.log 2>&1 &
fi

upload_data

popd || exit
