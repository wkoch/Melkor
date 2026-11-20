---
title: Verificar o cache de um pacote Go dispensa a toolchain C do alvo
author: William
date: 2026-11-20
branch: main
id: l-13562668
scope: repo
status: active
src: agent
touches:
  - ".github/workflows/m0.yml"
  - AGENTS.md
  - Makefile
  - README.md
  - core/Config.in
  - core/external.mk
  - docs/ARQUITETURA.md
  - docs/MARCOS.md
  - docs/TESTES.md
  - scripts/build-image.sh
---

## Decided
Para preparar ou verificar as fontes, deixe o Buildroot usar o Go fixado do host. O empacotamento via go mod vendor e a verificação dos hashes podem terminar sem compilar para o alvo.

## Tried and ruled out
Com as dependências padrão do Go, make source iniciou uma compilação inteira do GCC em um diretório descartável. A dependência vinha do suporte a CGO na arquitetura do alvo, mesmo com o resolvedor configurado para CGO_ENABLED=0.

## Watch out
Aplique o ajuste apenas à chamada source. O build da imagem conserva a resolução normal das dependências. O hash do pacote Go cobre o tarball reempacotado pelo Buildroot com vendor, e não o tar.gz original do GitHub.

In the code: scripts/prepare-env.sh, HOST_GO_DEPENDENCIES_CGO=, HOST_GO_CGO_ENABLED=0, core/package/dnsproxy/dnsproxy.hash.
