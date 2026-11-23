---
title: Pré-release de teste do PC não altera o canal estável até o HP validar
author: William
date: 2026-11-23
branch: codex/publica-desktop
cost: { minutes: 6, tokens: 208105 }
id: l-a0984481
scope: repo
status: active
src: agent
touches:
  - docs/TESTES.md
---

## Decided
Gerar uma pré-release nova a partir desta branch antes de testar no HP, e manter o canal pc-stable intocado até a build passar no notebook. A última release publicada (alpha.5-desktop) é anterior à troca do GRUB por Limine, então testar nela não valida o fluxo atual.

## Tried and ruled out
Usar a alpha.5 para o teste no HP foi descartado: o fluxo de boot e o instalador mudaram depois dela. Publicar direto no canal estável também foi descartado, porque os notebooks instalados seguem esse canal e a atualização não se desfaz facilmente.

## Watch out
As chaves oficiais de assinatura (RAUC e canal) ficam fora do checkout, na máquina de assinatura. Não copie para out/, para o repositório nem para a release; só o certificado público vai em core/board/pc/keys/. O canal só recebe a nova revisão depois do teste físico. In the code: Makefile (alvos image, bundle-pc, channel-pc) e docs/TESTES.md, seção Canal do desktop.
