MELKOR_STORAGE_VERSION = 1.0
MELKOR_STORAGE_SITE = $(BR2_EXTERNAL_MELKOR_PATH)/../apps
MELKOR_STORAGE_SITE_METHOD = local
MELKOR_STORAGE_GOMOD = melkor.local/apps
MELKOR_STORAGE_BUILD_TARGETS = ./cmd/melkor-storage ./cmd/melkor-core-fetch
MELKOR_STORAGE_INSTALL_BINS = melkor-storage melkor-core-fetch
MELKOR_STORAGE_GO_ENV = CGO_ENABLED=0

$(eval $(golang-package))
