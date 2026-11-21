---
title: A imagem temporária 4Kn precisa apresentar seu setor lógico ao particionador
author: William
date: 2026-11-21
branch: codex/instalacao-pc
id: l-6b6875c3
scope: repo
status: active
src: agent
touches:
  - Makefile
  - README.md
  - apps/internal/pcdisk/image.go
  - apps/internal/pcdisk/layout.go
  - core/Config.in
  - core/external.mk
  - docs/ARQUITETURA.md
  - docs/INSTALACAO.md
  - docs/TESTES.md
  - scripts/build-image.sh
---

## Decided

Prepare a GPT em um loop com setor lógico de 4096 bytes quando o destino for 4Kn. Formate a ESP com o mesmo setor e reserve 512 MiB para FAT32.

## Tried and ruled out

O sfdisk recente do host aceita --sector-size em arquivos. O util-linux 2.40.4 do instalador recusou essa opção durante a preparação, antes de abrir o disco de destino. O arquivo cru comum aparece com setores de 512 bytes.

## Watch out

In the code: partitionImage em apps/internal/pcdisk/image.go usa losetup apenas para 4Kn, fecha o loop após o sfdisk e propaga falha de desmontagem. O teste em tests/installer_vm.py anexa um NVMe real para o convidado com setor de 4096 bytes, reinicia pelo próprio disco e verifica ambos os slots.
