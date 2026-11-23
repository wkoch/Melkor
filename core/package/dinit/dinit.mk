################################################################################
#
# dinit
#
################################################################################

DINIT_VERSION = 0.22.1
DINIT_SOURCE = dinit-$(DINIT_VERSION).tar.xz
DINIT_SITE = https://github.com/davmac314/dinit/releases/download/v$(DINIT_VERSION)
DINIT_LICENSE = Apache-2.0
DINIT_LICENSE_FILES = LICENSE
DINIT_DEPENDENCIES = host-melkor-mold

define DINIT_CONFIGURE_CMDS
	cd $(@D) && $(TARGET_MAKE_ENV) $(TARGET_CONFIGURE_OPTS) \
		CXXFLAGS="$(TARGET_CXXFLAGS) -flto=auto" LDFLAGS="$(TARGET_LDFLAGS) $(MELKOR_MOLD_LDFLAGS) -flto=auto" \
		CXX_FOR_BUILD="$(HOSTCXX)" ./configure --platform=Linux \
		--prefix=/usr --sbindir=/sbin --disable-strip --disable-capabilities \
		--disable-cgroups --disable-utmpx
endef

define DINIT_BUILD_CMDS
	$(TARGET_MAKE_ENV) $(MAKE) -C $(@D)
endef

define DINIT_INSTALL_TARGET_CMDS
	$(INSTALL) -D -m 0755 $(@D)/src/dinit $(TARGET_DIR)/sbin/dinit
	$(INSTALL) -D -m 0755 $(@D)/src/dinitctl $(TARGET_DIR)/usr/bin/dinitctl
	$(INSTALL) -D -m 0755 $(@D)/src/shutdown $(TARGET_DIR)/sbin/shutdown
endef

$(eval $(generic-package))
