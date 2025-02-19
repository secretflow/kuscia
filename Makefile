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

# All make targets should be implemented in scripts/make/*.mk
# ====================================================================================================
# Supported Targets: (Run `make help` to see more information)
# ====================================================================================================

# --warn-undefined-variables flag.
# See: https://www.gnu.org/software/make/manual/make.html#Reading-Makefiles

_run:
	@$(MAKE) --warn-undefined-variables \
		-f scripts/make/common.mk \
		-f scripts/make/docs.mk \
		-f scripts/make/image.mk \
		-f scripts/make/golang.mk \
		-f scripts/make/lint.mk \
		-f scripts/make/fate.mk \
		$(MAKECMDGOALS)

.PHONY: _run

$(if $(MAKECMDGOALS),$(MAKECMDGOALS): %: _run)
