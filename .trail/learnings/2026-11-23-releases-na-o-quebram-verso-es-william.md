---
title: Releases não quebram versões anteriores sem concordância explícita do mantenedor
author: William
date: 2026-11-23
branch: codex/publica-desktop
cost: { minutes: 1, tokens: 227844 }
id: l-f62466ba
scope: repo
status: active
src: agent
touches:
  - AGENTS.md
  - docs/ARQUITETURA.md
---

## Decided
Nenhuma release nova pode quebrar instalações anteriores em uso. Uma quebra só entra com concordância explícita do mantenedor, e a versão indica a incompatibilidade pelo SemVer, com incremento de versão maior. Antes de publicar, confirmar que a versão anterior ainda recebe a atualização pelo canal sem intervenção manual.

## Tried and ruled out
Publicar a troca de GRUB para Limine como uma pré-release comum foi descartado: instalações alpha.5 baseadas em GRUB não recebem o canal novo sem migração ou reinstalação. Isso é quebra e precisa de aprovação antes da publicação.

## Watch out
Migrações de boot exigem decisão explícita antes de publicar, mesmo quando o teste em QEMU passa. In the code: AGENTS.md (seção Compatibilidade das releases) e docs/ARQUITETURA.md (seção Compatibilidade das releases).
