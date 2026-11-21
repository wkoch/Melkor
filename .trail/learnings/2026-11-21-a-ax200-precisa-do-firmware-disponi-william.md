---
title: A AX200 precisa do firmware disponível antes de abrir o console Live
author: William
date: 2026-11-21
branch: main
id: l-d098e81a
scope: repo
status: active
src: agent
touches:
  - core/Config.in
  - core/board/live/linux.fragment
  - core/configs/melkor_live_defconfig
  - docs/ARQUITETURA.md
  - docs/INSTALACAO.md
  - docs/MARCOS.md
  - docs/TESTES.md
  - scripts/test-boot.py
  - tests/test_image_tools.py
---

## Decided
Incorporar `iwlwifi-cc-a0-77.ucode` ao kernel Live. A consulta ao firmware embutido precede o acesso ao sistema de arquivos do initramfs. Habilitar PCI MSI e incluir a base regulatória assinada porque o mesmo boot relatou falhas nesses pontos.

## Tried and ruled out
A alpha.2 já trazia o firmware exato. Comparei os initramfs de ISO 9660 e de El Torito e vi o arquivo no console do QEMU após boot por Ventoy nos modos normal e GRUB2. Ainda assim, no HP, o `iwlwifi` retornou `-ENOENT` ao solicitar o arquivo. Adicionar outro arquivo com o mesmo nome ao initramfs não oferece garantia de que a busca no boot físico mude.

## Watch out
O Fedora Atomic COSMIC do mesmo HP criou `wlp1s0` com `iwlwifi` na placa `8086:2723`. A origem da falha de busca naquele boot do Live segue incerta. A prova da correção no hardware é `/usr/sbin/iw dev` mostrando a interface, com o log da AX200 se necessário.

In the code: `core/linux/linux-ext-melkor-live.mk`, `core/board/live/linux.fragment`, `core/configs/melkor_live_defconfig`, `docs/TESTES.md`.
