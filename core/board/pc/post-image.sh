#!/bin/sh
set -eu

install -D -m 0644 "$HOST_DIR/share/melkor-limine/BOOTX64.EFI" "$BINARIES_DIR/efi-part/EFI/BOOT/bootx64.efi"
