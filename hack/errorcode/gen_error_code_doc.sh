#!/bin/bash
#
# Copyright 2024 Ant Group Co., Ltd.
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

GREEN='\033[0;32m'
NC='\033[0m'
RED='\033[31m'

DEFAULT_LOCAL_LANGUAGE=zh-CN


INPUT_GO=$2
I18N_FILE_URI=$3
OUTPUT_MD=$4

I18N_CODE_PREFIX=error_code_
I18N_CODE_PREFIX_SUFFIX_DESC=_description
I18N_CODE_PREFIX_SUFFIX_SOLUTION=_solution
local_language=$DEFAULT_LOCAL_LANGUAGE

usage() {
    echo "$(basename "$0") MODE [OPTIONS]
DEPLOY_MODE:
  dif  Check i18n file lost.
  doc  gen markdown doc.

OPTIONS:
  -l i18n language. default is zh-CN
  -m i18n file.
    "
}

function log() {
  local log_content=$1
  echo -e "${GREEN}${log_content}${NC}"
}

function log_warn() {
  local log_content=$1
  echo -e "${RED}${log_content}${NC}"
}

function init_i18n_array() {

    i18n_file=$1
    log "loading i18n file: ${i18n_file}"
    while IFS= read -r line; do

        if [[ $line =~ ^[[:space:]]*([A-Za-z_][A-Za-z0-9_]*)[[:space:]]*=[[:space:]]*\"([^\"]+)\" ]]; then
            const_name=${BASH_REMATCH[1]}
            const_value=${BASH_REMATCH[2]}

            set_value "$const_name" "${const_value}"
        fi

    done < "$i18n_file"
}

function append_dif_i18n_file() {

    # read go file
    while IFS= read -r line; do

        # Check if the line has a constant in form of "ConstName = value"
        if [[ $line =~ ^[[:space:]]*([A-Za-z_][A-Za-z0-9_]*)[[:space:]]*=[[:space:]]*([0-9]+) ]]; then
            const_name=${BASH_REMATCH[1]}
            const_value=${BASH_REMATCH[2]}

            i18n_value_desc=$(get_value "$I18N_CODE_PREFIX""$const_value""${I18N_CODE_PREFIX_SUFFIX_DESC}" )
            if [ -z "$i18n_value_desc" ]; then
                echo "# ${const_name}" >> "$I18N_FILE_URI"
                echo "${I18N_CODE_PREFIX}${const_value}${I18N_CODE_PREFIX_SUFFIX_DESC} = \"\"" >> "$I18N_FILE_URI"
            fi
            i18n_value_solution=$(get_value "$I18N_CODE_PREFIX""$const_value""${I18N_CODE_PREFIX_SUFFIX_SOLUTION}" )
            if [ -z "$i18n_value_solution" ]; then
                echo "# ${const_name}" >> "$I18N_FILE_URI"
                echo "${I18N_CODE_PREFIX}${const_value}${I18N_CODE_PREFIX_SUFFIX_SOLUTION} = \"\"" >> "$I18N_FILE_URI"
            fi

        fi
    done < "$INPUT_GO"
}

function verify_i18n_file_completeness() {

   local i18n_value_desc
   local i18n_value_solution
    # read go file
    while IFS= read -r line; do

        # Check if the line has a constant in form of "ConstName = value"
        if [[ $line =~ ^[[:space:]]*([A-Za-z_][A-Za-z0-9_]*)[[:space:]]*=[[:space:]]*([0-9]+) ]]; then
            const_name=${BASH_REMATCH[1]}
            const_value=${BASH_REMATCH[2]}

            i18n_value_desc=$(get_value "$I18N_CODE_PREFIX""$const_value""${I18N_CODE_PREFIX_SUFFIX_DESC}" )
            if [ -z "${i18n_value_desc}" ]; then
                log_warn "Missing error code [${I18N_CODE_PREFIX}${const_value}${I18N_CODE_PREFIX_SUFFIX_DESC}] i18n configuration. Please perfect the i18n file configuration, file is ${I18N_FILE_URI}"
                exit 2
            fi
            i18n_value_solution=$(get_value "$I18N_CODE_PREFIX""$const_value""${I18N_CODE_PREFIX_SUFFIX_SOLUTION}" )
            if [ -z "${i18n_value_solution}" ]; then
                log_warn "Missing error code [${I18N_CODE_PREFIX}${const_value}${I18N_CODE_PREFIX_SUFFIX_SOLUTION}] i18n configuration. Please perfect the i18n file configuration, file is ${I18N_FILE_URI}"
                exit 2
            fi

        fi
    done < "${INPUT_GO}"
    log "read over file: ${INPUT_GO}"

}

function set_value() {
    local key=$1
    local value=$2

    local var_name="Map_${key}_${local_language}"
    printf -v "${var_name//-/_}" '%s' "$value"
}

function get_value() {
    local key=$1
    local var_name="Map_${key}_${local_language}"

    eval echo \$"${var_name//-/_}"
}

function gen_markdown_doc() {

    log "gen markdown doc > $OUTPUT_MD"
    # Preparing the Markdown file
    echo -e "$(get_value "doc_title")" > "$OUTPUT_MD"
    echo -e "$(get_value "doc_table_title")" >> "$OUTPUT_MD"
    echo "| ----- | ----------- | ----------- |" >> "$OUTPUT_MD"

    while IFS= read -r line; do
       if [[ $line =~ =[[:space:]]*([0-9]+) ]]; then
            const_value=${BASH_REMATCH[1]}
            # Write to the markdown output file
            echo -e "| $const_value | $(get_value "$I18N_CODE_PREFIX""$const_value""${I18N_CODE_PREFIX_SUFFIX_DESC}" ) | $(get_value "$I18N_CODE_PREFIX""$const_value""${I18N_CODE_PREFIX_SUFFIX_SOLUTION}" ) |" >> "$OUTPUT_MD"
        fi

    done < "$INPUT_GO"
}

# init i18n file
init_i18n_array "${I18N_FILE_URI}"

mode=
case "$1" in
append | doc | verify )
  mode=$1
  shift
  ;;
esac

while getopts 'l:m' option; do
  case "$option" in
  l)
    local_language=$option
    usage
    exit
    ;;
  m)
    if [ -z "$OPTARG" ]; then
      printf "Option -m requires an argument.\n" >&2
      exit 1
    fi
    I18N_FILE_URI=$option
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

case "$mode" in
doc)
  gen_markdown_doc
  log "Generated Markdown documentation at $OUTPUT_MD"
  ;;
append)
  append_dif_i18n_file
  log "append diff code $I18N_FILE_URI"
  ;;
verify)
  # append error code empty desc
  append_dif_i18n_file
  # verify
  verify_i18n_file_completeness
  log "verify i18n file [${I18N_FILE_URI}] is complete."
  ;;
*)
  printf "unsupported mode: %s\n" "$mode" >&2
  exit 1
  ;;
esac
