#!/usr/bin/env bash
# Prepara o ambiente de build do Melkor para funcionar sem internet.
#
#   scripts/prepare-env.sh           cria/atualiza o contêiner, o bundle do Buildroot e o cache de fontes
#   scripts/prepare-env.sh --check   só verifica se tudo está pronto, sem baixar nada (rede bloqueada)
#
# Pré-requisito: distrobox e podman no host. O primeiro uso precisa de rede; depois disso
# os comandos de build e de verificação rodam offline.
set -euo pipefail

prepare_env() {
    local mode=$1 container_marker=$2 board=${3:-m0}
    case "$mode" in prepare|--check) ;; *) echo "Uso: $0 [--check]" >&2; exit 2 ;; esac
    case "$board" in m0|m1|live|pc) ;; *) echo "Alvo desconhecido: $board. Use BOARD=m0, m1, live ou pc." >&2; exit 2 ;; esac

    project=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
    container=${MELKOR_BUILD_CONTAINER:-melkor-build}
    container_image=registry.fedoraproject.org/fedora-toolbox:45

    buildroot_release=2025.02.20
    buildroot_commit=2f9129e7909bff6d78ae8cfda3d0e73503dfcac6
    buildroot_remote=https://gitlab.com/buildroot.org/buildroot.git
    cache="$project/out/cache"
    bundle="$cache/buildroot-$buildroot_release.bundle"
    downloads="$project/out/dl"

    # --- Host: cria o contêiner se preciso e repassa a execução para dentro dele.
    if [[ ! -e "$container_marker" ]]; then
        command -v distrobox >/dev/null || { echo "Instale distrobox (e podman) no host." >&2; exit 2; }
        if ! containers=$(distrobox list --no-color); then
            echo "Não foi possível listar os contêineres do Distrobox." >&2
            exit 1
        fi
        if ! awk -F'|' -v name="$container" '
            NR > 1 { gsub(/ /, "", $2); found = found || ($2 == name) }
            END { exit !found }
        ' <<< "$containers"; then
            [[ "$mode" == "--check" ]] && { echo "Contêiner $container ausente. Rode scripts/prepare-env.sh." >&2; exit 1; }
            distrobox create --yes --name "$container" --image "$container_image"
        fi
        exec distrobox enter -n "$container" -- "$project/scripts/prepare-env.sh" "$mode" "$board"
    fi

    log() { printf '\n== %s\n' "$*"; }

    # --- Contêiner: pacotes de build e teste.
    log "Pacotes do contêiner"
    mapfile -t packages < <(grep -v '^#' "$project/scripts/build-packages.fedora" | grep -v '^$')
    if ! rpm -q --quiet --whatprovides "${packages[@]}"; then
        if [[ "$mode" == "--check" ]]; then
            for package in "${packages[@]}"; do
                rpm -q --quiet --whatprovides "$package" || printf 'Requisito ausente: %s\n' "$package" >&2
            done
            echo "Rode make env para instalar os requisitos." >&2
            exit 1
        fi
        sudo dnf install -y --setopt=install_weak_deps=False "${packages[@]}"
    fi
    for tool in patch makeinfo qemu-system-x86_64 mkfs.vfat mke2fs mcopy python3 gcc tcpdump; do
        printf '%-22s %s\n' "$tool" "$(command -v /usr/bin/"$tool" || echo AUSENTE)"
    done

    # --- Bundle do Buildroot fixado por commit (checado contra o commit esperado).
    log "Bundle do Buildroot $buildroot_release"
    mkdir -p "$cache"
    scratch=$(mktemp -d "$cache/prepare.XXXXXX")
    trap 'rm -rf -- "$scratch"' EXIT
    candidate="$bundle"
    if [[ ! -f "$bundle" ]]; then
        [[ "$mode" == "--check" ]] && { echo "Bundle ausente: $bundle" >&2; exit 1; }
        tmp_bare="$scratch/bare"
        candidate="$scratch/buildroot.bundle"
        git clone -q --bare --branch "$buildroot_release" "$buildroot_remote" "$tmp_bare"
        git -C "$tmp_bare" bundle create "$candidate" "refs/tags/$buildroot_release"
    fi
    src="$scratch/buildroot"
    git init -q "$src"
    git -C "$src" fetch --no-tags "$candidate" "refs/tags/$buildroot_release"
    actual=$(git -C "$src" rev-parse 'FETCH_HEAD^{commit}')
    [[ "$actual" == "$buildroot_commit" ]] || { echo "Bundle aponta para $actual; esperado $buildroot_commit." >&2; exit 1; }
    if [[ "$candidate" != "$bundle" ]]; then
        mv -- "$candidate" "$bundle"
    fi
    echo "bundle OK: $bundle ($buildroot_commit)"

    # --- Árvore do Buildroot usada só para baixar as fontes da configuração.
    log "Fontes do alvo melkor_$board em $downloads"
    build="$scratch/build"
    git -C "$src" -c advice.detachedHead=false checkout -q --detach "$buildroot_commit"
    mkdir -p "$downloads"
    make_br=(make --no-print-directory -C "$src" "O=$build" "BR2_EXTERNAL=$project/core" "BR2_DL_DIR=$downloads")
    "${make_br[@]}" "melkor_${board}_defconfig" >/dev/null
    # go mod vendor só usa o Go do host. A toolchain do alvo fica para o build.
    fetch_sources=("${make_br[@]}" HOST_GO_DEPENDENCIES_CGO= HOST_GO_CGO_ENABLED=0 source)
    if [[ "$mode" != "--check" ]]; then
        "${fetch_sources[@]}"
    fi

    # --- Verificação offline: com o proxy apontando para uma porta fechada, qualquer download falha.
    log "Verificação offline das fontes"
    offline_status=0
    env no_proxy= NO_PROXY= http_proxy=http://127.0.0.1:9 https_proxy=http://127.0.0.1:9 \
        HTTP_PROXY=http://127.0.0.1:9 HTTPS_PROXY=http://127.0.0.1:9 \
        "${fetch_sources[@]}" >"$scratch/offline-check.log" 2>&1 || offline_status=$?
    mv -- "$scratch/offline-check.log" "$cache/offline-check.log"
    if (( offline_status == 0 )); then
        echo "todas as fontes estão no cache: $downloads"
    else
        echo "Faltam fontes no cache. Veja $cache/offline-check.log. Rode scripts/prepare-env.sh com rede." >&2
        exit 1
    fi

    log "Ambiente pronto"
    echo "Build:      make image BOARD=$board"
    echo "Teste boot: make test-boot BOARD=$board"
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
    prepare_env "${1:-prepare}" /run/.containerenv "${2:-m0}"
fi
