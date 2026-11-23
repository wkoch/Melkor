---
title: "Preservar a fortificação do SDK quando o LTO falha com std::to_string"
author: William
date: 2026-11-22
branch: codex/publica-desktop
id: l-3b6f9ddd
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
Mantenha LTO no dinit, dhcpcd, Umbriel e Noctalia Greeter. Use a configuração de fortificação do SDK Alpine com exceção de LTO no Noctalia Shell e no Luciole.
## Tried and ruled out
GCC 15.2 falhou no link dos dois programas em __to_xstring.constprop, reportando que o corpo always_inline de vsnprintf em fortify/stdio.h poderia ser substituído na linkedição. -fno-semantic-interposition sozinho também reproduziu o erro no Shell.
## Watch out
A etapa de compilação termina antes desse erro. Valide a linkedição completa reativando LTO quando trocar o SDK, os headers de fortificação ou o compilador. O kernel segue com a toolchain Buildroot e seus flags.
In the code: scripts/pc-desktop-layer.py, make pc-session-layer, Alpine 3.24, Mold 2.39.1.
