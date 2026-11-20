OUT ?= out/m0

.PHONY: image test-boot
image:
	./scripts/build-image.sh "$(OUT)"

test-boot:
	python3 scripts/test-boot.py "$(OUT)/build/images/melkor-m0.img" --log-dir "$(OUT)/tests"
