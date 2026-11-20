#!/usr/bin/env bash
# Prepara o ambiente de build do Melkor para funcionar sem internet.
#
#   scripts/prepare-env.sh           cria/atualiza o contêiner, o bundle do Buildroot e o cache de fontes
#   scripts/prepare-env.sh --check   só verifica se tudo está pronto, sem baixar nada (rede bloqueada)
#
# Pré-requisito: distrobox e podman no host. O primeiro uso precisa de rede; depois disso
# os comandos de build e de verificação rodam offline.
set -euo pipefail

mode=${1:-prepare}
case "$mode" in prepare|--check) ;; *) echo "Uso: $0 [--check]" >&2; exit 2 ;; esac

project=$(cd -- "$(dirname -- "$0")/.." && pwd)
container=${MELKOR_BUILD_CONTAINER:-melkor-build}
container_image=registry.fedoraproject.org/fedora-toolbox:45

buildroot_release=2025.02.20
buildroot_commit=2f9129e7909bff6d78ae8cfda3d0e73503dfcac6
buildroot_remote=https://gitlab.com/buildroot.org/buildroot.git
cache="$project/out/cache"
bundle="$cache/buildroot-$buildroot_release.bundle"
downloads="$project/out/dl"
prep="$cache/source-prep"

# --- Host: cria o contêiner se preciso e repassa a execução para dentro dele.
if [[ ! -e /run/.containerenv ]]; then
    command -v distrobox >/dev/null || { echo "Instale distrobox (e podman) no host." >&2; exit 2; }
    if ! distrobox list --no-color | awk -F'|' 'NR>1 {gsub(/ /, "", $2); print $2}' | grep -qx "$container"; then
        [[ "$mode" == "--check" ]] && { echo "Contêiner $container ausente. Rode scripts/prepare-env.sh." >&2; exit 1; }
        distrobox create --yes --name "$container" --image "$container_image"
    fi
    exec distrobox enter -n "$container" -- "$project/scripts/prepare-env.sh" "$mode"
fi

log() { printf '\n== %s\n' "$*"; }

# --- Contêiner: pacotes de build e teste.
log "Pacotes do contêiner"
mapfile -t packages < <(grep -v '^#' "$project/scripts/build-packages.fedora" | grep -v '^$')
if ! rpm -q --quiet "${packages[@]}"; then
    [[ "$mode" == "--check" ]] && { echo "Pacotes ausentes. Rode scripts/prepare-env.sh." >&2; exit 1; }
    sudo dnf install -y --setopt=install_weak_deps=False "${packages[@]}"
fi
for tool in patch makeinfo qemu-system-x86_64 mkfs.vfat mcopy python3 gcc; do
    printf '%-22s %s\n' "$tool" "$(command -v /usr/bin/"$tool" || echo AUSENTE)"
done

# --- Bundle do Buildroot fixado por commit (checado contra o commit esperado).
log "Bundle do Buildroot $buildroot_release"
mkdir -p "$cache"
if [[ ! -f "$bundle" ]]; then
    [[ "$mode" == "--check" ]] && { echo "Bundle ausente: $bundle" >&2; exit 1; }
    tmp_bare="$cache/buildroot-bare.tmp"
    rm -rf "$tmp_bare"
    git clone -q --bare --branch "$buildroot_release" "$buildroot_remote" "$tmp_bare"
    git -C "$tmp_bare" bundle create "$bundle.tmp" "refs/tags/$buildroot_release"
    rm -rf "$tmp_bare"
    mv -- "$bundle.tmp" "$bundle"
fi
check_repo="$cache/bundle-check.tmp"
rm -rf "$check_repo"; git init -q "$check_repo"
git -C "$check_repo" fetch --no-tags --depth 1 "$bundle" "refs/tags/$buildroot_release"
actual=$(git -C "$check_repo" rev-parse 'FETCH_HEAD^{commit}')
rm -rf "$check_repo"
[[ "$actual" == "$buildroot_commit" ]] || { echo "Bundle aponta para $actual; esperado $buildroot_commit." >&2; exit 1; }
echo "bundle OK: $bundle ($buildroot_commit)"

# --- Árvore do Buildroot usada só para baixar as fontes da configuração.
log "Fontes do alvo melkor_m0 em $downloads"
src="$prep/buildroot"
build="$prep/build"
if [[ ! -d "$src/.git" ]]; then
    [[ "$mode" == "--check" ]] && { echo "Árvore de preparo ausente: $src" >&2; exit 1; }
    git init -q "$src"
    git -C "$src" fetch --no-tags -q "$bundle" "refs/tags/$buildroot_release"
    git -C "$src" -c advice.detachedHead=false checkout -q --detach "$buildroot_commit"
fi
mkdir -p "$downloads"
make_br=(make --no-print-directory -C "$src" "O=$build" "BR2_EXTERNAL=$project/core" "BR2_DL_DIR=$downloads")
"${make_br[@]}" melkor_m0_defconfig >/dev/null
if [[ "$mode" != "--check" ]]; then
    "${make_br[@]}" source
fi

# --- Verificação offline: com o proxy apontando para uma porta fechada, qualquer download falha.
log "Verificação offline das fontes"
if env http_proxy=http://127.0.0.1:9 https_proxy=http://127.0.0.1:9 \
    HTTP_PROXY=http://127.0.0.1:9 HTTPS_PROXY=http://127.0.0.1:9 \
    "${make_br[@]}" source >"$cache/offline-check.log" 2>&1; then
    echo "todas as fontes estão no cache: $downloads"
else
    echo "Faltam fontes no cache. Veja $cache/offline-check.log. Rode scripts/prepare-env.sh com rede." >&2
    exit 1
fi

log "Ambiente pronto"
echo "Build:      make image        (ou scripts/build-image.sh out/m0 dentro do contêiner)"
echo "Teste boot: make test-boot OUT=out/m0"
