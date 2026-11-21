---
title: O Ventoy precisa reconhecer a arquitetura da ISO e ler o initramfs cedo
author: William
date: 2026-11-21
branch: codex/instalacao-pc
id: l-98127c5f
scope: repo
status: active
src: agent
touches:
  - Makefile
  - README.md
  - apps/cmd/melkor-disco/main.go
  - apps/cmd/melkor-disco/main_test.go
  - apps/cmd/melkor-storage/main.go
  - apps/internal/pcdisk/image.go
  - apps/internal/pcdisk/image_test.go
  - apps/internal/pcdisk/layout.go
  - apps/internal/pcdisk/layout_test.go
  - apps/internal/pcdisk/storage.go
---

## Decided

Deixe o identificador x86_64 no rótulo ISO para que a mídia seja inicializável mesmo após renomear o arquivo. Dê prioridade aos blocos do initramfs, kernel e GRUB na ISO. Os dois modos do Ventoy 1.1.18 passaram em QEMU usando um arquivo dividido em dois trechos no pendrive virtual FAT32.

## Tried and ruled out

O rótulo anterior permitia que o Ventoy escolhesse um init de 32 bits quando o nome do arquivo era genérico. Apareceu erro de formato de executável em um kernel x86-64. Marcar a imagem como compatível desligava a injeção em modo normal, mas no modo GRUB2 o Ventoy ainda passava rdinit=/vtoy/vtoy. Com a arquitetura correta, mas com os arquivos na ordem antiga, o modo normal ainda falhou ao ler o fim do initramfs na imagem fragmentada. O modo GRUB2 e o boot como CD conseguiam ler a mesma ISO.

## Watch out

In the code: core/board/live/post-image.sh usa MELKOR_LIVE_x86_64 e pesos xorriso para colocar os arquivos lidos pelo GRUB antes da imagem El Torito. A ordenação cobre o caso reproduzido com o limite após cerca de 50 MiB. Ela não demonstra compatibilidade com todos os layouts FAT32. Verifique os modos normal e GRUB2 pela mídia final renomeada, além do boot como CD com OVMF.
