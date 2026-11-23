HOST_MELKOR_LIMINE_VERSION = 12.9.6
HOST_MELKOR_LIMINE_SOURCE = limine-binary.tar.xz
HOST_MELKOR_LIMINE_SITE = https://github.com/Limine-Bootloader/Limine/releases/download/v$(HOST_MELKOR_LIMINE_VERSION)
HOST_MELKOR_LIMINE_DL_SUBDIR = melkor-limine-$(HOST_MELKOR_LIMINE_VERSION)
HOST_MELKOR_LIMINE_LICENSE = BSD-2-Clause, 0BSD, MIT, CC-BY-3.0, others (see 3RDPARTY.md)
HOST_MELKOR_LIMINE_LICENSE_FILES = LICENSE 3RDPARTY.md

define HOST_MELKOR_LIMINE_INSTALL_CMDS
	$(INSTALL) -D -m 0644 $(@D)/BOOTX64.EFI $(HOST_DIR)/share/melkor-limine/BOOTX64.EFI
	$(INSTALL) -D -m 0644 $(@D)/LICENSE $(HOST_DIR)/share/melkor-limine/LICENSE
	$(INSTALL) -D -m 0644 $(@D)/3RDPARTY.md $(HOST_DIR)/share/melkor-limine/3RDPARTY.md
endef

$(eval $(host-generic-package))
