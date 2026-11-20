#!/usr/bin/env bash
set -euo pipefail

buildroot_release=2025.02.20
buildroot_commit=2f9129e7909bff6d78ae8cfda3d0e73503dfcac6
buildroot_url=https://gitlab.com/buildroot.org/buildroot.git

project=$(cd -- "$(dirname -- "$0")/.." && pwd)
output=$(realpath -m -- "${1:-out/m0}")
cache="$project/out/dl"

case "$output" in
    "$cache"|"$cache"/*|"$project"/out) echo "Escolha uma pasta de saída distinta do cache: $output" >&2; exit 2 ;;
    "$project"/*|/tmp/*) ;;
    *) echo "Escolha uma pasta de saída dentro do projeto ou de /tmp: $output" >&2; exit 2 ;;
esac

mkdir -p -- "$(dirname -- "$output")" "$cache"
if ! mkdir -- "$output"; then
    echo "Cada build precisa de uma nova pasta de saída. Use make image OUT=out/m0-2." >&2
    exit 2
fi

exec > >(tee "$output/build.log") 2>&1
source="$output/buildroot"
build="$output/build"

git init -q -- "$source"
git -C "$source" fetch --no-tags --depth 1 "$buildroot_url" "refs/tags/$buildroot_release"
actual_commit=$(git -C "$source" rev-parse 'FETCH_HEAD^{commit}')
if [[ "$actual_commit" != "$buildroot_commit" ]]; then
    echo "Buildroot $buildroot_release aponta para $actual_commit. Esperado: $buildroot_commit." >&2
    exit 1
fi
git -C "$source" -c advice.detachedHead=false checkout -q --detach "$buildroot_commit"

buildroot_make=(make --no-print-directory -C "$source" "O=$build" "BR2_EXTERNAL=$project/core" "BR2_DL_DIR=$cache")
"${buildroot_make[@]}" melkor_m0_defconfig
for option in BR2_TOOLCHAIN_BUILDROOT_MUSL BR2_GCC_VERSION_14_X BR2_INIT_NONE \
    BR2_PACKAGE_DINIT BR2_TARGET_GRUB2_X86_64_EFI BR2_TARGET_ROOTFS_SQUASHFS \
    BR2_DOWNLOAD_FORCE_CHECK_HASHES; do
    if ! grep -Fxq "$option=y" "$build/.config"; then
        echo "O Buildroot não ativou $option." >&2
        exit 1
    fi
done
"${buildroot_make[@]}"
"${buildroot_make[@]}" -s show-info > "$output/packages.json"

toolchain="$build/host/bin/x86_64-buildroot-linux-musl"
{
    "$toolchain-gcc" --version
    "$toolchain-ld" --version
    printf '\nInterpretador ELF do dinit\n'
    "$toolchain-readelf" -l "$build/target/sbin/dinit" | grep 'Requesting program interpreter'
} > "$output/toolchain.txt"

cd -- "$build/images"
sha256sum melkor-m0.img > "$output/SHA256SUMS"
printf 'Imagem gerada em %s/melkor-m0.img\n' "$PWD"
