ifeq ($(BR2_MELKOR_LIVE_BUILTIN_FIRMWARE),y)

LINUX_DEPENDENCIES += linux-firmware wireless-regdb
LINUX_PRE_BUILD_HOOKS += MELKOR_LIVE_STAGE_FIRMWARE

define MELKOR_LIVE_STAGE_FIRMWARE
	$(INSTALL) -d $(@D)/melkor-firmware
	$(INSTALL) -m 0644 -t $(@D)/melkor-firmware \
		$(TARGET_DIR)/lib/firmware/iwlwifi-cc-a0-77.ucode \
		$(TARGET_DIR)/lib/firmware/regulatory.db \
		$(TARGET_DIR)/lib/firmware/regulatory.db.p7s
endef

endif
