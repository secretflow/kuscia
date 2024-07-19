#!/bin/bash

# Copyright 2022 Ant Group Co., Ltd.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -e

nft_valid=false
legacy_valid=false

if xtables-legacy-multi iptables -L > /dev/null 2>&1 ; then
  legacy_valid=true
fi

if xtables-nft-multi iptables -L > /dev/null 2>&1 ; then
  nft_valid=true
fi

if [[ $legacy_valid == true && $nft_valid != true ]]; then
  export IPTABLES_MODE=legacy
  iptables -V > /dev/null
elif [[ $legacy_valid != true && $nft_valid == true ]]; then
  export IPTABLES_MODE=nft
  iptables -V > /dev/null
fi