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

# 定义版本匹配和替换规则
declare -A KUSCIA_PATTERNS=(
    # KUSCIA_IMAGE 环境变量（包括 export）
    ["kuscia_image"]="(export[[:space:]]+)?KUSCIA_IMAGE=.*kuscia:"
    # 镜像路径格式
    ["kuscia_image_path"]="([a-zA-Z0-9\-\.]+/)*kuscia:"
    # 简单格式 kuscia:version
    ["kuscia_colon"]="(^|[^a-zA-Z0-9\-/])kuscia:"
    # kuscia version:
    ["kuscia_version_colon"]="kuscia[[:space:]]*version[[:space:]]*:"
    # kuscia 空格 version
    ["kuscia_space"]="kuscia[[:space:]]+"
    # 文档中的版本引用（中文）
    ["kuscia_doc_cn"]="[kK]uscia[^a-zA-Z][^0-9]*"
    # msgstr/msgid 中的版本
    ["kuscia_msg"]="msg(str|id).*[kK]uscia.*version[[:space:]]*"
    # URL 路径中的版本
    ["kuscia_url"]="docs/kuscia/v"
)

declare -A SECRETFLOW_PATTERNS=(
    # SECRETFLOW_IMAGE 环境变量（包括 export）
    ["sf_image"]="(export[[:space:]]+)?SECRETFLOW_IMAGE=.*:"
    # kuscia image pull 命令中的 secretflow 镜像 - 新增
    ["sf_kuscia_pull"]="kuscia[[:space:]]+image[[:space:]]+pull.*[^[:space:]]*secretflow[^[:space:]]*:"
    # secretflow 镜像格式（包含路径的）
    ["sf_image_path"]="([a-zA-Z0-9\-\.]+/)*[^[:space:]]*secretflow[^[:space:]]*:"
    # 简单格式 secretflow:version
    ["sf_colon"]="(^|[^a-zA-Z0-9\-/])(secretflow|secret-flow):"
    # secretflow version:
    ["sf_version_colon"]="(secretflow|secret-flow)[[:space:]]*version[[:space:]]*:"
    # secretflow 空格 version
    ["sf_space"]="(secretflow|secret-flow)[[:space:]]+"
    # 文档中的版本引用（中文）
    ["sf_doc_cn"]="[sS]ecret[fF]low[^a-zA-Z][^0-9]*"
    # msgstr/msgid 中的版本
    ["sf_msg"]="msg(str|id).*[sS]ecret[fF]*low.*version[[:space:]]*"
    # URL 路径中的版本
    ["sf_url"]="docs/secretflow/v"
)

# 检查版本号是否为有效的IP地址
is_valid_ip() {
    local ip="$1"
    local IFS='.'
    local -a octets=($ip)
    
    # IP地址必须有4个部分
    if [[ ${#octets[@]} -ne 4 ]]; then
        return 1
    fi
    
    # 检查每个部分是否为0-255的数字
    for octet in "${octets[@]}"; do
        # 检查是否为纯数字
        if ! [[ "$octet" =~ ^[0-9]+$ ]]; then
            return 1
        fi
        # 检查数值范围
        if (( octet < 0 || octet > 255 )); then
            return 1
        fi
        # 检查前导零（除了单独的0）
        if [[ ${#octet} -gt 1 && "$octet" =~ ^0 ]]; then
            return 1
        fi
    done
    
    return 0
}

# 检查是否应该跳过这个版本号（排除误检测）
should_skip_version() {
    local content="$1"
    local found_version="$2"
    local product_type="$3"  # "kuscia" or "secretflow"
    
    # 0. 首先检查是否为IP地址
    if is_valid_ip "$found_version"; then
        log_verbose "  Skipping IP address: $found_version"
        return 0
    fi
    
    # 0.1 检查是否在网络相关上下文中（更通用的IP地址相关检测）
    if echo "$content" | grep -qiE "(ip|host|address|endpoint|server|client|domain|url|uri|port|socket|network|connection|--[0-9]|<--[0-9]|return.*code|http.*code|error.*code|status.*code)"; then
        # 如果在网络上下文中，且版本号看起来像IP（即使不是标准4段IP）
        if [[ "$found_version" =~ ^[0-9]+\.[0-9]+\.[0-9]+(\.[0-9]+)*$ ]]; then
            log_verbose "  Skipping network-context version that looks like IP: $found_version"
            return 0
        fi
    fi
    
    # 0.2 检查是否在端口号上下文中
    if echo "$content" | grep -qE ":[0-9]{1,5}[^0-9]" && [[ "$found_version" =~ ^[0-9]+\.[0-9]+$ ]]; then
        log_verbose "  Skipping version in port context: $found_version"
        return 0
    fi
    
    # 0.3 检查特定的错误消息模式
    if echo "$content" | grep -qE "(error-message|error.*message|exception|failure|failed|timeout|connection.*refused|http.*code|status.*code)"; then
        # 在错误消息上下文中，对版本号更加严格
        if [[ "$found_version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] && ! echo "$content" | grep -qiE "(version|v[0-9]|release|tag)"; then
            log_verbose "  Skipping version in error message without version context: $found_version"
            return 0
        fi
    fi
    
    # 1. 跳过 URL 中的版本号
    if echo "$content" | grep -qE "https?://[^[:space:]]*${found_version}"; then
        log_verbose "  Skipping version in URL: $found_version"
        return 0
    fi
    
    # 2. 跳过 Markdown 链接中的版本号
    if echo "$content" | grep -qE "\[[^\]]*\]\([^)]*${found_version}[^)]*\)"; then
        log_verbose "  Skipping version in Markdown link: $found_version"
        return 0
    fi
    
    # 3. 跳过其他已知产品的版本号
    if echo "$content" | grep -qiE "(envoy|prometheus|node_exporter|grafana|docker|kubernetes|k8s|etcd|containerd|nginx|apache|mysql|postgresql|redis|mongodb|elasticsearch|python|java|golang|nodejs|react|vue|angular).*${found_version}"; then
        local do_not_skip=false
        if [[ "$product_type" == "kuscia" ]] && echo "$content" | grep -qiE 'kuscia|KUSCIA_IMAGE'; then
            do_not_skip=true
        elif [[ "$product_type" == "secretflow" ]] && echo "$content" | grep -qiE 'secretflow|secret-flow|SF_VERSION'; then
            do_not_skip=true
        fi

        if [[ "$do_not_skip" == "false" ]]; then
            log_verbose "  Skipping other product version: $found_version"
            return 0
        else
            log_verbose "  Found other product keyword, but also target keyword. Not skipping."
        fi
    fi
    
    # 4. 跳过文档路径中非目标产品的版本号
    if echo "$content" | grep -qE "docs/[^/]*/${found_version}" && ! echo "$content" | grep -qiE "docs/(kuscia|secretflow)/${found_version}"; then
        log_verbose "  Skipping version in non-target docs path: $found_version"
        return 0
    fi
    
    # 5. 对于 Kuscia，确保版本号确实与 Kuscia 相关
    if [[ "$product_type" == "kuscia" ]]; then
        # 检查版本号前后是否有明确的 Kuscia 上下文指示
        local before_version after_version
        before_version=$(echo "$content" | sed "s/${found_version}.*//" | tail -c 50)
        after_version=$(echo "$content" | sed "s/.*${found_version}//" | head -c 50)
        
        # 如果版本号附近没有 Kuscia 相关的关键词，可能是误检测
        if ! echo "$before_version$after_version" | grep -qiE "(kuscia|KUSCIA_IMAGE|容器|镜像|版本|version)"; then
            # 除非是特定的已知模式（如 kuscia: 镜像格式）
            if ! echo "$content" | grep -qE "kuscia:.*${found_version}"; then
                log_verbose "  Skipping version without clear Kuscia context: $found_version"
                return 0
            fi
        fi
    fi
    
    # 6. 对于 SecretFlow，确保版本号确实与 SecretFlow 相关
    if [[ "$product_type" == "secretflow" ]]; then
        local before_version after_version
        before_version=$(echo "$content" | sed "s/${found_version}.*//" | tail -c 50)
        after_version=$(echo "$content" | sed "s/.*${found_version}//" | head -c 50)
        
        if ! echo "$before_version$after_version" | grep -qiE "(secretflow|secret-flow|版本|version)"; then
            if ! echo "$content" | grep -qE "(secretflow|secret-flow):.*${found_version}"; then
                log_verbose "  Skipping version without clear SecretFlow context: $found_version"
                return 0
            fi
        fi
    fi
    
    return 1  # 不跳过，应该检测这个版本号
}

# 获取版本号正则表达式
get_version_regex() {
    echo "v?[0-9]+\.[0-9]+\.[0-9]+[a-zA-Z0-9\-\.]*"
}

# 查找需要检查的文件
find_files() {
    local dirs_array
    IFS=' ' read -ra dirs_array <<< "$CHECK_DIRS"
    local files=()
    
    # 查找其他文件
    for dir in "${dirs_array[@]}"; do
        if [[ -d "$dir" ]]; then
            log_verbose "Searching files in: $dir"
            # 查找 markdown, po, yaml 文件
            while IFS= read -r -d '' file; do
                files+=("$file")
            done < <(find "$dir" -type f \( -name "*.md" -o -name "*.markdown" -o -name "*.po" -o -name "*.yaml" -o -name "*.yml" \) -print0)
            
        else
            log_warn "Directory not found: $dir"
        fi
    done

    if [[ ${#files[@]} -gt 0 ]]; then
        printf '%s\n' "${files[@]}"
    fi
}

# 使用优化的方法检查版本
check_versions_optimized() {
    local files=("$@")
    local total_issues=0
    local files_with_issues=()
    
    local version_regex
    version_regex=$(get_version_regex)
    
    log_verbose "Using version regex: $version_regex"
    
    # 第一步：检查所有 Kuscia 版本（包括镜像路径中包含secretflow的情况）
    log_verbose "Step 1: Checking all Kuscia versions"
    
    # 1. 优先检查 KUSCIA_IMAGE 和任何包含 kuscia: 的行
    local kuscia_image_results
    if kuscia_image_results=$(grep -rn -E "kuscia:${version_regex}" "${files[@]}" 2>/dev/null || true); then
        while IFS= read -r line; do
            if [[ -n "$line" ]]; then
                local file_line="${line%%:*}"
                local line_num="${line#*:}"
                line_num="${line_num%%:*}"
                local content="${line#*:*:}"
                
                # 提取版本号
                local found_version
                found_version=$(echo "$content" | grep -oE "$version_regex" | head -1)
                
                if [[ -n "$found_version" && "$found_version" != "$KUSCIA_VERSION" ]]; then
                    if should_skip_version "$content" "$found_version" "kuscia"; then
                        continue
                    fi
                    log_error "Issue found in $file_line:$line_num"
                    echo "  Found Kuscia version '$found_version' (kuscia image), expected '$KUSCIA_VERSION'"
                    echo "  Content: $content"
                    ((total_issues++))
                    
                    if [[ ! " ${files_with_issues[*]} " =~ " $file_line " ]]; then
                        files_with_issues+=("$file_line")
                    fi
                fi
            fi
        done <<< "$kuscia_image_results"
    fi
    
    # 2. 检查其他 Kuscia 版本模式（排除已经检查过的镜像）
    for pattern_name in "${!KUSCIA_PATTERNS[@]}"; do
        # 跳过镜像相关的模式，因为已经在上面处理了
        if [[ "$pattern_name" == "kuscia_image" || "$pattern_name" == "kuscia_image_path" ]]; then
            continue
        fi
        
        local pattern="${KUSCIA_PATTERNS[$pattern_name]}"
        log_verbose "Checking Kuscia pattern: $pattern_name"
        
        local grep_results
        if grep_results=$(grep -rn -E "${pattern}${version_regex}" "${files[@]}" 2>/dev/null || true); then
            while IFS= read -r line; do
                if [[ -n "$line" ]]; then
                    local file_line="${line%%:*}"
                    local line_num="${line#*:}"
                    line_num="${line_num%%:*}"
                    local content="${line#*:*:}"
                    
                    # 跳过包含 kuscia: 的行（因为已经在第1步处理过了）
                    if echo "$content" | grep -q "kuscia:"; then
                        continue
                    fi
                    
                    # 提取版本号
                    local found_version
                    found_version=$(echo "$content" | grep -oE "$version_regex" | head -1)
                    
                    if [[ -n "$found_version" && "$found_version" != "$KUSCIA_VERSION" ]]; then
                        local context_between
                        context_between=$(echo "$content" | sed -n "s/.*[kK]uscia\(.*\)${found_version}.*/\1/p")
                        if echo "$context_between" | grep -qiE 'secretflow|secret-flow'; then
                            continue
                        fi
                        if should_skip_version "$content" "$found_version" "kuscia"; then
                            continue
                        fi
                        log_error "Issue found in $file_line:$line_num"
                        echo "  Found Kuscia version '$found_version' (pattern: $pattern_name), expected '$KUSCIA_VERSION'"
                        echo "  Content: $content"
                        ((total_issues++))
                        
                        if [[ ! " ${files_with_issues[*]} " =~ " $file_line " ]]; then
                            files_with_issues+=("$file_line")
                        fi
                    fi
                fi
            done <<< "$grep_results"
        fi
    done
    
    # 第二步：检查 SecretFlow 版本（排除包含 kuscia: 的行）
    log_verbose "Step 2: Checking SecretFlow versions (excluding kuscia images)"

    # 1. 优先检查 SECRETFLOW_IMAGE 环境变量
    local sf_image_results
    if sf_image_results=$(grep -rn -E "(export[[:space:]]+)?SECRETFLOW_IMAGE=.*:${version_regex}" "${files[@]}" 2>/dev/null || true); then
        while IFS= read -r line; do
            if [[ -n "$line" ]]; then
                local file_line="${line%%:*}"
                local line_num="${line#*:}"
                line_num="${line_num%%:*}"
                local content="${line#*:*:}"
                
                # 提取版本号
                local found_version
                found_version=$(echo "$content" | grep -oE "$version_regex" | head -1)
                
                if [[ -n "$found_version" && "$found_version" != "$SECRETFLOW_VERSION" ]]; then
                    if should_skip_version "$content" "$found_version" "secretflow"; then
                        continue
                    fi
                    log_error "Issue found in $file_line:$line_num"
                    echo "  Found SecretFlow version '$found_version' (SECRETFLOW_IMAGE), expected '$SECRETFLOW_VERSION'"
                    echo "  Content: $content"
                    ((total_issues++))
                    
                    if [[ ! " ${files_with_issues[*]} " =~ " $file_line " ]]; then
                        files_with_issues+=("$file_line")
                    fi
                fi
            fi
        done <<< "$sf_image_results"
    fi
    
    local sf_kuscia_pull_results
    if sf_kuscia_pull_results=$(grep -rn -E "kuscia[[:space:]]+image[[:space:]]+pull.*[^[:space:]]*secretflow[^[:space:]]*:${version_regex}" "${files[@]}" 2>/dev/null || true); then
        while IFS= read -r line; do
            if [[ -n "$line" ]]; then
                local file_line="${line%%:*}"
                local line_num="${line#*:}"
                line_num="${line_num%%:*}"
                local content="${line#*:*:}"
                
                # 提取版本号
                local found_version
                found_version=$(echo "$content" | grep -oE "$version_regex" | head -1)
                
                if [[ -n "$found_version" && "$found_version" != "$SECRETFLOW_VERSION" ]]; then
                    if should_skip_version "$content" "$found_version" "secretflow"; then
                        continue
                    fi
                    log_error "Issue found in $file_line:$line_num"
                    echo "  Found SecretFlow version '$found_version' (kuscia pull command), expected '$SECRETFLOW_VERSION'"
                    echo "  Content: $content"
                    ((total_issues++))
                    
                    if [[ ! " ${files_with_issues[*]} " =~ " $file_line " ]]; then
                        files_with_issues+=("$file_line")
                    fi
                fi
            fi
        done <<< "$sf_kuscia_pull_results"
    fi

    for pattern_name in "${!SECRETFLOW_PATTERNS[@]}"; do
        # 跳过SECRETFLOW_IMAGE模式，因为已经在上面处理了 - 新增条件
        if [[ "$pattern_name" == "sf_image" || "$pattern_name" == "sf_kuscia_pull" ]]; then
            continue
        fi

        local pattern="${SECRETFLOW_PATTERNS[$pattern_name]}"
        log_verbose "Checking SecretFlow pattern: $pattern_name"
        
        local grep_results
        if grep_results=$(grep -rn -E "${pattern}${version_regex}" "${files[@]}" 2>/dev/null || true); then
            while IFS= read -r line; do
                if [[ -n "$line" ]]; then
                    local file_line="${line%%:*}"
                    local line_num="${line#*:}"
                    line_num="${line_num%%:*}"
                    local content="${line#*:*:}"
                    
                    # 重要：跳过包含 kuscia: 的行，这些应该被识别为 Kuscia 镜像
                    if echo "$content" | grep -q "kuscia:"; then
                        log_verbose "  Skipping line with kuscia: $content"
                        continue
                    fi
                    
                    # 提取版本号
                    local found_version
                    # 首先尝试提取紧跟在 secretflow 关键词后的版本号
                    found_version=$(echo "$content" | grep -oiE "(secretflow|secret-flow)[^0-9]*${version_regex}" | grep -oE "$version_regex" | head -1)
                    # 如果没有找到，再提取整行第一个版本号
                    if [[ -z "$found_version" ]]; then
                        found_version=$(echo "$content" | grep -oE "$version_regex" | head -1)
                    fi
                    
                    if [[ -n "$found_version" && "$found_version" != "$SECRETFLOW_VERSION" ]]; then
                        local context_between
                        context_between=$(echo "$content" | sed -n "s/.*[sS]ecret[fF]*low\(.*\)${found_version}.*/\1/p")
                        if echo "$context_between" | grep -qi 'kuscia'; then
                            continue
                        fi
                        if should_skip_version "$content" "$found_version" "secretflow"; then
                            continue
                        fi
                        log_error "Issue found in $file_line:$line_num"
                        echo "  Found SecretFlow version '$found_version' (pattern: $pattern_name), expected '$SECRETFLOW_VERSION'"
                        echo "  Content: $content"
                        ((total_issues++))
                        
                        if [[ ! " ${files_with_issues[*]} " =~ " $file_line " ]]; then
                            files_with_issues+=("$file_line")
                        fi
                    fi
                fi
            done <<< "$grep_results"
        fi
    done
    
    # 第三步：检查复合模式（同时包含两个版本号的情况）
    log_verbose "Step 3: Checking combined version patterns"
    
    # 中文组合模式检查
    local combined_cn_results
    if combined_cn_results=$(grep -rn -E "此处以[[:space:]]*[kK]uscia[[:space:]]*${version_regex}[[:space:]]*[,，][[:space:]]*[sS]ecret[fF]*low[[:space:]]*${version_regex}[[:space:]]*版本为例" "${files[@]}" 2>/dev/null || true); then
        while IFS= read -r line; do
            if [[ -n "$line" ]]; then
                local file_line="${line%%:*}"
                local line_num="${line#*:}"
                line_num="${line_num%%:*}"
                local content="${line#*:*:}"
                
                # 提取两个版本号
                local versions
                versions=($(echo "$content" | grep -oE "$version_regex"))
                
                if [[ ${#versions[@]} -ge 2 ]]; then
                    local kuscia_version="${versions[0]}"
                    local sf_version="${versions[1]}"
                    
                    if [[ "$kuscia_version" != "$KUSCIA_VERSION" ]]; then
                        if ! should_skip_version "$content" "$kuscia_version" "kuscia"; then
                            log_error "Issue found in $file_line:$line_num"
                            echo "  Found Kuscia version '$kuscia_version' in combined pattern, expected '$KUSCIA_VERSION'"
                            echo "  Content: $content"
                            ((total_issues++))
                            
                            if [[ ! " ${files_with_issues[*]} " =~ " $file_line " ]]; then
                                files_with_issues+=("$file_line")
                            fi
                        fi
                    fi
                    
                    if [[ "$sf_version" != "$SECRETFLOW_VERSION" ]]; then
                        if ! should_skip_version "$content" "$sf_version" "secretflow"; then
                            log_error "Issue found in $file_line:$line_num"
                            echo "  Found SecretFlow version '$sf_version' in combined pattern, expected '$SECRETFLOW_VERSION'"
                            echo "  Content: $content"
                            ((total_issues++))
                            
                            if [[ ! " ${files_with_issues[*]} " =~ " $file_line " ]]; then
                                files_with_issues+=("$file_line")
                            fi
                        fi
                    fi
                fi
            fi
        done <<< "$combined_cn_results"
    fi

    echo "$total_issues:${files_with_issues[*]}"
}

# 修复版本号 - 严格按照检查到的位置修改
fix_versions_optimized() {
    local files=("$@")
    local fixed_files=()
    local version_regex
    version_regex=$(get_version_regex)
    
    log_verbose "Starting precise fix based on check results"
    
    # 第一步：修复所有 Kuscia 版本（包括镜像路径中包含secretflow的情况）
    log_verbose "Step 1: Fixing all Kuscia versions"
    
    # 1. 修复任何包含 kuscia: 的行
    local kuscia_image_results
    if kuscia_image_results=$(grep -rn -E "kuscia:${version_regex}" "${files[@]}" 2>/dev/null || true); then
        while IFS= read -r line; do
            if [[ -n "$line" ]]; then
                local file_line="${line%%:*}"
                local line_num="${line#*:}"
                line_num="${line_num%%:*}"
                local content="${line#*:*:}"
                
                # 提取版本号
                local found_version
                found_version=$(echo "$content" | grep -oE "$version_regex" | head -1)
                
                if [[ -n "$found_version" && "$found_version" != "$KUSCIA_VERSION" ]]; then
                    if should_skip_version "$content" "$found_version" "kuscia"; then
                        continue
                    fi
                    log_verbose "Fixing Kuscia version at $file_line:$line_num: $found_version -> $KUSCIA_VERSION"
                    
                    # 使用 sed 精确替换该行的版本号
                    sed -i "${line_num}s/${found_version}/${KUSCIA_VERSION}/g" "$file_line"
                    
                    if [[ ! " ${fixed_files[*]} " =~ " $file_line " ]]; then
                        fixed_files+=("$file_line")
                    fi
                fi
            fi
        done <<< "$kuscia_image_results"
    fi
    
    # 2. 修复其他 Kuscia 版本模式（排除已经检查过的镜像）
    for pattern_name in "${!KUSCIA_PATTERNS[@]}"; do
        # 跳过镜像相关的模式，因为已经在上面处理了
        if [[ "$pattern_name" == "kuscia_image" || "$pattern_name" == "kuscia_image_path" ]]; then
            continue
        fi
        
        local pattern="${KUSCIA_PATTERNS[$pattern_name]}"
        log_verbose "Fixing Kuscia pattern: $pattern_name"
        
        local grep_results
        if grep_results=$(grep -rn -E "${pattern}${version_regex}" "${files[@]}" 2>/dev/null || true); then
            while IFS= read -r line; do
                if [[ -n "$line" ]]; then
                    local file_line="${line%%:*}"
                    local line_num="${line#*:}"
                    line_num="${line_num%%:*}"
                    local content="${line#*:*:}"
                    
                    # 跳过包含 kuscia: 的行（因为已经在第1步处理过了）
                    if echo "$content" | grep -q "kuscia:"; then
                        continue
                    fi
                    
                    # 提取版本号
                    local found_version
                    found_version=$(echo "$content" | grep -oE "$version_regex" | head -1)
                    
                    if [[ -n "$found_version" && "$found_version" != "$KUSCIA_VERSION" ]]; then
                        local context_between
                        context_between=$(echo "$content" | sed -n "s/.*[kK]uscia\(.*\)${found_version}.*/\1/p")
                        if echo "$context_between" | grep -qiE 'secretflow|secret-flow'; then
                            continue
                        fi
                        if should_skip_version "$content" "$found_version" "kuscia"; then
                            continue
                        fi
                        log_verbose "Fixing Kuscia version at $file_line:$line_num: $found_version -> $KUSCIA_VERSION"
                        
                        # 使用 sed 精确替换该行的版本号
                        sed -i "${line_num}s/${found_version}/${KUSCIA_VERSION}/g" "$file_line"
                        
                        if [[ ! " ${fixed_files[*]} " =~ " $file_line " ]]; then
                            fixed_files+=("$file_line")
                        fi
                    fi
                fi
            done <<< "$grep_results"
        fi
    done
    
    # 第二步：修复 SecretFlow 版本（排除包含 kuscia: 的行）
    log_verbose "Step 2: Fixing SecretFlow versions (excluding kuscia images)"

    # 1. 修复 SECRETFLOW_IMAGE 环境变量
    local sf_image_results
    if sf_image_results=$(grep -rn -E "(export[[:space:]]+)?SECRETFLOW_IMAGE=.*:${version_regex}" "${files[@]}" 2>/dev/null || true); then
        while IFS= read -r line; do
            if [[ -n "$line" ]]; then
                local file_line="${line%%:*}"
                local line_num="${line#*:}"
                line_num="${line_num%%:*}"
                local content="${line#*:*:}"
                
                # 提取版本号
                local found_version
                found_version=$(echo "$content" | grep -oE "$version_regex" | head -1)
                
                if [[ -n "$found_version" && "$found_version" != "$SECRETFLOW_VERSION" ]]; then
                    if should_skip_version "$content" "$found_version" "secretflow"; then
                        continue
                    fi
                    log_verbose "Fixing SecretFlow version at $file_line:$line_num: $found_version -> $SECRETFLOW_VERSION"
                    
                    # 使用 sed 精确替换该行的版本号
                    sed -i "${line_num}s/${found_version}/${SECRETFLOW_VERSION}/g" "$file_line"
                    
                    if [[ ! " ${fixed_files[*]} " =~ " $file_line " ]]; then
                        fixed_files+=("$file_line")
                    fi
                fi
            fi
        done <<< "$sf_image_results"
    fi

    local sf_kuscia_pull_results
    if sf_kuscia_pull_results=$(grep -rn -E "kuscia[[:space:]]+image[[:space:]]+pull.*[^[:space:]]*secretflow[^[:space:]]*:${version_regex}" "${files[@]}" 2>/dev/null || true); then
        while IFS= read -r line; do
            if [[ -n "$line" ]]; then
                local file_line="${line%%:*}"
                local line_num="${line#*:}"
                line_num="${line_num%%:*}"
                local content="${line#*:*:}"
                
                # 提取版本号
                local found_version
                found_version=$(echo "$content" | grep -oE "$version_regex" | head -1)
                
                if [[ -n "$found_version" && "$found_version" != "$SECRETFLOW_VERSION" ]]; then
                    if should_skip_version "$content" "$found_version" "secretflow"; then
                        continue
                    fi
                    log_verbose "Fixing SecretFlow version at $file_line:$line_num: $found_version -> $SECRETFLOW_VERSION"
                    
                    # 使用 sed 精确替换该行的版本号
                    sed -i "${line_num}s/${found_version}/${SECRETFLOW_VERSION}/g" "$file_line"
                    
                    if [[ ! " ${fixed_files[*]} " =~ " $file_line " ]]; then
                        fixed_files+=("$file_line")
                    fi
                fi
            fi
        done <<< "$sf_kuscia_pull_results"
    fi
        
    for pattern_name in "${!SECRETFLOW_PATTERNS[@]}"; do
        # 跳过SECRETFLOW_IMAGE模式，因为已经在上面处理了 - 新增条件
        if [[ "$pattern_name" == "sf_image" || "$pattern_name" == "sf_kuscia_pull" ]]; then
            continue
        fi
        
        local pattern="${SECRETFLOW_PATTERNS[$pattern_name]}"
        log_verbose "Fixing SecretFlow pattern: $pattern_name"
        
        local grep_results
        if grep_results=$(grep -rn -E "${pattern}${version_regex}" "${files[@]}" 2>/dev/null || true); then
            while IFS= read -r line; do
                if [[ -n "$line" ]]; then
                    local file_line="${line%%:*}"
                    local line_num="${line#*:}"
                    line_num="${line_num%%:*}"
                    local content="${line#*:*:}"
                    
                    # 重要：跳过包含 kuscia: 的行，这些应该被识别为 Kuscia 镜像
                    if echo "$content" | grep -q "kuscia:"; then
                        log_verbose "  Skipping line with kuscia: $content"
                        continue
                    fi
                    
                    # 提取版本号
                    local found_version
                    # 首先尝试提取紧跟在 secretflow 关键词后的版本号
                    found_version=$(echo "$content" | grep -oiE "(secretflow|secret-flow)[^0-9]*${version_regex}" | grep -oE "$version_regex" | head -1)
                    # 如果没有找到，再提取整行第一个版本号
                    if [[ -z "$found_version" ]]; then
                        found_version=$(echo "$content" | grep -oE "$version_regex" | head -1)
                    fi
                    
                    if [[ -n "$found_version" && "$found_version" != "$SECRETFLOW_VERSION" ]]; then
                        local context_between
                        context_between=$(echo "$content" | sed -n "s/.*[sS]ecret[fF]*low\(.*\)${found_version}.*/\1/p")
                        if echo "$context_between" | grep -qi 'kuscia'; then
                            continue
                        fi
                        if should_skip_version "$content" "$found_version" "secretflow"; then
                            continue
                        fi
                        log_verbose "Fixing SecretFlow version at $file_line:$line_num: $found_version -> $SECRETFLOW_VERSION"
                        
                        # 使用 sed 精确替换该行的版本号
                        sed -i "${line_num}s/${found_version}/${SECRETFLOW_VERSION}/g" "$file_line"
                        
                        if [[ ! " ${fixed_files[*]} " =~ " $file_line " ]]; then
                            fixed_files+=("$file_line")
                        fi
                    fi
                fi
            done <<< "$grep_results"
        fi
    done
    
    # 第三步：处理复合模式（同时包含两个版本号的情况）
    log_verbose "Step 3: Fixing combined version patterns"
    
    # 中文组合模式：此处以 Kuscia X.X.X，SecretFlow Y.Y.Y 版本为例
    local combined_cn_results
    if combined_cn_results=$(grep -rn -E "此处以[[:space:]]*[kK]uscia[[:space:]]*${version_regex}[[:space:]]*[,，][[:space:]]*[sS]ecret[fF]*low[[:space:]]*${version_regex}[[:space:]]*版本为例" "${files[@]}" 2>/dev/null || true); then
        while IFS= read -r line; do
            if [[ -n "$line" ]]; then
                local file_line="${line%%:*}"
                local line_num="${line#*:}"
                line_num="${line_num%%:*}"
                local content="${line#*:*:}"
                
                # 提取两个版本号
                local versions
                versions=($(echo "$content" | grep -oE "$version_regex"))
                
                if [[ ${#versions[@]} -ge 2 ]]; then
                    local kuscia_version="${versions[0]}"
                    local sf_version="${versions[1]}"
                    
                    local need_fix=false
                    if [[ "$kuscia_version" != "$KUSCIA_VERSION" ]]; then
                        if ! should_skip_version "$content" "$kuscia_version" "kuscia"; then
                            need_fix=true
                        fi
                    fi
                    if [[ "$sf_version" != "$SECRETFLOW_VERSION" ]]; then
                        if ! should_skip_version "$content" "$sf_version" "secretflow"; then
                            need_fix=true
                        fi
                    fi
                    
                    if [[ "$need_fix" == "true" ]]; then
                        log_verbose "Fixing combined pattern at $file_line:$line_num: Kuscia $kuscia_version -> $KUSCIA_VERSION, SecretFlow $sf_version -> $SECRETFLOW_VERSION"
                        
                        # 精确替换：先替换第一个版本号，再替换第二个版本号
                        if [[ "$kuscia_version" != "$KUSCIA_VERSION" ]] && ! should_skip_version "$content" "$kuscia_version" "kuscia"; then
                            sed -i "${line_num}s/${kuscia_version}/${KUSCIA_VERSION}/" "$file_line"
                        fi
                        if [[ "$sf_version" != "$SECRETFLOW_VERSION" ]] && ! should_skip_version "$content" "$sf_version" "secretflow"; then
                            sed -i "${line_num}s/${sf_version}/${SECRETFLOW_VERSION}/" "$file_line"
                        fi
                        
                        if [[ ! " ${fixed_files[*]} " =~ " $file_line " ]]; then
                            fixed_files+=("$file_line")
                        fi
                    fi
                fi
            fi
        done <<< "$combined_cn_results"
    fi
    
    # 英文组合模式：Here we take Kuscia X.X.X and SecretFlow Y.Y.Y versions as examples
    local combined_en_results
    if combined_en_results=$(grep -rn -E "Here we take [kK]uscia[[:space:]]*${version_regex}[[:space:]]*and[[:space:]]*[sS]ecret[fF]*low[[:space:]]*${version_regex}[[:space:]]*versions as examples" "${files[@]}" 2>/dev/null || true); then
        while IFS= read -r line; do
            if [[ -n "$line" ]]; then
                local file_line="${line%%:*}"
                local line_num="${line#*:}"
                line_num="${line_num%%:*}"
                local content="${line#*:*:}"
                
                # 提取两个版本号
                local versions
                versions=($(echo "$content" | grep -oE "$version_regex"))
                
                if [[ ${#versions[@]} -ge 2 ]]; then
                    local kuscia_version="${versions[0]}"
                    local sf_version="${versions[1]}"
                    
                    local need_fix=false
                    if [[ "$kuscia_version" != "$KUSCIA_VERSION" ]]; then
                        need_fix=true
                    fi
                    if [[ "$sf_version" != "$SECRETFLOW_VERSION" ]]; then
                        need_fix=true
                    fi
                    
                    if [[ "$need_fix" == "true" ]]; then
                        log_verbose "Fixing English combined pattern at $file_line:$line_num: Kuscia $kuscia_version -> $KUSCIA_VERSION, SecretFlow $sf_version -> $SECRETFLOW_VERSION"
                        
                        # 精确替换：先替换第一个版本号，再替换第二个版本号
                        if [[ "$kuscia_version" != "$KUSCIA_VERSION" ]]; then
                            sed -i "${line_num}s/${kuscia_version}/${KUSCIA_VERSION}/" "$file_line"
                        fi
                        if [[ "$sf_version" != "$SECRETFLOW_VERSION" ]]; then
                            sed -i "${line_num}s/${sf_version}/${SECRETFLOW_VERSION}/" "$file_line"
                        fi
                        
                        if [[ ! " ${fixed_files[*]} " =~ " $file_line " ]]; then
                            fixed_files+=("$file_line")
                        fi
                    fi
                fi
            fi
        done <<< "$combined_en_results"
    fi
    
    printf '%s\n' "${fixed_files[@]}"
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

    if [[ $total_files -eq 0 ]]; then
        log_warn "No files found in specified directories"
        exit 0
    fi
    
    log_info "Found $total_files files to check"
    
    if [[ "$MODE" == "check" ]]; then
        local check_result
        check_result=$(check_versions_optimized "${files[@]}")
        local total_issues="${check_result%%:*}"
        local files_with_issues_str="${check_result#*:}"
        
        if [[ $total_issues -eq 0 ]]; then
            log_success "No version inconsistencies found! All $total_files files are consistent."
            exit 0
        else
            log_error "Found $total_issues version inconsistencies"
            if [[ -n "$files_with_issues_str" ]]; then
                echo ""
                log_error "Files with issues:"
                local files_with_issues_array=($files_with_issues_str)
                for file in "${files_with_issues_array[@]}"; do
                    echo "  - $file"
                done
            fi
            echo ""
            log_error "Version consistency check failed!"
            log_info "To fix these issues automatically, run with --mode fix"
            exit 1
        fi
    elif [[ "$MODE" == "fix" ]]; then
        local fixed_files=()
        mapfile -t fixed_files < <(fix_versions_optimized "${files[@]}")
        
        if [[ ${#fixed_files[@]} -eq 0 ]]; then
            log_success "No files needed version fixes."
        else
            log_success "Fixed versions in ${#fixed_files[@]} files: ${fixed_files[*]}"
            log_warn "Please review the changes and commit them if appropriate."
        fi
        exit 0
    fi
}

# Run main function with all arguments
main "$@"
