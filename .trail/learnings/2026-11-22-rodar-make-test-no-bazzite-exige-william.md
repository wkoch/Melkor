---
title: "Rodar make test no Bazzite exige kcov do host, não o do contêiner"
author: William
date: 2026-11-22
branch: codex/publica-desktop
cost: { minutes: 8, tokens: 170956 }
id: l-1310b5ef
scope: repo
status: active
src: agent
touches:
  - README.md
  - docs/ARQUITETURA.md
  - docs/INSTALACAO.md
  - docs/MARCOS.md
  - docs/TESTES.md
  - apps/cmd/melkor-boot/main.go
  - apps/cmd/melkor-core-fetch/channel.go
  - apps/cmd/melkor-disco/main.go
  - apps/cmd/melkor-disco/main_test.go
  - apps/cmd/melkor-instalar/main.go
---

## Decided
Rodar a suíte de shell com um kcov extraído do RPM Fedora 45 para o host, e não por um atalho que chama kcov dentro de melkor-build. O host é imutável, mas o RPM extraído com rpm2cpio roda com as bibliotecas que já existem nele.

## Tried and ruled out
Um atalho que chamava `distrobox enter -n melkor-build -- kcov` fez três falhas em testes de shell (boot, RAUC e lançadores). O motivo: o PATH do teste tem stubs de `distrobox`, então o atalho chamava o stub, que não executava nada e deixava o log vazio. Depois de corrigir o caminho absoluto, ainda sobrou erro de ambiente dentro do contêiner (HOME trocado, podman pedindo criação de contêiner, /tmp não compartilhado com o host). Nenhum desses erros veio do código.

## Watch out
Se `make test` falhar com "not found in []" ou com saída vazia de wrappers de boot, confira primeiro o kcov e o PATH, antes de mexer nos scripts. In the code: tests/test_build_scripts.py (wrappers boot, rauc, lançadores) e tests/coverage_suite.py (main, variável KCOV). Comando que passou: `make test TEST_PYTHON=out/test-venv/bin/python KCOV=$PWD/out/host-tools/kcov`. Documentado em docs/TESTES.md, seção Cobertura do código próprio.
