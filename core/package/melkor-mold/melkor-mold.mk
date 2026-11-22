HOST_MELKOR_MOLD_VERSION = 2.39.1
HOST_MELKOR_MOLD_SOURCE = mold-$(HOST_MELKOR_MOLD_VERSION)-x86_64-linux.tar.gz
HOST_MELKOR_MOLD_SITE = https://github.com/rui314/mold/releases/download/v$(HOST_MELKOR_MOLD_VERSION)
HOST_MELKOR_MOLD_LICENSE = MIT
HOST_MELKOR_MOLD_LICENSE_FILES = share/doc/mold/LICENSE

MELKOR_MOLD_LDFLAGS = -B$(HOST_DIR)/bin -fuse-ld=mold

define HOST_MELKOR_MOLD_INSTALL_CMDS
	$(INSTALL) -D -m 0755 $(@D)/bin/mold $(HOST_DIR)/bin/mold
	ln -sf mold $(HOST_DIR)/bin/ld.mold
endef

$(eval $(host-generic-package))
