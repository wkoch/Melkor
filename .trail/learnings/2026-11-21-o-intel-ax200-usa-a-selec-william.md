---
title: O Intel AX200 usa a seleção de firmware chamada 22260
author: William
date: 2026-11-21
branch: main
id: l-bbc478ba
scope: repo
status: active
src: agent
touches:
  - ".github/workflows/m0.yml"
  - Makefile
  - README.md
  - core/board/live/genimage.cfg
  - core/board/live/grub.cfg
  - core/board/live/linux.fragment
  - core/board/live/post-image.sh
  - core/board/live/rootfs-overlay/etc/dinit.d/boot
  - core/board/live/rootfs-overlay/etc/dinit.d/console-local
  - core/board/live/rootfs-overlay/etc/motd
---

## Decided
Na mídia do HP ProBook, usar a seleção de firmware `IWLWIFI_22260` do Buildroot fixado. O driver Linux para o AX200 procura arquivos `iwlwifi-cc-a0-*.ucode`, que essa seleção instala.

## Tried and ruled out
A seleção `IWLWIFI_22000` parece adequada pelo nome da família do driver. A receita instala apenas arquivos com prefixos `Qu` e `QuZ` nessa opção, então a mantivemos fora da configuração de diagnóstico.

## Watch out
A família do HP teve mais de uma placa de rede. A presença dos arquivos na imagem confirma o empacotamento, e a unidade física é que confirma se o driver inicia.

In the code: `core/configs/melkor_live_defconfig`; Buildroot 2025.02.20 `package/linux-firmware/linux-firmware.mk`; Linux 6.12.117 `drivers/net/wireless/intel/iwlwifi/cfg/22000.c`.
