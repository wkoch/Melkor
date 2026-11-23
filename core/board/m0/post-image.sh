#!/bin/sh
set -eu

board_dir=$(dirname "$0")
test -s "$HOST_DIR/share/melkor-limine/BOOTX64.EFI"
# O genimage resolve os itens de "files" e "image" em --inputpath, não em --rootpath.
# Por isso o conteúdo do ESP e o rootfs são copiados para uma pasta de entrada própria.
stage=$(mktemp -d "$BUILD_DIR/melkor-stage.XXXXXX")
trap 'rm -rf -- "$stage" "$stage.tmp"' EXIT

install -D -m 0644 "$HOST_DIR/share/melkor-limine/BOOTX64.EFI" "$stage/EFI/BOOT/BOOTX64.EFI"
install -m 0644 "$board_dir/limine.conf" "$stage/EFI/BOOT/limine.conf"
install -m 0644 "$HOST_DIR/share/melkor-limine/LICENSE" "$HOST_DIR/share/melkor-limine/3RDPARTY.md" "$stage/EFI/BOOT/"
cp "$BINARIES_DIR/bzImage" "$stage/bzImage"
cp "$BINARIES_DIR/rootfs.squashfs" "$stage/rootfs.squashfs"
"$HOST_DIR/bin/genimage" --rootpath "$stage" --inputpath "$stage" \
    --outputpath "$BINARIES_DIR" --tmppath "$stage.tmp" --config "$board_dir/genimage.cfg"
