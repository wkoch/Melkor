#!/bin/sh
set -eu

board_dir=$(dirname "$0")
test -s "$BINARIES_DIR/efi-part/EFI/BOOT/bootx64.efi"
# O genimage resolve os itens de "files" e "image" em --inputpath, não em --rootpath.
# Por isso o conteúdo do ESP e o rootfs são copiados para uma pasta de entrada própria.
stage=$(mktemp -d "$BUILD_DIR/melkor-stage.XXXXXX")
trap 'rm -rf -- "$stage" "$stage.tmp"' EXIT

cp -a "$BINARIES_DIR/efi-part/EFI" "$stage/EFI"
cp "$board_dir/grub.cfg" "$stage/EFI/BOOT/grub.cfg"
cp "$BINARIES_DIR/bzImage" "$stage/bzImage"
cp "$BINARIES_DIR/rootfs.squashfs" "$stage/rootfs.squashfs"
"$HOST_DIR/bin/genimage" --rootpath "$stage" --inputpath "$stage" \
    --outputpath "$BINARIES_DIR" --tmppath "$stage.tmp" --config "$board_dir/genimage.cfg"
