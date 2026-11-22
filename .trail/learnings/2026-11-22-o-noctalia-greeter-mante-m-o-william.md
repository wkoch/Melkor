---
title: O Noctalia Greeter mantém o Cage para preservar o console de recuperação
author: William
date: 2026-11-22
branch: codex/noctalia-greeter
id: l-c7ad6a5e
scope: repo
status: active
src: agent
touches:
  - README.md
  - apps/desktop/alpine-3.24-sdk.lock
  - core/board/pc/graphical-login/usr/libexec/melkor-greeter
  - core/board/pc/post-build.sh
  - core/board/pc/rootfs-overlay/usr/bin/melkor-session-exec
  - docs/ARQUITETURA.md
  - docs/TESTES.md
  - scripts/pc-desktop-layer.py
  - tests/pc_desktop_vm.py
---

## Decided

Use o cliente Wayland do Noctalia Greeter para a tela de entrada, com Cage atendendo o VT1. O greetd e a pilha PAM continuam responsáveis pela conta local; o VT2 continua acessível pelo atalho da tela de login.

## Tried and ruled out

O compositor distribuído no Noctalia Greeter 1.7.1 iniciava a sessão, mas em algumas execuções de QEMU com seatd caiu em libwlroots ao pressionar Ctrl+Alt+F2 logo depois de encerrar Umbriel. O greetd reiniciou e voltou ao VT1 antes que o usuário pudesse usar o console. Com Cage, login e 11 idas e voltas pelo console de recuperação passaram na mesma VM. Prefira esse caminho até revalidar a transição de VT com outra versão do compositor.

## Watch out

In the code: preserve o ambiente privado e o D-Bus de sessão em core/board/pc/graphical-login/usr/libexec/melkor-greeter. O teste tests/pc_desktop_vm.py cobre senha inválida, login, encerramento de sessão e os atalhos de VT. A compilação paralela da versão 1.7.1 também precisa declarar os cabeçalhos Wayland gerados como entradas do cliente no Meson.
