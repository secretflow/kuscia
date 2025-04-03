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
# If new independent parameters are added, please set a reasonable shift
shift 3
interconn_protocol=kuscia
auth_type=MTLS
token=
insecure="false"
transit_domain=
need_interop="true"

function join_to_host() {
  local host=${host_endpoint}
  local port=80
  local path="/"

  istls="false"
  if [[ "${host_endpoint}" == *"://"* ]]; then
    host_and_path=${host_endpoint##*://}
    host=${host_and_path%%/*}
    if [[ $host_and_path == *"/"* ]]; then
      path=/${host_and_path#*/}
    fi
    protocol=${host_endpoint%%://*}
    if [ "${protocol}" == "https" ]; then
      istls="true"
    fi
  fi

  if [[ "${host}" == *":"* ]]; then
    port=${host##*:}
    host=${host%%:*}
  elif [ "$istls" = "true" ]; then
    port=443
  fi

  local domain_route_template
  if [[ ${transit_domain} == "" ]]; then
    domain_route_template=$(sed "s/{{.SELF_DOMAIN}}/${self_domain_id}/g;
      s/{{.SRC_DOMAIN}}/${self_domain_id}/g;
      s/{{.DEST_DOMAIN}}/${host_domain_id}/g;
      s/{{.INTERCONN_PROTOCOL}}/${interconn_protocol}/g;
      s/{{.HOST}}/${host}/g;
      s/{{.PORT}}/${port}/g;
      s@{{.PATH}}@${path}@g;
      s/{{.ISTLS}}/${istls}/g;
      s/{{.TOKEN}}/${token}/g" \
      <"${ROOT}/scripts/templates/cluster_domain_route.token.yaml")
  else
    domain_route_template=$(sed "s/{{.SELF_DOMAIN}}/${self_domain_id}/g;
      s/{{.SRC_DOMAIN}}/${self_domain_id}/g;
      s/{{.DEST_DOMAIN}}/${host_domain_id}/g;
      s/{{.INTERCONN_PROTOCOL}}/${interconn_protocol}/g;
      s/{{.HOST}}/${host}/g;
      s/{{.PORT}}/${port}/g;
      s@{{.PATH}}@${path}@g;
      s/{{.ISTLS}}/${istls}/g;
      s/{{.TRANSIT_DOMAIN}}/${transit_domain}/g;
      s/{{.TOKEN}}/${token}/g" \
      <"${ROOT}/scripts/templates/cluster_domain_route.token.transit.yaml")
  fi
  echo "${domain_route_template}" | kubectl apply -f -
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
    -t    Token used for authentication.
    -x    transit domain name."
}

while getopts 'p:a:t:x:k:i:h' option; do
  case "$option" in
  p)
    interconn_protocol=$OPTARG
    ;;
  a)
    auth_type=$OPTARG
    [[ "$auth_type" == "MTLS" || "$auth_type" == "Token" || "$auth_type" == "None" ]] && continue
    printf "illegal value for -%s\n" "$option" >&2
    usage
    exit
    ;;
  t)
    token=$OPTARG
    ;;
  x)
    transit_domain=$OPTARG
    ;;
  k)
    insecure="true"
    ;;
  i)
    need_interop=$OPTARG
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

if [[ $auth_type == "Token" && $token == "" ]]; then
  printf "missing argument -t {{token}}\n" >&2
  usage
  exit 1
fi

join_to_host
