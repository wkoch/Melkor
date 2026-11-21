---
title: A imagem EFI da ISO precisa ter um tamanho explícito no catálogo
author: William
date: 2026-11-21
branch: main
id: l-dad84fff
scope: repo
status: active
src: agent
touches:
  - ".github/workflows/m0.yml"
  - README.md
  - core/board/live/genimage.cfg
  - core/board/live/post-image.sh
  - core/configs/melkor_live_defconfig
  - docs/ARQUITETURA.md
  - docs/INSTALACAO.md
  - docs/MARCOS.md
  - docs/TESTES.md
  - scripts/build-image.sh
---

## Decided
Gerar uma imagem FAT16 de 30 MiB para o El Torito, separada da ESP da mídia USB bruta, pois o campo de tamanho do catálogo só comporta 65535 setores de 512 bytes. Confirmamos o valor 61440 e iniciamos a ISO pelo Ventoy 1.1.18 nos modos normal e GRUB2.

## Tried and ruled out
Reaproveitar a ESP FAT32 de 64 MiB produziu o valor zero no catálogo e um aviso do xorriso. O firmware OVMF iniciou essa variante, mas ela depende da interpretação especial do campo pelo carregador.

## Watch out
Se o kernel e o initramfs não couberem na imagem de 30 MiB, reavalie onde o GRUB vai carregar os arquivos. `xorriso -report_el_torito plain` mostra o tamanho gravado. Repita o boot pelo menu do Ventoy quando modificar o empacotamento.

In the code: `core/board/live/genimage.cfg`, `core/board/live/post-image.sh`, `docs/TESTES.md`.
