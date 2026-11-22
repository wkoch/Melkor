---
title: A sessão Wi-Fi precisa compartilhar o endereço do barramento de sistema
author: William
date: 2026-11-21
branch: codex/instalacao-pc
id: l-2a96be70
scope: repo
status: active
src: agent
touches:
  - Makefile
  - README.md
  - core/Config.in
  - core/board/live/rootfs-overlay/etc/dinit.d/boot
  - core/board/network/rootfs-overlay/etc/dhcpcd.conf
  - core/board/network/rootfs-overlay/etc/dinit.d/dns
  - core/board/network/rootfs-overlay/etc/dinit.d/network-link
  - core/board/network/rootfs-overlay/etc/dinit.d/network-local
  - core/board/network/rootfs-overlay/etc/dnsproxy.json
  - core/board/network/rootfs-overlay/etc/resolv.conf
---

## Decided

O dinit e a sessão de login usam o endereço explícito do socket para que o IWD e seu cliente cheguem ao mesmo barramento D-Bus. O teste de boot consulta o nome do serviço no D-Bus. O teste com rádios virtuais verifica o cliente e a associação WPA2.

## Tried and ruled out

O IWD saía com código 1 mesmo com o barramento iniciado e com o kernel registrando os rádios. O caminho padrão do ELL apontava para /var/run/dbus, enquanto o daemon já escutava em /run/dbus. Nesse rootfs /var/run é um diretório próprio. O cliente inicializado logo após o login também podia aguardar indefinidamente antes da criação do socket.

## Watch out

In the code: melkor-system-bus anuncia prontidão pelo descritor 4. melkor-dbus.sh e o serviço wifi exportam DBUS_SYSTEM_BUS_ADDRESS para unix:path=/run/dbus/system_bus_socket. O Live pode iniciar sem rádios, portanto check_live_console em scripts/test-boot.py consulta o nome no barramento. tests/live_wifi_vm.py aguarda o socket antes de chamar iwctl e usa dois rádios para testar a associação em QEMU.
