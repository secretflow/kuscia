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

set -e

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)

self_domain_id=$1
host_domain_id=$2
host_endpoint=$3
interconn_protocol=kuscia
auth_type=MTLS
token=
insecure="false"

function join_to_host() {
  local host=${host_endpoint}
  local port=80
  local tls_ca
  local src_cert
  local src_key

  if [[ "${host_endpoint}" == *":"* ]]; then
    host=${host_endpoint%%:*}
    port=${host_endpoint##*:}
  fi

  pushd $ROOT/etc/certs >/dev/null || exit

  if [[ ${insecure} != "true" ]]; then
    local tls_ca_file=${host_domain_id}.host.ca.crt
    tls_ca=$(base64 ${tls_ca_file} | tr -d "\n")
  fi

  if [[ ${auth_type} == "MTLS" ]]; then
    local src_cert_file=domain-2-${host_domain_id}.crt
    src_cert=$(base64 ${src_cert_file} | tr -d "\n")
    local src_key_file=domain.key
    src_key=$(base64 ${src_key_file} | tr -d "\n")
  fi

  popd >/dev/null || exit

  local domain_route_template
  domain_route_template=$(sed "s/{{.SELF_DOMAIN}}/${self_domain_id}/g;
    s/{{.SRC_DOMAIN}}/${self_domain_id}/g;
    s/{{.DEST_DOMAIN}}/${host_domain_id}/g;
    s/{{.INTERCONN_PROTOCOL}}/${interconn_protocol}/g;
    s/{{.HOST}}/${host}/g;
    s/{{.PORT}}/${port}/g;
    s/{{.TLS_CA}}/${tls_ca}/g;
    s/{{.SRC_CERT}}/${src_cert}/g;
    s/{{.SRC_KEY}}/${src_key}/g" \
    < "${ROOT}/scripts/templates/domain_route.mtls.yaml")
  echo "${domain_route_template}" | kubectl apply -f -

  if [[ ${interconn_protocol} == "kuscia" ]]; then
     INTEROP_CONFIG_TEMPLATE=$(sed "s/{{.MEMBER_DOMAIN_ID}}/${self_domain_id}/g;
       s/{{.HOST_DOMAIN_ID}}/${host_domain_id}/g" \
       < "${ROOT}/scripts/templates/interop_config.yaml")
     echo "${INTEROP_CONFIG_TEMPLATE}" | kubectl apply -f -
  fi
}

usage() {
  echo "$(basename "$0") SELF_DOMAIN_ID HOST_DOMAIN_ID HOST_ENDPOINT [OPTIONS]

SELF_DOMAIN_ID:
    self domain ID

HOST_DOMAIN_ID:
    host domain ID

HOST_ENDPOINT:
    host endpoint (ip:port)

OPTIONS:
    -h    Show this help text.
    -k    Allow insecure server connections when using SSL.
    -p    Interconnection protocol, default is 'kuscia'.
    -a    Authentication type from this domain to the host domain, must be one of 'MTLS', 'Token' or 'None', default is 'MTLS'. If set to Token, you need to set the -t parameter.
    -t    Token used for authentication. "
}

while getopts 'p:a:t:kh' option; do
  case "$option" in
  p)
    interconn_protocol=$OPTARG
    ;;
  a)
    auth_type=$OPTARG
    [ "$auth_type" == "MTLS" -o "$auth_type" == "Token" -o "$auth_type" == "None" ] && continue
    printf "illegal value for -%s\n" "$option" >&2
    usage
    exit
    ;;
  t)
    token=$OPTARG
    ;;
  k)
    insecure="true"
    ;;
  h)
    usage
    exit
    ;;
  :)
    printf "missing argument for -%s\n" "$OPTARG" >&2
    exit 1
    ;;
  \?)
    printf "illegal option: -%s\n" "$OPTARG" >&2
    exit 1
    ;;
  esac
done
shift $((OPTIND - 1))

if [[ $auth_type == "Token" && $token == "" ]]; then
  printf "missing argument -t {{token}}\n" >&2
  usage
  exit 1
fi

join_to_host