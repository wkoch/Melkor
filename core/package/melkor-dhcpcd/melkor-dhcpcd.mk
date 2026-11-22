MELKOR_DHCPCD_VERSION = 10.5.2
MELKOR_DHCPCD_SITE = https://github.com/NetworkConfiguration/dhcpcd/releases/download/v$(MELKOR_DHCPCD_VERSION)
MELKOR_DHCPCD_SOURCE = dhcpcd-$(MELKOR_DHCPCD_VERSION).tar.xz
MELKOR_DHCPCD_LICENSE = BSD-2-Clause
MELKOR_DHCPCD_LICENSE_FILES = LICENSE
MELKOR_DHCPCD_DEPENDENCIES = host-pkgconf host-melkor-mold

define MELKOR_DHCPCD_CONFIGURE_CMDS
	cd $(@D) && $(TARGET_MAKE_ENV) $(TARGET_CONFIGURE_OPTS) \
		CFLAGS="$(TARGET_CFLAGS) -flto=auto" LDFLAGS="$(TARGET_LDFLAGS) $(MELKOR_MOLD_LDFLAGS) -flto=auto" \
		./configure --os=linux --sbindir=/sbin --sysconfdir=/etc \
		--runstatedir=/run --dbdir=/run/melkor-leases \
		--enable-privsep --privsepuser=dhcpcd --without-udev --without-libpcap
endef

define MELKOR_DHCPCD_BUILD_CMDS
	$(TARGET_MAKE_ENV) $(MAKE) -C $(@D)
endef

define MELKOR_DHCPCD_INSTALL_TARGET_CMDS
	$(INSTALL) -D -m 0755 $(@D)/src/dhcpcd $(TARGET_DIR)/sbin/dhcpcd
endef

define MELKOR_DHCPCD_USERS
	dhcpcd -1 dhcpcd -1 * /var/empty/dhcpcd - - DHCP
endef

$(eval $(generic-package))
