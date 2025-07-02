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
find_markdown_files() {
    local dirs_array
    IFS=' ' read -ra dirs_array <<< "$CHECK_DIRS"
    local files=()
    
    for dir in "${dirs_array[@]}"; do
        if [[ -d "$dir" ]]; then
            log_verbose "Searching for markdown files in: $dir"
            while IFS= read -r -d '' file; do
                files+=("$file")
            done < <(find "$dir" -type f \( -name "*.md" -o -name "*.markdown" \) -print0)
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
check_file_versions() {
    local file="$1"
    local issues=()
    local line_num=0
    
    log_verbose "Checking file: $file"
    
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
    
    printf '%s\n' "${issues[@]}"
}

# Fix version inconsistencies in a file
fix_file_versions() {
    local file="$1"
    local temp_file
    local fixed=false

    temp_file=$(mktemp)
    
    log_verbose "Fixing file: $file"
    
    # Create comprehensive regex patterns for replacement
    # Kuscia patterns: match various formats including legacy versions
    local kuscia_pattern_1='(kuscia[^:]*:)v?[0-9]+\.[0-9]+\.[0-9]+[a-zA-Z0-9\-\.]*'
    local kuscia_pattern_2='(kuscia[[:space:]]+)v?[0-9]+\.[0-9]+\.[0-9]+[a-zA-Z0-9\-\.]*'
    local kuscia_pattern_3='(kuscia[[:space:]]*version[[:space:]]*:?[[:space:]]*)v?[0-9]+\.[0-9]+\.[0-9]+[a-zA-Z0-9\-\.]*'
    
    # SecretFlow patterns: handle both secretflow and secret-flow variants
    local secretflow_pattern_1='(secretflow[^:]*:|secret-flow[^:]*:)v?[0-9]+\.[0-9]+\.[0-9]+[a-zA-Z0-9\-\.]*'
    local secretflow_pattern_2='(secretflow[[:space:]]+|secret-flow[[:space:]]+)v?[0-9]+\.[0-9]+\.[0-9]+[a-zA-Z0-9\-\.]*'
    local secretflow_pattern_3='((secretflow|secret-flow)[[:space:]]*version[[:space:]]*:?[[:space:]]*)v?[0-9]+\.[0-9]+\.[0-9]+[a-zA-Z0-9\-\.]*'
    
    # Fix Kuscia versions with multiple patterns
    for pattern in "$kuscia_pattern_1" "$kuscia_pattern_2" "$kuscia_pattern_3"; do
        if sed "s|$pattern|\1$KUSCIA_VERSION|g" "$file" > "$temp_file"; then
            if ! cmp -s "$file" "$temp_file"; then
                fixed=true
            fi
            cp "$temp_file" "$file"
        fi
    done
    
    # Fix SecretFlow versions with multiple patterns
    for pattern in "$secretflow_pattern_1" "$secretflow_pattern_2" "$secretflow_pattern_3"; do
        if sed "s|$pattern|\1$SECRETFLOW_VERSION|g" "$file" > "$temp_file"; then
            if ! cmp -s "$file" "$temp_file"; then
                fixed=true
            fi
            cp "$temp_file" "$file"
        fi
    done
    
    rm -f "$temp_file"
    
    if [[ "$fixed" == "true" ]]; then
        log_verbose "  Fixed versions in: $file"
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
    mapfile -t files < <(find_markdown_files)
    
    local total_files=${#files[@]}
    local total_issues=0
    local files_with_issues=()
    local fixed_files=()

    if [[ $total_files -eq 0 ]]; then
        log_warn "No markdown files found in specified directories"
        exit 0
    fi
    
    log_info "Found $total_files markdown files to check"
    
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
