#!/bin/sh
set -eu

board_dir=$(dirname "$0")
stage=$(mktemp -d "$BUILD_DIR/melkor-ab.XXXXXX")
trap 'rm -rf -- "$stage" "$stage.tmp"' EXIT

install -D -m 0644 "$HOST_DIR/share/melkor-limine/BOOTX64.EFI" "$stage/EFI/BOOT/BOOTX64.EFI"
install -m 0644 "$board_dir/limine.conf" "$stage/EFI/BOOT/limine.conf"
install -m 0644 "$HOST_DIR/share/melkor-limine/LICENSE" "$HOST_DIR/share/melkor-limine/3RDPARTY.md" "$stage/EFI/BOOT/"
install -m 0644 "$BINARIES_DIR/bzImage" "$stage/EFI/BOOT/kernel-A"
install -m 0644 "$BINARIES_DIR/bzImage" "$stage/EFI/BOOT/kernel-B"
ln -s "$BINARIES_DIR/rootfs.squashfs" "$stage/rootfs.squashfs"

truncate -s 128M "$stage/persist.ext4"
# Mantenha o UUID em sincronia com melkor-persist e o teste do disco.
mke2fs -q -F -t ext4 -L melkor-data -U a04e841b-82ca-478c-b741-4314eb96ee30 \
    -E root_owner=0:0 "$stage/persist.ext4"

"$HOST_DIR/bin/genimage" --rootpath "$stage" --inputpath "$stage" \
    --outputpath "$BINARIES_DIR" --tmppath "$stage.tmp" --config "$board_dir/genimage.cfg"
