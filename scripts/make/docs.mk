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

# ========================================== docs.mk ===============================================
# All make targets related to docs should be defined here.
# Sphinx Github: https://github.com/sphinx-doc/sphinx
# Sphinx Docs: https://sphinx-doc.readthedocs.io/zh-cn/master/
# ========================================== docs.mk ===============================================

# Links that need to be skipped in bad link detection.
# For example, 127.0.0.1 is an unreachable link and needs to be skipped

# =========================== error link =================================
# http://public.aliyun.com/secretflow/app:v1
# docs/_build/_static/css/custom.css
# docs/_build/_static/js/custom.js
# =========================== error link =================================
LINKINATOR_IGNORE := "jsdelivr custom kuscia ip:port 127.0.0.1 service oss.xxx 1.1.1.1 10.0.0.14 101.11.11.11 docker data.name aliyun.com dev.mysql"

include .VERSION
DOCS_ROOT_DIR        ?= docs
DOCS_SOURCE_DIR      = .
DOCS_OUTPUT_DIR      = _build

# Sphinx build options: https://zh-sphinx-doc.readthedocs.io/en/latest/invocation.html
SPHINX_BUILD   	     ?= sphinx-build
SPHINX_AUTOBUILD	 ?= sphinx-autobuild
SPHINX_OPTS    		 ?= -b html
LANGUAGE             ?= zh_CN

# Version check configuration
VERSION_CHECK_SCRIPT ?= hack/version_check.sh
# Note: The script will automatically detect MODULE.bazel files in addition to markdown, po, and yaml files
VERSION_CHECK_DIRS   ?= docs scripts hack

.PHONY: sphinx-build
sphinx-build: sphinx-clean
sphinx-build: markdown-check
sphinx-build: version_check
sphinx-build:
	@$(LOG_TARGET)
	@$(call errorLog, Warning: if build failed please check sphinx version it must to be 6.2.1!)
	@$(call log, "docs build prepare ....")

	@python3 --version
	@pip3 --version
	@$(SPHINX_BUILD) --version

	@$(call log, "docs build starting ....")
	@$(call log, "Generating documentation in $(LANGUAGE)...")
	# sphinx-build -b html . _build/html
	$(SPHINX_BUILD) $(SPHINX_OPTS) -D language=$(LANGUAGE) $(DOCS_ROOT_DIR)/$(DOCS_SOURCE_DIR) $(DOCS_ROOT_DIR)/$(DOCS_OUTPUT_DIR)
	make link-check
	@$(call log, "docs build success!")


.PHONY: sphinx-clean
sphinx-clean: TARGET_DIR = $(DOCS_ROOT_DIR)/$(DOCS_OUTPUT_DIR)
sphinx-clean:
	@$(LOG_TARGET)
	@if [ "$(wildcard $(TARGET_DIR)/)" ]; then \
    	  rm -rf $(TARGET_DIR); \
	else \
		echo "$(TARGET_DIR) not found, maybe you can need build docs..."; \
	fi


.PHONY: sphinx-preview
sphinx-preview: TARGET_DIR = $(DOCS_ROOT_DIR)/$(DOCS_OUTPUT_DIR)
sphinx-preview: sphinx-build
sphinx-preview:
	@$(LOG_TARGET)
	$(SPHINX_AUTOBUILD) $(DOCS_ROOT_DIR)/$(DOCS_SOURCE_DIR) $(DOCS_ROOT_DIR)/$(DOCS_OUTPUT_DIR)


# https://github.com/JustinBeckwith/linkinator
# More command options: https://github.com/JustinBeckwith/linkinator?tab=readme-ov-file#command-usage
.PHONY: link-check
link-check: TARGET_DIR = $(DOCS_ROOT_DIR)/$(DOCS_OUTPUT_DIR)
link-check:
	@$(LOG_TARGET)
	# linkinator --version
	@if [ "$(wildcard $(TARGET_DIR)/)" ]; then \
		linkinator $(DOCS_ROOT_DIR)/$(DOCS_OUTPUT_DIR) -r --concurrency 25 --skip $(LINKINATOR_IGNORE); \
	else \
		echo "$(TARGET_DIR) not found, maybe you can need build docs..."; \
	fi


.PHONY: verify_error_code
verify_error_code: # Verify integrity of error code i18n configuration.
	@$(LOG_TARGET)
	bash hack/errorcode/gen_error_code_doc.sh verify proto/api/v1alpha1/errorcode/error_code.proto hack/errorcode/i18n/errorcode.zh-CN.toml


.PHONY: gen_error_code_doc
gen_error_code_doc: verify_error_code # Generate error code markdown doc.
	@$(LOG_TARGET)
	bash hack/errorcode/gen_error_code_doc.sh doc proto/api/v1alpha1/errorcode/error_code.proto hack/errorcode/i18n/errorcode.zh-CN.toml docs/reference/apis/error_code_cn.md


.PHONY: version_check
version_check: ## Check and validate Kuscia and SecretFlow version consistency in markdown, po, yaml, and MODULE.bazel files.
	@$(LOG_TARGET)
	@$(call log, "Starting version consistency check...")
	@if [ ! -f "$(VERSION_CHECK_SCRIPT)" ]; then \
		$(call errorLog, "Version check script not found: $(VERSION_CHECK_SCRIPT)"); \
		exit 1; \
	fi
	@bash $(VERSION_CHECK_SCRIPT) \
		--kuscia-version "$(KUSCIA_VERSION)" \
		--secretflow-version "$(SECRETFLOW_VERSION)" \
		--check-dirs "$(VERSION_CHECK_DIRS)" \
		--mode check
	@$(call log, "Version consistency check completed successfully!")


.PHONY: version_fix
version_fix: ## Fix version inconsistencies in markdown, po, yaml, and MODULE.bazel files (use with caution).
	@$(LOG_TARGET)
	@$(call log, "Starting version fix...")
	@if [ ! -f "$(VERSION_CHECK_SCRIPT)" ]; then \
		$(call errorLog, "Version check script not found: $(VERSION_CHECK_SCRIPT)"); \
		exit 1; \
	fi
	@bash $(VERSION_CHECK_SCRIPT) \
		--kuscia-version "$(KUSCIA_VERSION)" \
		--secretflow-version "$(SECRETFLOW_VERSION)" \
		--check-dirs "$(VERSION_CHECK_DIRS)" \
		--mode fix
	@$(call log, "Version fix completed!")


##@ Docs

.PHONY: docs
docs: ## Build docs.
docs: docs-clean gen_error_code_doc version_check sphinx-build

.PHONY: docs-clean
docs-clean: ## Clean docs build.
docs-clean: sphinx-clean

.PHONY: docs-preview
docs-preview: ## Start docs serve to preview docs.
docs-preview: docs-clean sphinx-preview

.PHONY: docs-link-check
docs-link-check: ## Check docs links.
docs-link-check: link-check
