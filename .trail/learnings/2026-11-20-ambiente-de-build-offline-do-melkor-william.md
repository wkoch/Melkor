---
title: "Ambiente de build offline do Melkor (distrobox melkor-build, cache em out/) e como o…"
author: William
date: 2026-11-20
id: l-7a6a8be9
scope: repo
status: active
touches:
  - scripts/prepare-env.sh
---

## Decided
O host é Bazzite sem sudo sem senha. As ferramentas de build ficam no contêiner distrobox `melkor-build` (Fedora 45), criado por `make env` (`scripts/prepare-env.sh`). O cache offline fica em `out/cache/` (bundle do Buildroot 2025.02.20, commit 2f9129e…) e `out/dl/` (fontes do alvo). Ambos estão no `.gitignore`: não apagar.

**Why:** o Codex que programa o projeto não tem acesso à internet, então precisa de tudo já baixado e verificado localmente.

## Watch out
para verificar, use `make check-env` (roda com a rede bloqueada). Para build e teste de boot, use `make image OUT=out/<nova>` e `make test-boot OUT=out/<nova>`; o Makefile já entra no contêiner. Cada build exige uma pasta OUT nova. Ver a seção "Ambiente de build sem internet" do README.

_From Claude Code's memory of this repo, last updated Nov 20._
