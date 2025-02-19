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

# ========================================== golang.mk ===============================================
# All make targets related to golang should be defined here.
# ========================================== golang.mk ===============================================

CMD_EXCLUDE_TESTS = "example|webdemo|testing|test|container"
PKG_EXCLUDE_TESTS = "crd|testing|test"

# TEST_SUITE used by integration test
TEST_SUITE ?= all

##@ Build

.PHONY: fmt
fmt: # Run go fmt against code.
	@$(LOG_TARGET)
	go fmt ./...


.PHONY: vet
vet: # Run go vet against code.
	@$(LOG_TARGET)
	go vet ./...


.PHONY: test
test: ## Run tests.
test:
	@$(LOG_TARGET)
	rm -rf ./test-results
	mkdir -p test-results

	GOEXPERIMENT=nocoverageredesign go test $$(go list ./cmd/... | grep -Ev ${CMD_EXCLUDE_TESTS}) --parallel 4 -gcflags="all=-N -l" -coverprofile=test-results/cmd.covprofile.out | tee test-results/cmd.output.txt
	GOEXPERIMENT=nocoverageredesign go test $$(go list ./pkg/... | grep -Ev ${PKG_EXCLUDE_TESTS}) --parallel 4 -gcflags="all=-N -l" -coverprofile=test-results/pkg.covprofile.out | tee test-results/pkg.output.txt

	cat ./test-results/cmd.output.txt | go-junit-report > ./test-results/TEST-cmd.xml
	cat ./test-results/pkg.output.txt | go-junit-report > ./test-results/TEST-pkg.xml

	echo "mode: set" > ./test-results/coverage.out && cat ./test-results/*.covprofile.out | grep -v mode: | sort -r | awk '{if($$1 != last) {print $0;last=$$1}}' >> ./test-results/coverage.out
	cat ./test-results/coverage.out | gocover-cobertura > ./test-results/coverage.xml


.PHONY: clean
clean: ## clean build and test product.
clean:
	@$(LOG_TARGET)
	-rm -rf ./test-results
	-rm -rf ./build/apps
	-rm -rf ./build/framework
	-rm -rf ./tmp-crd-code
	-rm -rf ./build/linux


.PHONY: build
build: ## build kuscia binary.
build: check_code
build:
	@$(LOG_TARGET)
	bash hack/build.sh -t kuscia
	mkdir -p build/linux/${ARCH}
	cp -rp build/apps build/linux/${ARCH}




.PHONY: check_code
check_code: ## check code format.
check_code: fmt vet
check_code: verify_error_code
check_code:
	@$(LOG_TARGET)
	@$(call log,  "check code FINISH")


.PHONY: integration_test
integration_test: ## Run Integration Test
integration_test: image
integration_test:
	mkdir -p run/test
	cd run && KUSCIA_IMAGE=${IMG} docker run --rm ${IMG} cat /home/kuscia/scripts/test/integration_test.sh > ./test/integration_test.sh && chmod u+x ./test/integration_test.sh
	cd run && KUSCIA_IMAGE=${IMG} ./test/integration_test.sh ${TEST_SUITE}
