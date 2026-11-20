DNSPROXY_VERSION = 0.86.0
DNSPROXY_SITE = $(call github,AdguardTeam,dnsproxy,v$(DNSPROXY_VERSION))
DNSPROXY_GOMOD = github.com/AdguardTeam/dnsproxy
DNSPROXY_LICENSE = Apache-2.0
DNSPROXY_LICENSE_FILES = LICENSE
DNSPROXY_GO_ENV = CGO_ENABLED=0
DNSPROXY_LDFLAGS = -X github.com/AdguardTeam/golibs/version.version=v$(DNSPROXY_VERSION)

define DNSPROXY_USERS
	melkor-dns -1 melkor-dns -1 * - - - Loopback DNS
endef

$(eval $(golang-package))
