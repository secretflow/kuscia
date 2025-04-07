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

# ========================================== lint.mk ===============================================
# All make targets related to lint should be defined here.
# ========================================== lint.mk ===============================================

# ======================================== golang lint =============================================

.PHONY: lint-golang
lint-golang: # Run golang linter.
	@$(LOG_TARGET)
	golangci-lint --version
	golangci-lint run --out-format=colored-line-number --config=.golangci.yml


# ========================================= yaml lint ==============================================

# https://github.com/adrienverge/yamllint
# rules: https://yamllint.readthedocs.io/en/stable/rules.html
.PHONY: lint-yaml
lint-yaml: # Run yaml linter.
	@$(LOG_TARGET)
	yamllint --version
	yamllint --config-file=./scripts/linter/yamllint/.yamllint .


# ======================================= markdown lint ============================================

# https://github.com/DavidAnson/markdownlint
# rules: https://github.com/DavidAnson/markdownlint/blob/main/doc/Rules.md
# if need to fix md style, can use the --fix parameter
.PHONY: lint-markdown
lint-markdown:
	@$(LOG_TARGET)
	# markdownlint --version
	markdownlint --config ./scripts/linter/markdown/markdown_lint_config.yaml --fix '**/*.md'


# ====================================== code spell lint ===========================================
# https://github.com/codespell-project/codespell
# This makefile command enables silent printing and interactive modification by default.
# You can check for spelling errors in your code and correct them by running `make lint-codespell-check`.
.PHONY: lint-codespell-check
lint-codespell-check: CODESPELL_SKIP := $(shell cat scripts/linter/codespell/.codespell.skip | tr \\n ',')
lint-codespell-check:
	@$(LOG_TARGET)
	codespell --version
	codespell --skip $(CODESPELL_SKIP) --ignore-words scripts/linter/codespell/.codespell.ignorewords


# ================================ license check and fix ============================================
# The license check tools is apache/skywalking-eyes.
# The license check rules is defined in scripts/linter/license/.licenserc.yaml.
# apache/skywalking-eyes config: https://github.com/apache/skywalking-eyes?tab=readme-ov-file#configurations
.PHONY: lint-license-check
lint-license-check:
	@$(LOG_TARGET)
	license-eye -c scripts/linter/license/.licenserc.yaml header check


# You can use make lint-license-fix to automatically add the license header
# to files that have not been added.
.PHONY: lint-license-fix
lint-license-fix:
	@$(LOG_TARGET)
	license-eye --version
	license-eye -c scripts/linter/license/.licenserc.yaml header fix


# ========================================= shell check ============================================
SHELLCHECK_SKIP := SC1091,SC2034

.PHONY: lint-shell-check
lint-shell-check:
	@$(LOG_TARGET)
	shellcheck --version
	shellcheck -e ${SHELLCHECK_SKIP} ./hack/**/*.sh
	shellcheck -e ${SHELLCHECK_SKIP} ./scripts/**/*.sh


##@ Lint

.PHONY: check
check: ## Run all linter checks.
check: go-check yaml-check shell-check markdown-check codespell-check

.PHONY: go-check
go-check: ## Check golang code format.
go-check: lint-golang

.PHONY: yaml-check
yaml-check: ## Check yaml code format.
yaml-check: lint-yaml

.PHONY: shell-check
shell-check: ## Check shell script code format.
shell-check: lint-shell-check

.PHONY: license-fix
license-fix: ## Automatically add license to un added license files
license-fix: lint-license-fix

.PHONY: license-check
license-check: ## Check file license header.
license-check: lint-license-check

.PHONY: markdown-check
markdown-check: ## Check markdown code format.
markdown-check: lint-markdown

.PHONY: codespell-check
codespell-check: ## Check spelling errors in the code.
codespell-check: lint-codespell-check
