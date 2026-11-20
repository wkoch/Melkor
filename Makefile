OUT ?= out/m0

# No host Bazzite, as ferramentas de build e de teste ficam no contêiner melkor-build.
# No CI, sem distrobox, os comandos rodam diretamente.
ifeq ($(wildcard /run/.containerenv),)
ifneq ($(shell command -v distrobox 2>/dev/null),)
BUILD_ENV ?= distrobox enter -n melkor-build --
endif
endif

.PHONY: env check-env image test-boot
env:
	./scripts/prepare-env.sh

check-env:
	./scripts/prepare-env.sh --check

image:
	$(BUILD_ENV) ./scripts/build-image.sh "$(OUT)"

test-boot:
	$(BUILD_ENV) python3 scripts/test-boot.py "$(OUT)/build/images/melkor-m0.img" --log-dir "$(OUT)/tests"
