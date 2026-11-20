#!/bin/sh
set -eu

board_dir=$(dirname "$0")
test -s "$BINARIES_DIR/efi-part/EFI/BOOT/bootx64.efi"
esp_dir=$(mktemp -d "$BUILD_DIR/melkor-esp.XXXXXX")
trap 'rm -rf -- "$esp_dir" "$esp_dir.tmp"' EXIT

cp -a "$BINARIES_DIR/efi-part/EFI" "$esp_dir/EFI"
cp "$board_dir/grub.cfg" "$esp_dir/EFI/BOOT/grub.cfg"
cp "$BINARIES_DIR/bzImage" "$esp_dir/bzImage"
"$HOST_DIR/bin/genimage" --rootpath "$esp_dir" --inputpath "$BINARIES_DIR" \
    --outputpath "$BINARIES_DIR" --tmppath "$esp_dir.tmp" --config "$board_dir/genimage.cfg"
