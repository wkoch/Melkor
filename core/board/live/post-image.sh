#!/bin/sh
set -eu

board_dir=$(dirname "$0")
stage=$(mktemp -d "$BUILD_DIR/melkor-live.XXXXXX")
trap 'rm -rf -- "$stage" "$stage.tmp"' EXIT

install -D -m 0644 "$BINARIES_DIR/efi-part/EFI/BOOT/bootx64.efi" "$stage/EFI/BOOT/bootx64.efi"
install -m 0644 "$board_dir/grub.cfg" "$stage/EFI/BOOT/grub.cfg"
ln -s "$BINARIES_DIR/bzImage" "$stage/bzImage"
ln -s "$BINARIES_DIR/rootfs.cpio.gz" "$stage/rootfs.cpio.gz"
test -s "$stage/bzImage"
test -s "$stage/rootfs.cpio.gz"

"$HOST_DIR/bin/grub-script-check" "$stage/EFI/BOOT/grub.cfg"
"$HOST_DIR/bin/genimage" --rootpath "$stage" --inputpath "$stage" \
    --outputpath "$BINARIES_DIR" --tmppath "$stage.tmp" --config "$board_dir/genimage.cfg"
