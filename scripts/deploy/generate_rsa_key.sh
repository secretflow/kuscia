#!/bin/bash
set -e
GREEN='\033[0;32m'
NC='\033[0m'
KEY=$(openssl genrsa 2048 2>/dev/null | base64 | tr -d "\n" && echo)
echo -e "${GREEN}生成节点私钥配置:\n\n$KEY\n${NC}"