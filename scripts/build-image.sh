#!/usr/bin/env bash
set -euo pipefail

buildroot_release=2025.02.20
buildroot_commit=2f9129e7909bff6d78ae8cfda3d0e73503dfcac6
buildroot_remote=https://gitlab.com/buildroot.org/buildroot.git

project=$(cd -- "$(dirname -- "$0")/.." && pwd)
board=${2:-m0}
case "$board" in
    m0|m1|pc) rootfs_option=BR2_TARGET_ROOTFS_SQUASHFS ;;
    live|installer) rootfs_option=BR2_TARGET_ROOTFS_CPIO_GZIP ;;
    *) echo "Alvo desconhecido: $board. Use BOARD=m0, m1, live, pc ou installer." >&2; exit 2 ;;
esac
if [[ ${MELKOR_PC_MINIMAL:-0} == 1 ]] &&
   [[ "$board" != pc || -n "${MELKOR_PC_DESKTOP_LAYER:-}${MELKOR_PC_CHROMIUM_LAYER:-}${MELKOR_PC_SESSION_LAYER:-}" ]]; then
    echo "MELKOR_PC_MINIMAL=1 exige BOARD=pc e camadas de desktop desabilitadas." >&2
    exit 2
fi
output=$(realpath -m -- "${1:-out/$board}")
cache="$project/out/dl"
# Sem rede, use o bundle criado por scripts/prepare-env.sh. MELKOR_BUILDROOT_URL substitui a origem.
buildroot_bundle="$project/out/cache/buildroot-$buildroot_release.bundle"
if [[ -n "${MELKOR_BUILDROOT_URL:-}" ]]; then
    buildroot_url=$MELKOR_BUILDROOT_URL
elif [[ -f "$buildroot_bundle" ]]; then
    buildroot_url=$buildroot_bundle
else
    buildroot_url=$buildroot_remote
fi

case "$output" in
    "$cache"|"$cache"/*|"$project"/out) echo "Escolha uma pasta de saída distinta do cache: $output" >&2; exit 2 ;;
    "$project"/*|/tmp/*) ;;
    *) echo "Escolha uma pasta de saída dentro do projeto ou de /tmp: $output" >&2; exit 2 ;;
esac

mkdir -p -- "$(dirname -- "$output")" "$cache"
if ! mkdir -- "$output"; then
    echo "Cada build precisa de uma nova pasta de saída. Use make image BOARD=$board OUT=out/$board-2." >&2
    exit 2
fi

exec > >(tee "$output/build.log") 2>&1
source="$output/buildroot"
build="$output/build"

git init -q -- "$source"
git -C "$source" fetch --no-tags --depth 1 -- "$buildroot_url" "refs/tags/$buildroot_release"
actual_commit=$(git -C "$source" rev-parse 'FETCH_HEAD^{commit}')
if [[ "$actual_commit" != "$buildroot_commit" ]]; then
    echo "Buildroot $buildroot_release aponta para $actual_commit. Esperado: $buildroot_commit." >&2
    exit 1
fi
git -C "$source" -c advice.detachedHead=false checkout -q --detach "$buildroot_commit"

buildroot_make=(make --no-print-directory -C "$source" "O=$build" "BR2_EXTERNAL=$project/core" "BR2_DL_DIR=$cache")
if [[ ${MELKOR_PC_MINIMAL:-0} == 1 ]]; then
    sed -E '/^BR2_PACKAGE_(SEATD|SEATD_DAEMON|WESTON|WESTON_DEFAULT_DRM|FOOT|MELKOR_NUSHELL|BUBBLEWRAP|DEJAVU)=/d' \
        "$project/core/configs/melkor_pc_defconfig" > "$output/pc-minimal.defconfig"
    "${buildroot_make[@]}" "BR2_DEFCONFIG=$output/pc-minimal.defconfig" defconfig
else
    "${buildroot_make[@]}" "melkor_${board}_defconfig"
fi
for option in BR2_TOOLCHAIN_BUILDROOT_MUSL BR2_GCC_VERSION_14_X BR2_INIT_NONE \
    BR2_PACKAGE_DINIT BR2_TARGET_GRUB2_X86_64_EFI "$rootfs_option" \
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
images=("melkor-$board.img")
case "$board" in
    live) images+=(melkor-live.iso) ;;
    pc) images=(rootfs.squashfs bzImage efi-part/EFI/BOOT/bootx64.efi) ;;
    installer) images=(rootfs.cpio.gz bzImage) ;;
esac
sha256sum "${images[@]}" > "$output/SHA256SUMS"
printf 'Imagem gerada em %s\n' "${images[@]/#/$PWD/}"
