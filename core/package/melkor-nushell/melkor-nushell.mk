MELKOR_NUSHELL_VERSION = 0.117.0
MELKOR_NUSHELL_SOURCE = nu-$(MELKOR_NUSHELL_VERSION)-x86_64-unknown-linux-musl.tar.gz
MELKOR_NUSHELL_SITE = https://github.com/nushell/nushell/releases/download/$(MELKOR_NUSHELL_VERSION)
MELKOR_NUSHELL_LICENSE = MIT
MELKOR_NUSHELL_LICENSE_FILES = LICENSE

define MELKOR_NUSHELL_INSTALL_TARGET_CMDS
	$(INSTALL) -D -m 0755 $(@D)/nu $(TARGET_DIR)/usr/bin/nu
	$(INSTALL) -D -m 0644 $(@D)/LICENSE $(TARGET_DIR)/usr/share/licenses/nushell/LICENSE
endef

$(eval $(generic-package))
