#!/bin/sh
set -eu

board_dir=$(dirname "$0")
stage=$(mktemp -d "$BUILD_DIR/melkor-ab.XXXXXX")
trap 'rm -rf -- "$stage" "$stage.tmp"' EXIT

install -D -m 0644 "$BINARIES_DIR/efi-part/EFI/BOOT/bootx64.efi" "$stage/EFI/BOOT/bootx64.efi"
install -m 0644 "$board_dir/grub.cfg" "$stage/EFI/BOOT/grub.cfg"
"$HOST_DIR/bin/grub-editenv" "$stage/EFI/BOOT/grubenv" create
"$HOST_DIR/bin/grub-editenv" "$stage/EFI/BOOT/grubenv" set 'ORDER=A B' A_OK=1 A_TRY=0 B_OK=0 B_TRY=0
ln -s "$BINARIES_DIR/rootfs.squashfs" "$stage/rootfs.squashfs"

truncate -s 128M "$stage/persist.ext4"
# Mantenha o UUID em sincronia com melkor-persist e o teste do disco.
mke2fs -q -F -t ext4 -L melkor-data -U a04e841b-82ca-478c-b741-4314eb96ee30 \
    -E root_owner=0:0 "$stage/persist.ext4"

"$HOST_DIR/bin/grub-script-check" "$stage/EFI/BOOT/grub.cfg"
"$HOST_DIR/bin/genimage" --rootpath "$stage" --inputpath "$stage" \
    --outputpath "$BINARIES_DIR" --tmppath "$stage.tmp" --config "$board_dir/genimage.cfg"
