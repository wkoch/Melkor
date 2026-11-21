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

mkdir -p "$stage/boot/grub"
install -m 0644 "$board_dir/grub.cfg" "$stage/boot/grub/grub.cfg"
"$HOST_DIR/bin/xorriso" -as mkisofs -R -V MELKOR_LIVE \
    -e boot/efi.img -no-emul-boot -follow-links \
    -o "$BINARIES_DIR/melkor-live.iso" -graft-points \
    "/=$stage" "boot/efi.img=$BINARIES_DIR/melkor-live-eltorito.vfat"
