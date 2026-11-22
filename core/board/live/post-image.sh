#!/bin/sh
set -eu

board_dir=$(dirname "$0")
stage=$(mktemp -d "$BUILD_DIR/melkor-live.XXXXXX")
trap 'rm -rf -- "$stage" "$stage.tmp"' EXIT

limine="$HOST_DIR/share/melkor-limine"
install -D -m 0644 "$limine/BOOTX64.EFI" "$stage/EFI/BOOT/BOOTX64.EFI"
install -m 0644 "$board_dir/limine.conf" "$stage/EFI/BOOT/limine.conf"
install -m 0644 "$limine/LICENSE" "$limine/3RDPARTY.md" "$stage/EFI/BOOT/"
ln -s "$BINARIES_DIR/bzImage" "$stage/bzImage"
ln -s "$BINARIES_DIR/rootfs.cpio.gz" "$stage/rootfs.cpio.gz"
test -s "$stage/bzImage"
test -s "$stage/rootfs.cpio.gz"

"$HOST_DIR/bin/genimage" --rootpath "$stage" --inputpath "$stage" \
    --outputpath "$BINARIES_DIR" --tmppath "$stage.tmp" --config "$board_dir/genimage.cfg"

# O Ventoy consulta o rótulo para escolher a arquitetura do init que injeta.
# Mantenha os arquivos de boot no começo da ISO.
"$HOST_DIR/bin/xorriso" -as mkisofs -R -V MELKOR_LIVE_x86_64 \
    --sort-weight 10 /rootfs.cpio.gz --sort-weight 9 /bzImage \
    --sort-weight 8 /EFI/BOOT/BOOTX64.EFI --sort-weight 7 /EFI/BOOT/limine.conf \
    -e boot/efi.img -no-emul-boot -follow-links \
    -o "$BINARIES_DIR/melkor-live.iso" -graft-points \
    "/=$stage" "boot/efi.img=$BINARIES_DIR/melkor-live-eltorito.vfat"
