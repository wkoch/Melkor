---
title: Regra de compatibilidade das releases vale só para versões finais
author: William
date: 2026-11-23
branch: codex/publica-desktop
cost: { minutes: 1, tokens: 2439 }
id: l-94d64fac
scope: repo
status: active
replaces: l-f62466ba
src: agent
touches:
  - AGENTS.md
  - docs/ARQUITETURA.md
---

## Decided
A regra de não quebrar versões anteriores em uso vale só para versões finais (1.0, 1.1, 1.1.1). Versões alpha e beta podem quebrar instalações de teste. Uma quebra em versão final ainda exige concordância explícita do mantenedor e incremento de versão maior pelo SemVer.

## Tried and ruled out
Aplicar a regra também às pré-releases travava a publicação de alpha.6 sem necessidade, porque a troca de boot para Limine só afeta quem testa.

## Watch out
Migrações de boot continuam exigindo decisão explícita antes de qualquer versão final. In the code: AGENTS.md (seção Compatibilidade das releases) e docs/ARQUITETURA.md (seção Compatibilidade das releases).
