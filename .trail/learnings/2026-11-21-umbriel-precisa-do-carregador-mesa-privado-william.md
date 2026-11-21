---
title: Umbriel precisa do carregador Mesa privado e de memória compartilhada
author: William
date: 2026-11-21
branch: codex/instalacao-pc
id: l-24a23c89
scope: repo
status: active
src: agent
touches:
  - Makefile
  - README.md
  - core/Config.in
  - core/board/pc/linux.fragment
  - core/board/pc/post-build.sh
  - core/board/pc/rootfs-overlay/etc/xdg/weston/weston.ini
  - core/board/pc/rootfs-overlay/usr/bin/melkor-desktop
  - core/board/pc/rootfs-overlay/usr/libexec/melkor-pc-persist
  - core/configs/melkor_pc_defconfig
  - core/external.mk
---

## Decided
A sessão Umbriel usa as bibliotecas Mesa da camada Alpine. Indique os diretórios de plugins DRI e GBM ao carregar o compositor. Monte /dev/shm no boot do PC para a tabela de formatos DMA-BUF.

## Tried and ruled out
Só LIBGL_DRIVERS_PATH não bastou. O Mesa tentou /usr/lib/gbm/dri_gbm.so da raiz Buildroot e a criação do EGL falhou. Com o caminho GBM correto, EGL e llvmpipe iniciaram, mas wlroots não abriu a memória compartilhada da tabela antes de existir /dev/shm.

## Watch out
In the code: `core/board/pc/rootfs-overlay/usr/bin/melkor-session-exec` define LIBGL_DRIVERS_PATH e GBM_BACKENDS_PATH. `core/board/pc/rootfs-overlay/usr/libexec/melkor-pc-persist` monta /dev/shm como tmpfs com modo 1777, nosuid, nodev e noexec. `tests/pc_desktop_vm.py` comprova Umbriel com Noctalia sobre bochs DRM e Mesa llvmpipe.
