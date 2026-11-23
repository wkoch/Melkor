---
title: Build de desktop e instalador usam OUT separados e revisões diferentes
author: William
date: 2026-11-23
branch: codex/publica-desktop
cost: { minutes: 7, tokens: 12041 }
id: l-68f320b5
scope: repo
status: active
src: agent
touches:
  - docs/TESTES.md
  - scripts/installer-iso.py
---

## Decided
Gerar duas imagens PC separadas: uma mínima com revisão 1, que alimenta o instalador, e uma de desktop com revisão 2, que gera o bundle assinado. Cada uma em um OUT próprio. A revisão 2 é o que o cliente instalado recebe pelo canal, então o instalador precisa sair com a revisão 1 para o fluxo de atualização funcionar.

## Tried and ruled out
Usar uma única imagem de desktop para o instalador foi descartado: o instalador ficaria com a revisão 2 e a atualização não teria o que mostrar. Reutilizar o mesmo OUT para as duas builds também foi descartado: o diretório de build do Buildroot é compartilhado e as duas corrompem o estado uma da outra.

## Watch out
Ao passar as camadas do desktop, MELKOR_PC_MINIMAL precisa ficar vazio, senão o build cai no modo mínimo sem avisar. Com as chaves oficiais, o RAUC usa docs/TESTES.md e core/board/pc/keys/. In the code: Makefile (alvo image, variáveis MELKOR_CORE_REVISION e MELKOR_PC_MINIMAL), scripts/installer-iso.py.
