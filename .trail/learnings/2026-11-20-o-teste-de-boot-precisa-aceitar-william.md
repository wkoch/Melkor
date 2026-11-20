---
title: O teste de boot precisa aceitar o menu do GRUB sem contagem
author: William
date: 2026-11-20
branch: main
id: l-4af5b348
scope: repo
status: active
src: agent
touches:
  - ".github/workflows/m0.yml"
  - README.md
  - docs/MARCOS.md
  - scripts/test-boot.py
  - tests/test_image_tools.py
---

## Decided
Usar a entrada destacada no menu para escolher o slot B. Aguardar a contagem por até um segundo antes de enviar teclas dá tempo para o desenho do menu terminar, mesmo quando o GRUB aguarda confirmação manual.

## Tried and ruled out
Aguardar sempre a frase da contagem fez o teste expirar em QEMU com TCG. Enviar a seta logo após o primeiro destaque perdeu a tecla em KVM e deixou o slot A iniciar.

## Watch out
O teste de persistência reinicia rapidamente após o login de A. Se o serviço de confirmação do slot ainda não concluiu, o menu pode surgir sem contagem. O teste precisa confirmar o destaque em B antes de enviar Enter.

In the code: scripts/test-boot.py, tests/test_image_tools.py. `make test-boot BOARD=m1 OUT=out/v0.1.0-alpha.1-build` passou com KVM e com `QEMU_ACCEL=tcg`.
