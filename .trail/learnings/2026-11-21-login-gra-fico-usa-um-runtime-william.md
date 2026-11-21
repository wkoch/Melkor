---
title: Login gráfico usa um runtime privado e a senha provisionada pelo instalador
author: William
date: 2026-11-21
branch: codex/instalacao-pc
id: l-b138d244
scope: repo
status: active
src: agent
touches:
  - Makefile
  - README.md
  - apps/desktop/alpine-3.24-sdk.lock
  - apps/desktop/alpine-3.24.lock
  - core/board/pc/post-build.sh
  - core/board/pc/rootfs-overlay/usr/bin/melkor-session-exec
  - core/board/pc/users.txt
  - core/configs/melkor_pc_defconfig
  - docs/ARQUITETURA.md
  - docs/TESTES.md
---

## Decided
O dinit inicia greetd após persist e seatd na imagem Umbriel. A conta de serviço desenha o login via Cage e PAM autentica a conta comum no shadow provisionado. O processo da sessão volta ao greetd quando o compositor termina.
## Tried and ruled out
Iniciar greetd com o carregador musl como programa mudaria /proc/self/exe e impediria o reexec usado para as sessões. A janela GTK padrão em QEMU também abortou ao carregar ícones PNG de decoração do ambiente isolado. O gtkgreet 0.8 tinha um buffer de 127 bytes úteis para uma senha que o instalador aceita com até 128 bytes.
## Watch out
Preserve a execução do greetd por ELF com intérprete privado e a separação dos serviços PAM do greeter e do usuário. O greeter precisa de uma regra auth para pam_setcred, mesmo quando o greetd inicia a conta de serviço sem pedir senha. O runtime Wayland do greeter é 0700 e pertence à conta de serviço. O executor pode verificar o processo por /proc antes de acionar o teclado via QMP.
In the code: core/board/pc/post-build.sh, core/board/pc/graphical-login, apps/desktop/patches/gtkgreet-login.patch, tests/pc_desktop_vm.py.
