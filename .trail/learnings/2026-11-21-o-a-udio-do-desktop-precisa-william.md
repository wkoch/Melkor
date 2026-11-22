---
title: O áudio do desktop precisa dos perfis ALSA privados e de uma regra Pulse que sempre corresponda
author: William
date: 2026-11-21
branch: codex/instalacao-pc
id: l-b41d357a
scope: repo
status: active
src: agent
touches:
  - Makefile
  - README.md
  - apps/desktop/alpine-3.24.lock
  - apps/desktop/debian-13.lock
  - core/board/pc/linux.fragment
  - core/board/pc/post-build.sh
  - core/board/pc/rootfs-overlay/etc/xdg/melkor/blanc.desktop
  - core/board/pc/rootfs-overlay/etc/xdg/melkor/brave-origin.desktop
  - core/board/pc/rootfs-overlay/etc/xdg/melkor/firefox.desktop
  - core/board/pc/rootfs-overlay/usr/bin/melkor-browser
---

## Decided
O PipeWire, o WirePlumber e os clientes Pulse nativos usam as bibliotecas e os perfis da camada Alpine. Para áudio, os navegadores recebem o socket Unix do servidor Pulse da sessão. A regra de captura cobre a presença e a ausência do nome do cliente para que a decisão não dependa de dados enviados pelo aplicativo.

## Tried and ruled out
A placa HDA apareceu no kernel, mas o WirePlumber procurou o perfil ALSA em /usr/share/alsa-card-profile e não publicou a saída. O pactl também procurou libpulsecommon no diretório da distribuição. Uma condição vazia em pulse.rules não selecionou o cliente e o teste de captura deixou de receber a rejeição esperada.

## Watch out
In the code: melkor-session-exec aponta ACP_PATHS_DIR, ACP_PROFILES_DIR e o caminho do carregador para a camada privada. audio-pulse.conf cobre application.name = null e application.name = !null. tests/pc_desktop_vm.py verifica a placa HDA virtual e reprova a candidata se o servidor abrir um fluxo de captura ou permitir carregamento de módulos.
