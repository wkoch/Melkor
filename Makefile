BOARD ?= m0
OUT ?= out/$(BOARD)
TEST_PYTHON ?= python3

# No host Bazzite, as ferramentas de build e de teste ficam no contêiner melkor-build.
# No CI, sem distrobox, os comandos rodam diretamente.
ifeq ($(wildcard /run/.containerenv),)
ifneq ($(shell command -v distrobox 2>/dev/null),)
BUILD_ENV ?= distrobox enter -n melkor-build --
endif
endif

.PHONY: env check-env image test-boot check-m1 bundle-m1 test-update test-dns tools test test-packages
env:
	./scripts/prepare-env.sh prepare "$(BOARD)"

check-env:
	./scripts/prepare-env.sh --check "$(BOARD)"

image:
	$(BUILD_ENV) env MELKOR_RAUC_KEYRING="$(MELKOR_RAUC_KEYRING)" MELKOR_LAB_ROOT_HASH_FILE="$(MELKOR_LAB_ROOT_HASH_FILE)" ./scripts/build-image.sh "$(OUT)" "$(BOARD)"

test-boot:
	$(BUILD_ENV) python3 scripts/test-boot.py "$(OUT)/build/images/melkor-$(BOARD).img" --log-dir "$(OUT)/tests" $(if $(filter m1,$(BOARD)),--ab) $(if $(filter live,$(BOARD)),--live)

check-m1: BOARD = m1
check-m1:
	$(BUILD_ENV) python3 scripts/check-m1-image.py "$(OUT)/build"

bundle-m1: BOARD = m1
bundle-m1:
	$(BUILD_ENV) python3 scripts/bundle-m1.py "$(OUT)/build" --cert "$(CERT)" --key "$(KEY)" \
		--keyring "$(KEYRING)" --version "$(VERSION)" --output "$(OUT)/melkor-m1-$(VERSION).raucb"

test-update: BOARD = m1
test-update:
	$(BUILD_ENV) python3 tests/m1_update_vm.py "$(OUT)/build" --cert "$(CERT)" --key "$(KEY)" \
		--root-password-file "$(ROOT_PASSWORD_FILE)" --log-dir "$(OUT)/update-tests"

test-dns: BOARD = m1
test-dns:
	$(BUILD_ENV) python3 tests/m1_dns_vm.py "$(OUT)/build/images/melkor-m1.img" --log-dir "$(OUT)/dns-tests" \
		$(if $(filter 1,$(DNS_IPV6_ONLY)),--ipv6-only)

tools:
	mkdir -p out/tools
	cd apps && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOTOOLCHAIN=local go build -trimpath -o ../out/tools/melkor .

test-packages: tools
	"$(TEST_PYTHON)" tests/m3_package_smoke.py out/tools/melkor "$(RPM)" "$(RPM_SHA256)" --log-dir "$(OUT)/package-tests"

test:
	"$(TEST_PYTHON)" tests/coverage_suite.py
