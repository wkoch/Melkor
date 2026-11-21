BOARD ?= m0
OUT ?= out/$(BOARD)
TEST_PYTHON ?= python3
PC_DISK_GIB ?= 16

# No host Bazzite, as ferramentas de build e de teste ficam no contêiner melkor-build.
# No CI, sem distrobox, os comandos rodam diretamente.
ifeq ($(wildcard /run/.containerenv),)
ifneq ($(shell command -v distrobox 2>/dev/null),)
BUILD_ENV ?= distrobox enter -n melkor-build --
endif
endif

.PHONY: env check-env image test-boot check-m1 bundle-m1 bundle-pc test-update test-dns test-live-wifi tools test test-packages pc-disk test-pc-disk test-pc-desktop
env:
	./scripts/prepare-env.sh prepare "$(BOARD)"

check-env:
	./scripts/prepare-env.sh --check "$(BOARD)"

image:
	$(BUILD_ENV) env MELKOR_RAUC_KEYRING="$(MELKOR_RAUC_KEYRING)" MELKOR_LAB_ROOT_HASH_FILE="$(MELKOR_LAB_ROOT_HASH_FILE)" ./scripts/build-image.sh "$(OUT)" "$(BOARD)"

test-boot:
	$(BUILD_ENV) python3 scripts/test-boot.py "$(if $(filter pc,$(BOARD)),$(OUT)/melkor-pc.img,$(OUT)/build/images/melkor-$(BOARD).img)" --log-dir "$(OUT)/tests" \
		$(if $(filter pc,$(BOARD)),--pc) $(if $(filter m1,$(BOARD)),--ab) $(if $(filter live,$(BOARD)),--live) \
		$(if $(PC_USER_PASSWORD_FILE),--pc-user-password "$(PC_USER_PASSWORD_FILE)") \
		$(if $(PC_ROOT_PASSWORD_FILE),--pc-root-password "$(PC_ROOT_PASSWORD_FILE)")

check-m1: BOARD = m1
check-m1:
	$(BUILD_ENV) python3 scripts/check-m1-image.py "$(OUT)/build"

bundle-m1: BOARD = m1
bundle-m1:
	$(BUILD_ENV) python3 scripts/bundle-m1.py "$(OUT)/build" --cert "$(CERT)" --key "$(KEY)" \
		--keyring "$(KEYRING)" --version "$(VERSION)" --output "$(OUT)/melkor-m1-$(VERSION).raucb"

bundle-pc: BOARD = pc
bundle-pc:
	$(BUILD_ENV) python3 scripts/bundle-m1.py "$(OUT)/build" --pc --cert "$(CERT)" --key "$(KEY)" \
		--keyring "$(KEYRING)" --version "$(VERSION)" --output "$(OUT)/melkor-pc-$(VERSION).raucb"

test-update: BOARD = m1
test-update:
	$(BUILD_ENV) python3 tests/m1_update_vm.py "$(OUT)/build" --cert "$(CERT)" --key "$(KEY)" \
		--root-password-file "$(ROOT_PASSWORD_FILE)" --log-dir "$(OUT)/update-tests" \
		$(if $(filter pc,$(BOARD)),--pc-image "$(OUT)/melkor-pc.img")

test-dns: BOARD = m1
test-dns:
	$(BUILD_ENV) python3 tests/m1_dns_vm.py "$(if $(filter pc,$(BOARD)),$(OUT)/melkor-pc.img,$(OUT)/build/images/$(if $(filter live,$(BOARD)),melkor-live.iso,melkor-m1.img))" --log-dir "$(OUT)/dns-tests" \
		$(if $(filter 1,$(DNS_IPV6_ONLY)),--ipv6-only)

test-live-wifi: BOARD = live
test-live-wifi:
	$(BUILD_ENV) python3 tests/live_wifi_vm.py "$(OUT)/build/images" "$(HWSIM)" --log-dir "$(OUT)/wifi-tests"

pc-disk: BOARD = pc
pc-disk:
	cd apps && CGO_ENABLED=0 GOTOOLCHAIN=local go build -trimpath -o "$(abspath $(OUT))/melkor-disco" ./cmd/melkor-disco
	$(BUILD_ENV) env PATH="$(abspath $(OUT))/build/host/bin:/usr/bin:/usr/sbin" "$(OUT)/melkor-disco" \
		-saida "$(OUT)/melkor-pc.img" -gib "$(PC_DISK_GIB)" -core "$(OUT)/build/images/rootfs.squashfs" \
		-sha256 "$$(awk '$$2 == "rootfs.squashfs" {print $$1}' "$(OUT)/SHA256SUMS")" \
		-efi "$(OUT)/build/images/efi-part/EFI/BOOT/bootx64.efi" -grub core/board/pc/grub.cfg \
		$(if $(PC_CREDENTIALS),-credenciais "$(PC_CREDENTIALS)")

test-pc-disk: BOARD = pc
test-pc-disk: test-boot

test-pc-desktop: BOARD = pc
test-pc-desktop:
	$(BUILD_ENV) python3 tests/pc_desktop_vm.py "$(OUT)/melkor-pc.img" --log-dir "$(OUT)/desktop-tests"

tools:
	mkdir -p out/tools
	cd apps && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOTOOLCHAIN=local go build -trimpath -o ../out/tools/melkor .

test-packages: tools
	"$(TEST_PYTHON)" tests/m3_package_smoke.py out/tools/melkor "$(RPM)" "$(RPM_SHA256)" --log-dir "$(OUT)/package-tests"

test:
	"$(TEST_PYTHON)" tests/coverage_suite.py
