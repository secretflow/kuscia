#!/bin/bash
#
# Copyright 2025 Ant Group Co., Ltd.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#   https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

# Version consistency checker for Kuscia and SecretFlow
# This script checks markdown files for version inconsistencies and can optionally fix them

set -euo pipefail

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default values
KUSCIA_VERSION=""
SECRETFLOW_VERSION=""
CHECK_DIRS="docs scripts hack"
MODE="check"
VERBOSE=false

# Usage function
usage() {
    cat << EOF
Usage: $0 [OPTIONS]

Check and validate Kuscia and SecretFlow version consistency in markdown files.

OPTIONS:
    --kuscia-version VERSION        Expected Kuscia version (required)
    --secretflow-version VERSION    Expected SecretFlow version (required)
    --check-dirs DIRS              Directories to check (default: "docs scripts hack")
    --mode MODE                    Operation mode: check or fix (default: check)
    --verbose                      Enable verbose output
    -h, --help                     Show this help message

EXAMPLES:
    $0 --kuscia-version v0.10.0b0 --secretflow-version v1.12.0b0
    $0 --kuscia-version v0.10.0b0 --secretflow-version v1.12.0b0 --mode fix
    $0 --kuscia-version v0.10.0b0 --secretflow-version v1.12.0b0 --check-dirs "docs" --verbose

EOF
}

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1" >&2
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_verbose() {
    if [[ "$VERBOSE" == "true" ]]; then
        echo -e "${BLUE}[VERBOSE]${NC} $1"
    fi
}

# Parse command line arguments
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            --kuscia-version)
                KUSCIA_VERSION="$2"
                shift 2
                ;;
            --secretflow-version)
                SECRETFLOW_VERSION="$2"
                shift 2
                ;;
            --check-dirs)
                CHECK_DIRS="$2"
                shift 2
                ;;
            --mode)
                MODE="$2"
                shift 2
                ;;
            --verbose)
                VERBOSE=true
                shift
                ;;
            -h|--help)
                usage
                exit 0
                ;;
            *)
                log_error "Unknown option: $1"
                usage
                exit 1
                ;;
        esac
    done
}

# Validate arguments
validate_args() {
    if [[ -z "$KUSCIA_VERSION" ]]; then
        log_error "Kuscia version is required (--kuscia-version)"
        exit 1
    fi
    
    if [[ -z "$SECRETFLOW_VERSION" ]]; then
        log_error "SecretFlow version is required (--secretflow-version)"
        exit 1
    fi
    
    if [[ "$MODE" != "check" && "$MODE" != "fix" ]]; then
        log_error "Mode must be 'check' or 'fix'"
        exit 1
    fi
}

# Find markdown files in specified directories
find_files() {
    local dirs_array
    IFS=' ' read -ra dirs_array <<< "$CHECK_DIRS"
    local files=()
    
    # 专门检查根目录下的 MODULE.bazel 文件
    if [[ -f "./MODULE.bazel" ]]; then
        files+=("./MODULE.bazel")
        log_verbose "Found MODULE.bazel in current directory"
    fi
    
    # 原有的逻辑保持不变
    for dir in "${dirs_array[@]}"; do
        if [[ -d "$dir" ]]; then
            log_verbose "Searching for markdown, po, yaml, and MODULE.bazel files in: $dir"
            # Search for markdown, po, yaml files
            while IFS= read -r -d '' file; do
                files+=("$file")
            done < <(find "$dir" -type f \( -name "*.md" -o -name "*.markdown" -o -name "*.po" -o -name "*.yaml" -o -name "*.yml" \) -print0)
            
            # Search for MODULE.bazel files specifically
            while IFS= read -r -d '' file; do
                files+=("$file")
            done < <(find "$dir" -type f -name "MODULE.bazel" -print0)
        else
            log_warn "Directory not found: $dir"
        fi
    done

    if [[ ${#files[@]} -gt 0 ]]; then
        printf '%s\n' "${files[@]}"
    fi
}

# Enhanced version pattern matching
extract_kuscia_version() {
    local text="$1"
    # Match Kuscia version only when directly associated with kuscia keyword
    # Patterns: kuscia version 1.2.3, kuscia:1.2.3, kuscia-1.2.3, kuscia v1.2.3
    echo "$text" | grep -oiE "kuscia[[:space:]]*[-:]?[[:space:]]*(version[[:space:]]+)?v?[0-9]+\.[0-9]+\.[0-9]+[a-zA-Z0-9\-\.]*" | \
                   grep -oE 'v?[0-9]+\.[0-9]+\.[0-9]+[a-zA-Z0-9\-\.]*' | head -1
}

extract_secretflow_version() {
    local text="$1"
    # Match SecretFlow version only when directly associated with secretflow keyword
    # Patterns: secretflow:1.2.3, secret-flow 1.2.3, secretflow v1.2.3
    echo "$text" | grep -oiE "(secretflow|secret-flow)[[:space:]]*[-:]?[[:space:]]*(version[[:space:]]+)?v?[0-9]+\.[0-9]+\.[0-9]+[a-zA-Z0-9\-\.]*" | \
                   grep -oE 'v?[0-9]+\.[0-9]+\.[0-9]+[a-zA-Z0-9\-\.]*' | head -1
}

extract_image_version() {
    local text="$1"
    local product="$2"  # "kuscia" or "secretflow"
    
    # Match Docker image format: product/image:version or product:version
    local pattern="(^|[[:space:]])${product}[^[:space:]]*:[[:space:]]*v?[0-9]+\.[0-9]+\.[0-9]+[a-zA-Z0-9\-\.]*"
    echo "$text" | grep -oiE "$pattern" | \
                   grep -oE 'v?[0-9]+\.[0-9]+\.[0-9]+[a-zA-Z0-9\-\.]*' | head -1
}

# Check version consistency in a file
# check_file_versions() {
#     local file="$1"
#     local issues=()
#     local line_num=0
    
#     log_verbose "Checking file: $file"
    
#     while IFS= read -r line; do
#         ((line_num++))
        
#         # Check for Kuscia versions with context awareness
#         local kuscia_version
#         kuscia_version=$(extract_kuscia_version "$line")
#         if [[ -n "$kuscia_version" && "$kuscia_version" != "$KUSCIA_VERSION" ]]; then
#             issues+=("$file:$line_num: Found Kuscia version '$kuscia_version', expected '$KUSCIA_VERSION'")
#             log_verbose "  Line $line_num: $line"
#         fi
        
#         # Check for SecretFlow versions with context awareness
#         local sf_version
#         sf_version=$(extract_secretflow_version "$line")
#         if [[ -n "$sf_version" && "$sf_version" != "$SECRETFLOW_VERSION" ]]; then
#             issues+=("$file:$line_num: Found SecretFlow version '$sf_version', expected '$SECRETFLOW_VERSION'")
#             log_verbose "  Line $line_num: $line"
#         fi
        
#         # Check for Docker/container image versions
#         local kuscia_image_version
#         kuscia_image_version=$(extract_image_version "$line" "kuscia")
#         if [[ -n "$kuscia_image_version" && "$kuscia_image_version" != "$KUSCIA_VERSION" ]]; then
#             issues+=("$file:$line_num: Found Kuscia image version '$kuscia_image_version', expected '$KUSCIA_VERSION'")
#             log_verbose "  Line $line_num: $line"
#         fi
        
#         local sf_image_version
#         sf_image_version=$(extract_image_version "$line" "secretflow")
#         if [[ -n "$sf_image_version" && "$sf_image_version" != "$SECRETFLOW_VERSION" ]]; then
#             issues+=("$file:$line_num: Found SecretFlow image version '$sf_image_version', expected '$SECRETFLOW_VERSION'")
#             log_verbose "  Line $line_num: $line"
#         fi
        
#     done < "$file"
    
#     printf '%s\n' "${issues[@]}"
# }
# 增强版的 check_file_versions 函数
check_file_versions() {
    local file="$1"
    local issues=()
    local line_num=0
    
    log_verbose "Checking file: $file"
    
    # 特殊处理 MODULE.bazel 文件
    if [[ "$file" == *"MODULE.bazel" ]]; then
        log_verbose "  Special handling for MODULE.bazel file"
        
        # 检查 kuscia 模块
        if grep -q 'name.*=.*"kuscia"' "$file"; then
            log_verbose "  Found kuscia module definition"
            
            # 查找版本号
            local current_version
            current_version=$(grep -A 10 'name.*=.*"kuscia"' "$file" | grep -o 'version.*=.*"[^"]*"' | grep -o '"[^"]*"' | tr -d '"' | head -1)
            
            if [[ -n "$current_version" && "$current_version" != "$KUSCIA_VERSION" ]]; then
                local version_line_num
                version_line_num=$(grep -n 'version.*=.*"[^"]*"' "$file" | head -1 | cut -d: -f1)
                issues+=("$file:$version_line_num: Found Kuscia version '$current_version', expected '$KUSCIA_VERSION'")
                log_verbose "  MODULE.bazel: Found Kuscia version '$current_version', expected '$KUSCIA_VERSION'"
            fi
        fi
        
        # 检查 secretflow 模块
        if grep -q 'name.*=.*"secretflow"' "$file"; then
            log_verbose "  Found secretflow module definition"
            
            local current_version
            current_version=$(grep -A 10 'name.*=.*"secretflow"' "$file" | grep -o 'version.*=.*"[^"]*"' | grep -o '"[^"]*"' | tr -d '"' | head -1)
            
            if [[ -n "$current_version" && "$current_version" != "$SECRETFLOW_VERSION" ]]; then
                local version_line_num
                version_line_num=$(grep -n 'version.*=.*"[^"]*"' "$file" | head -1 | cut -d: -f1)
                issues+=("$file:$version_line_num: Found SecretFlow version '$current_version', expected '$SECRETFLOW_VERSION'")
                log_verbose "  MODULE.bazel: Found SecretFlow version '$current_version', expected '$SECRETFLOW_VERSION'"
            fi
        fi
        
        # 检查 bazel_dep 中的版本
        while IFS= read -r line; do
            ((line_num++))
            
            # 检查 bazel_dep 中的 kuscia 版本
            if echo "$line" | grep -q 'bazel_dep.*name.*=.*"kuscia"'; then
                local dep_version
                dep_version=$(echo "$line" | grep -o 'version.*=.*"[^"]*"' | grep -o '"[^"]*"' | tr -d '"')
                if [[ -n "$dep_version" && "$dep_version" != "$KUSCIA_VERSION" ]]; then
                    issues+=("$file:$line_num: Found Kuscia bazel_dep version '$dep_version', expected '$KUSCIA_VERSION'")
                    log_verbose "  Line $line_num: $line"
                fi
            fi
            
            # 检查 bazel_dep 中的 secretflow 版本
            if echo "$line" | grep -q 'bazel_dep.*name.*=.*"secretflow"'; then
                local dep_version
                dep_version=$(echo "$line" | grep -o 'version.*=.*"[^"]*"' | grep -o '"[^"]*"' | tr -d '"')
                if [[ -n "$dep_version" && "$dep_version" != "$SECRETFLOW_VERSION" ]]; then
                    issues+=("$file:$line_num: Found SecretFlow bazel_dep version '$dep_version', expected '$SECRETFLOW_VERSION'")
                    log_verbose "  Line $line_num: $line"
                fi
            fi
        done < "$file"
    else
        # 其他文件
        while IFS= read -r line; do
            ((line_num++))
            
            # Check for Kuscia versions with context awareness
            local kuscia_version
            kuscia_version=$(extract_kuscia_version "$line")
            if [[ -n "$kuscia_version" && "$kuscia_version" != "$KUSCIA_VERSION" ]]; then
                issues+=("$file:$line_num: Found Kuscia version '$kuscia_version', expected '$KUSCIA_VERSION'")
                log_verbose "  Line $line_num: $line"
            fi
            
            # Check for SecretFlow versions with context awareness
            local sf_version
            sf_version=$(extract_secretflow_version "$line")
            if [[ -n "$sf_version" && "$sf_version" != "$SECRETFLOW_VERSION" ]]; then
                issues+=("$file:$line_num: Found SecretFlow version '$sf_version', expected '$SECRETFLOW_VERSION'")
                log_verbose "  Line $line_num: $line"
            fi
            
            # Check for Docker/container image versions
            local kuscia_image_version
            kuscia_image_version=$(extract_image_version "$line" "kuscia")
            if [[ -n "$kuscia_image_version" && "$kuscia_image_version" != "$KUSCIA_VERSION" ]]; then
                issues+=("$file:$line_num: Found Kuscia image version '$kuscia_image_version', expected '$KUSCIA_VERSION'")
                log_verbose "  Line $line_num: $line"
            fi
            
            local sf_image_version
            sf_image_version=$(extract_image_version "$line" "secretflow")
            if [[ -n "$sf_image_version" && "$sf_image_version" != "$SECRETFLOW_VERSION" ]]; then
                issues+=("$file:$line_num: Found SecretFlow image version '$sf_image_version', expected '$SECRETFLOW_VERSION'")
                log_verbose "  Line $line_num: $line"
            fi
            
        done < "$file"
    fi
    
    printf '%s\n' "${issues[@]}"
}

fix_file_versions() {
    local file="$1"
    local temp_file
    local fixed=false
    
    temp_file=$(mktemp)
    
    log_verbose "Fixing file: $file"
    
    # Copy original file to temp
    cp "$file" "$temp_file"
    
    # 先处理更具体的模式，避免误匹配
    
    # 优先处理 Kuscia 镜像的特殊情况
    # Pattern: KUSCIA_IMAGE 环境变量中的版本 (支持多种镜像源)
    if sed 's#\(KUSCIA_IMAGE=.*kuscia:\)v\?[0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*[a-zA-Z0-9\-\.]*\b#\1'"$KUSCIA_VERSION"'#g' "$temp_file" > "$temp_file.tmp"; then
        if ! cmp -s "$temp_file" "$temp_file.tmp"; then
            fixed=true
        fi
        mv "$temp_file.tmp" "$temp_file"
    fi
    
    # Pattern: export KUSCIA_IMAGE 的情况
    if sed 's#\(export[ \t][ \t]*KUSCIA_IMAGE=.*kuscia:\)v\?[0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*[a-zA-Z0-9\-\.]*\b#\1'"$KUSCIA_VERSION"'#g' "$temp_file" > "$temp_file.tmp"; then
        if ! cmp -s "$temp_file" "$temp_file.tmp"; then
            fixed=true
        fi
        mv "$temp_file.tmp" "$temp_file"
    fi
    
    # 处理其他 Kuscia 版本引用
    
    # Pattern: kuscia:version or kuscia-version
    if sed 's#\b\(kuscia[^:]*:\)v\?[0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*[a-zA-Z0-9\-\.]*\b#\1'"$KUSCIA_VERSION"'#g' "$temp_file" > "$temp_file.tmp"; then
        if ! cmp -s "$temp_file" "$temp_file.tmp"; then
            fixed=true
        fi
        mv "$temp_file.tmp" "$temp_file"
    fi
    
    # Pattern: kuscia followed by space and version
    if sed 's#\bkuscia[ \t][ \t]*v\?[0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*[a-zA-Z0-9\-\.]*\b#kuscia '"$KUSCIA_VERSION"'#g' "$temp_file" > "$temp_file.tmp"; then
        if ! cmp -s "$temp_file" "$temp_file.tmp"; then
            fixed=true
        fi
        mv "$temp_file.tmp" "$temp_file"
    fi
    
    # Pattern: kuscia version:
    if sed 's#\bkuscia[ \t]*version[ \t]*:[ \t]*v\?[0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*[a-zA-Z0-9\-\.]*\b#kuscia version: '"$KUSCIA_VERSION"'#g' "$temp_file" > "$temp_file.tmp"; then
        if ! cmp -s "$temp_file" "$temp_file.tmp"; then
            fixed=true
        fi
        mv "$temp_file.tmp" "$temp_file"
    fi
    
    # Pattern: 文档中的 Kuscia 版本号引用
    if sed 's#\([^a-zA-Z]*[kK]uscia[^a-zA-Z][^0-9]*\)v\?[0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*[a-zA-Z0-9\-\.]*\([ \t]*版本[^a-zA-Z]*\)#\1'"$KUSCIA_VERSION"'\2#g' "$temp_file" > "$temp_file.tmp"; then
        if ! cmp -s "$temp_file" "$temp_file.tmp"; then
            fixed=true
        fi
        mv "$temp_file.tmp" "$temp_file"
    fi
    
    # 处理 msgstr/msgid 中的 Kuscia 版本号
    # Pattern: msgstr "...Kuscia, here we use version X.X.X..."
    if sed 's#\(msgstr.*[kK]uscia.*version[ \t][ \t]*\)v\?[0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*[a-zA-Z0-9\-\.]*\b#\1'"$KUSCIA_VERSION"'#g' "$temp_file" > "$temp_file.tmp"; then
        if ! cmp -s "$temp_file" "$temp_file.tmp"; then
            fixed=true
        fi
        mv "$temp_file.tmp" "$temp_file"
    fi
    
    # Pattern: msgid "...Kuscia version..."
    if sed 's#\(msgid.*[kK]uscia.*\)v\?[0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*[a-zA-Z0-9\-\.]*\b#\1'"$KUSCIA_VERSION"'#g' "$temp_file" > "$temp_file.tmp"; then
        if ! cmp -s "$temp_file" "$temp_file.tmp"; then
            fixed=true
        fi
        mv "$temp_file.tmp" "$temp_file"
    fi
    
    # 新增：处理 URL 路径中的 Kuscia 版本号
    # Pattern: docs/kuscia/vX.X.X/
    if sed 's#\(docs/kuscia/v\)[0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*[a-zA-Z0-9\-\.]*\b#\1'"$KUSCIA_VERSION"'#g' "$temp_file" > "$temp_file.tmp"; then
        if ! cmp -s "$temp_file" "$temp_file.tmp"; then
            fixed=true
        fi
        mv "$temp_file.tmp" "$temp_file"
    fi
    
    # 新增：处理中英文文档中同时包含 Kuscia 和 SecretFlow 版本的情况
    # 注意：需要特别处理，因为一行中可能同时包含两个版本号
    # Pattern: 此处以 Kuscia X.X.X，Secretflow Y.Y.Y 版本为例
    if sed 's#\(此处以[ \t]*[kK]uscia[ \t]*\)v\?[0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*[a-zA-Z0-9\-\.]*\([ \t]*[,，][ \t]*[sS]ecret[fF]*low[ \t]*\)v\?[0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*[a-zA-Z0-9\-\.]*\([ \t]*版本为例\)#\1'"$KUSCIA_VERSION"'\2'"$SECRETFLOW_VERSION"'\3#g' "$temp_file" > "$temp_file.tmp"; then
        if ! cmp -s "$temp_file" "$temp_file.tmp"; then
            fixed=true
        fi
        mv "$temp_file.tmp" "$temp_file"
    fi
    
    # Pattern: Here we take Kuscia X.X.X and Secretflow Y.Y.Y versions as examples
    if sed 's#\(Here we take [kK]uscia[ \t]*\)v\?[0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*[a-zA-Z0-9\-\.]*\([ \t]*and[ \t]*[sS]ecret[fF]*low[ \t]*\)v\?[0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*[a-zA-Z0-9\-\.]*\([ \t]*versions as examples\)#\1'"$KUSCIA_VERSION"'\2'"$SECRETFLOW_VERSION"'\3#g' "$temp_file" > "$temp_file.tmp"; then
        if ! cmp -s "$temp_file" "$temp_file.tmp"; then
            fixed=true
        fi
        mv "$temp_file.tmp" "$temp_file"
    fi
    
    # Pattern: msgstr/msgid 中同时包含两个版本号的情况
    if sed 's#\(msg[si].*[kK]uscia[ \t]*\)v\?[0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*[a-zA-Z0-9\-\.]*\([ \t]*[,，][ \t]*[sS]ecret[fF]*low[ \t]*\)v\?[0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*[a-zA-Z0-9\-\.]*\([ \t]*版本为例\)#\1'"$KUSCIA_VERSION"'\2'"$SECRETFLOW_VERSION"'\3#g' "$temp_file" > "$temp_file.tmp"; then
        if ! cmp -s "$temp_file" "$temp_file.tmp"; then
            fixed=true
        fi
        mv "$temp_file.tmp" "$temp_file"
    fi
    
    # Pattern: msgstr 中英文版本的情况
    if sed 's#\(msgstr.*[kK]uscia[ \t]*\)v\?[0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*[a-zA-Z0-9\-\.]*\([ \t]*and[ \t]*[sS]ecret[fF]*low[ \t]*\)v\?[0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*[a-zA-Z0-9\-\.]*\([ \t]*versions\)#\1'"$KUSCIA_VERSION"'\2'"$SECRETFLOW_VERSION"'\3#g' "$temp_file" > "$temp_file.tmp"; then
        if ! cmp -s "$temp_file" "$temp_file.tmp"; then
            fixed=true
        fi
        mv "$temp_file.tmp" "$temp_file"
    fi
    
    # 处理 SecretFlow 版本（已经处理完 kuscia 镜像，不会冲突）
    
    # Pattern: secretflow: or secret-flow: 
    # 使用更精确的模式，确保不会匹配到已经处理过的 kuscia 镜像
    if sed 's#\b\(secretflow\(/[^/]*\)\?:\)v\?[0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*[a-zA-Z0-9\-\.]*\b#\1'"$SECRETFLOW_VERSION"'#g' "$temp_file" > "$temp_file.tmp"; then
        # 检查是否意外匹配到了 kuscia 镜像，如果是则回滚
        if grep -q "kuscia:$SECRETFLOW_VERSION" "$temp_file.tmp"; then
            # 回滚这个修改
            cp "$temp_file" "$temp_file.tmp"
        else
            if ! cmp -s "$temp_file" "$temp_file.tmp"; then
                fixed=true
            fi
        fi
        mv "$temp_file.tmp" "$temp_file"
    fi
    
    # Pattern: secret-flow:
    if sed 's#\b\(secret-flow[^:]*:\)v\?[0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*[a-zA-Z0-9\-\.]*\b#\1'"$SECRETFLOW_VERSION"'#g' "$temp_file" > "$temp_file.tmp"; then
        if ! cmp -s "$temp_file" "$temp_file.tmp"; then
            fixed=true
        fi
        mv "$temp_file.tmp" "$temp_file"
    fi
    
    # Pattern: secretflow or secret-flow followed by space
    if sed 's#\b\(secretflow\|secret-flow\)[ \t][ \t]*v\?[0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*[a-zA-Z0-9\-\.]*\b#\1 '"$SECRETFLOW_VERSION"'#g' "$temp_file" > "$temp_file.tmp"; then
        if ! cmp -s "$temp_file" "$temp_file.tmp"; then
            fixed=true
        fi
        mv "$temp_file.tmp" "$temp_file"
    fi
    
    # Pattern: secretflow version: or secret-flow version:
    if sed 's#\b\(secretflow\|secret-flow\)[ \t]*version[ \t]*:[ \t]*v\?[0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*[a-zA-Z0-9\-\.]*\b#\1 version: '"$SECRETFLOW_VERSION"'#g' "$temp_file" > "$temp_file.tmp"; then
        if ! cmp -s "$temp_file" "$temp_file.tmp"; then
            fixed=true
        fi
        mv "$temp_file.tmp" "$temp_file"
    fi
    
    # Pattern: 文档中的 SecretFlow 版本号引用
    if sed 's#\([^a-zA-Z]*[sS]ecret[fF]low[^a-zA-Z][^0-9]*\)v\?[0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*[a-zA-Z0-9\-\.]*\([ \t]*版本[^a-zA-Z]*\)#\1'"$SECRETFLOW_VERSION"'\2#g' "$temp_file" > "$temp_file.tmp"; then
        if ! cmp -s "$temp_file" "$temp_file.tmp"; then
            fixed=true
        fi
        mv "$temp_file.tmp" "$temp_file"
    fi
    
    # 新增：处理 msgstr/msgid 中的 SecretFlow 版本号（单独出现的情况）
    # Pattern: msgstr "...SecretFlow version X.X.X..."
    if sed 's#\(msgstr.*[sS]ecret[fF]*low.*version[ \t][ \t]*\)v\?[0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*[a-zA-Z0-9\-\.]*\b#\1'"$SECRETFLOW_VERSION"'#g' "$temp_file" > "$temp_file.tmp"; then
        if ! cmp -s "$temp_file" "$temp_file.tmp"; then
            fixed=true
        fi
        mv "$temp_file.tmp" "$temp_file"
    fi
    
    # Pattern: msgid "...SecretFlow version..."
    if sed 's#\(msgid.*[sS]ecret[fF]*low.*\)v\?[0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*[a-zA-Z0-9\-\.]*\b#\1'"$SECRETFLOW_VERSION"'#g' "$temp_file" > "$temp_file.tmp"; then
        if ! cmp -s "$temp_file" "$temp_file.tmp"; then
            fixed=true
        fi
        mv "$temp_file.tmp" "$temp_file"
    fi
    
    # 新增：处理 URL 路径中的 SecretFlow 版本号
    # Pattern: docs/secretflow/vX.X.X/
    if sed 's#\(docs/secretflow/v\)[0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*[a-zA-Z0-9\-\.]*\b#\1'"$SECRETFLOW_VERSION"'#g' "$temp_file" > "$temp_file.tmp"; then
        if ! cmp -s "$temp_file" "$temp_file.tmp"; then
            fixed=true
        fi
        mv "$temp_file.tmp" "$temp_file"
    fi

    # 新增：处理 MODULE.bazel 文件中的特殊模式
    if [[ "$file" == "MODULE.bazel" ]]; then
        log_verbose "  Processing MODULE.bazel file: $file"
        
        # 检查是否是 kuscia 模块
        if grep -q 'name.*=.*"kuscia"' "$temp_file"; then
            log_verbose "  Found kuscia module definition"
            
            # Pattern 1: 处理多行格式的 version 行
            if sed 's/^\([ \t]*version[ \t]*=[ \t]*"\)[0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*[a-zA-Z0-9\-\.]*\(",.*\)$/\1'"$KUSCIA_VERSION"'\2/' "$temp_file" > "$temp_file.tmp"; then
                if ! cmp -s "$temp_file" "$temp_file.tmp"; then
                    fixed=true
                    log_verbose "  Applied kuscia multi-line version pattern"
                fi
                mv "$temp_file.tmp" "$temp_file"
            fi
            
            # Pattern 2: 处理单行格式
            if sed 's/\(module.*name.*=.*"kuscia".*version.*=.*"\)[0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*[a-zA-Z0-9\-\.]*\("/\1'"$KUSCIA_VERSION"'\2/g' "$temp_file" > "$temp_file.tmp"; then
                if ! cmp -s "$temp_file" "$temp_file.tmp"; then
                    fixed=true
                    log_verbose "  Applied kuscia single-line version pattern"
                fi
                mv "$temp_file.tmp" "$temp_file"
            fi
        fi
        
        # 检查是否是 secretflow 模块
        if grep -q 'name.*=.*"secretflow"' "$temp_file"; then
            log_verbose "  Found secretflow module definition"
            
            # Pattern 1: 处理多行格式的 version 行
            if sed 's/^\([ \t]*version[ \t]*=[ \t]*"\)[0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*[a-zA-Z0-9\-\.]*\(",.*\)$/\1'"$SECRETFLOW_VERSION"'\2/' "$temp_file" > "$temp_file.tmp"; then
                if ! cmp -s "$temp_file" "$temp_file.tmp"; then
                    fixed=true
                    log_verbose "  Applied secretflow multi-line version pattern"
                fi
                mv "$temp_file.tmp" "$temp_file"
            fi
            
            # Pattern 2: 处理单行格式
            if sed 's/\(module.*name.*=.*"secretflow".*version.*=.*"\)[0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*[a-zA-Z0-9\-\.]*\("/\1'"$SECRETFLOW_VERSION"'\2/g' "$temp_file" > "$temp_file.tmp"; then
                if ! cmp -s "$temp_file" "$temp_file.tmp"; then
                    fixed=true
                    log_verbose "  Applied secretflow single-line version pattern"
                fi
                mv "$temp_file.tmp" "$temp_file"
            fi
        fi
    fi

    # Update original file if changes were made
    if [[ "$fixed" == "true" ]]; then
        cp "$temp_file" "$file"
        log_verbose "  Fixed versions in: $file"
    fi
    
    # Clean up
    rm -f "$temp_file" "$temp_file.tmp"
    
    if [[ "$fixed" == "true" ]]; then
        return 0
    else
        return 1
    fi
}

# Main function
main() {
    parse_args "$@"
    validate_args
    
    log_info "Starting version consistency check..."
    log_info "Kuscia version: $KUSCIA_VERSION"
    log_info "SecretFlow version: $SECRETFLOW_VERSION"
    log_info "Check directories: $CHECK_DIRS"
    log_info "Mode: $MODE"
    
    local files=()
    mapfile -t files < <(find_files)

    local total_files=${#files[@]}
    local total_issues=0
    local files_with_issues=()
    local fixed_files=()
    

    if [[ $total_files -eq 0 ]]; then
        log_warn "No files found in specified directories"
        exit 0
    fi
    
    log_info "Found $total_files files to check"
    
    for file in "${files[@]}"; do
        if [[ "$MODE" == "check" ]]; then
            local issues=()
            mapfile -t issues < <(check_file_versions "$file")
            if [[ ${#issues[@]} -gt 0 ]]; then
                files_with_issues+=("$file")
                total_issues=$((total_issues + ${#issues[@]}))
                log_error "Issues found in $file:"
                for issue in "${issues[@]}"; do
                    echo "  $issue"
                done
            fi
        elif [[ "$MODE" == "fix" ]]; then
            if fix_file_versions "$file"; then
                fixed_files+=("$file")
                log_info "Fixed versions in: $file"
            fi
        fi
    done
    
    echo ""
    
    if [[ "$MODE" == "check" ]]; then
        if [[ $total_issues -eq 0 ]]; then
            log_success "No version inconsistencies found! All $total_files files are consistent."
            exit 0
        else
            log_error "Found $total_issues version inconsistencies in ${#files_with_issues[@]} files:"
            for file in "${files_with_issues[@]}"; do
                echo "  - $file"
            done
            echo ""
            log_error "Version consistency check failed!"
            log_info "To fix these issues automatically, run with --mode fix"
            exit 1
        fi
    elif [[ "$MODE" == "fix" ]]; then
        if [[ ${#fixed_files[@]} -eq 0 ]]; then
            log_success "No files needed version fixes."
        else
            log_success "Fixed versions in ${#fixed_files[@]} files:"
            for file in "${fixed_files[@]}"; do
                echo "  - $file"
            done
            log_warn "Please review the changes and commit them if appropriate."
        fi
        exit 0
    fi
}

# Run main function with all arguments
main "$@"
