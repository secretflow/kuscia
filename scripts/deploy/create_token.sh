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

usage="$(basename "$0") DOMAIN_ID"

DOMAIN_ID=$1
if [[ ${DOMAIN_ID} == "" ]]; then
  echo "missing argument: $usage"
  exit 1
fi

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)

# each master create a clusterrole
kubectl apply -f ${ROOT}/etc/conf/domain-cluster-res.yaml

# each domain create a role
DOMAIN_NAMESPACE_RES_TEMPLATE=$(sed "s/{{.DOMAIN_ID}}/${DOMAIN_ID}/g" < "${ROOT}/etc/conf/domain-namespace-res.yaml")
echo "${DOMAIN_NAMESPACE_RES_TEMPLATE}" | kubectl apply -f -

#ech domain create a serviceaccount
if ! kubectl get serviceaccount $DOMAIN_ID -n $DOMAIN_ID 2>&1 ; then
  kubectl create serviceaccount $DOMAIN_ID -n $DOMAIN_ID
fi

#binding role and clusterrole to domain's serviceaccount
if ! kubectl get rolebinding $DOMAIN_ID -n $DOMAIN_ID 2>&1 ; then
  kubectl create rolebinding $DOMAIN_ID --role domain-namespace-res --serviceaccount ${DOMAIN_ID}:${DOMAIN_ID} -n $DOMAIN_ID
fi

if ! kubectl get clusterrolebinding $DOMAIN_ID 2>&1 ; then
  kubectl create clusterrolebinding $DOMAIN_ID --clusterrole domain-cluster-res --serviceaccount ${DOMAIN_ID}:${DOMAIN_ID}
fi

#create token
kubectl create token $DOMAIN_ID --duration 87600h -n $DOMAIN_ID

# test
# curl https://127.0.0.1:6443/api/v1/namespaces/${DOMAIN_ID}/pods -H'Authorization: Bearer ${TOKEN}'
