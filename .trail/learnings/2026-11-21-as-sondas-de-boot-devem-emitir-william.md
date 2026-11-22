---
title: As sondas de boot devem emitir cada campo do console em uma linha completa
author: William
date: 2026-11-21
branch: codex/instalacao-pc
id: l-d043ee34
scope: repo
status: active
src: agent
touches:
  - docs/TESTES.md
  - scripts/test-boot.py
  - tests/test_image_tools.py
---

## Decided
Colete o valor antes de imprimir o campo completo com printf. O console serial é compartilhado pelo shell e pelos serviços de boot.
## Tried and ruled out
Imprimir o prefixo e executar awk ou cat depois abre uma janela para mensagens do dinit. No job M1 da CI, rauc-setup apareceu depois de M1-SLOT= e o valor A caiu na linha seguinte. O kernel já havia iniciado A corretamente.
## Watch out
Mantenha a comparação exata dos campos. Exercite o ruído concorrente no teste de regressão e confirme A e B com a mídia em QEMU.
In the code: scripts/test-boot.py, tests/test_image_tools.py.
