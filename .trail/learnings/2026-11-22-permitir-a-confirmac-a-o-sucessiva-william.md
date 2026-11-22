---
title: Permitir a confirmação sucessiva de slots com o Limine
author: William
date: 2026-11-22
branch: codex/publica-desktop
id: l-61c59b3d
scope: repo
status: active
src: agent
touches:
  - ".github/workflows/m0.yml"
  - Makefile
  - README.md
  - apps/cmd/melkor-core-fetch/channel.go
  - apps/cmd/melkor-disco/main.go
  - apps/cmd/melkor-disco/main_test.go
  - apps/cmd/melkor-instalar/main.go
  - apps/cmd/melkor-instalar/main_test.go
  - apps/desktop/alpine-3.24-sdk.lock
  - apps/internal/pcdisk/image.go
---

## Decided
O RAUC agenda a candidata uma vez via Boot Loader Interface. A confirmação pelo dinit troca a seleção persistente. No efivarfs, tentar editar uma variável persistente já existente pode falhar por imutabilidade, mesmo como root.
## Tried and ruled out
Gravar a variável diretamente funcionou no primeiro A → B, quando ela ainda não existia, e falhou no teste de voltar manualmente ao A e confirmá-lo. Testar só a primeira promoção ocultava o problema.
## Watch out
Limite qualquer mudança de atributo à variável que o backend já validou e restaure a proteção mesmo se a etapa de escrita falhar. Confirme os dois sentidos no mesmo OVMF, preservando o arquivo de variáveis através dos reboots.
In the code: apps/internal/pcdisk/boot.go, tests/m1_update_vm.py, BusyBox chattr, LoaderEntryDefault, LoaderEntryOneShot.
