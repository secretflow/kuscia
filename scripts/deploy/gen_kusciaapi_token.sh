#!/bin/bash

set -e
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'
DOMAIN_ID=$1
DOMAIN_KEY_DATA=$2

if [ "$#" -ne 2 ]; then
    echo -e "${RED}Please run the script in this format: $0 \${domainID} \${domainKeyData}${NC}"
    exit 1
fi

echo $DOMAIN_KEY_DATA | base64 -d > /tmp/domain_key
echo -n $DOMAIN_ID | openssl dgst -sha256 -sign /tmp/domain_key -out /tmp/signture_file
cat /tmp/signture_file | base64  > /tmp/signture_file_base64
TOKEN=$(head -c 32 /tmp/signture_file_base64)
echo -e "${GREEN}$TOKEN${NC}"
rm -rf /tmp/domain_key /tmp/signture_file /tmp/signture_file_base64