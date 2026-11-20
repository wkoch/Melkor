---
title: A recuperação precisa exercitar o estado gravado pelo GRUB e uma escrita parcial real
author: William
date: 2026-11-20
branch: main
id: l-dfad2438
scope: repo
status: active
src: agent
touches:
  - ".github/workflows/m0.yml"
  - AGENTS.md
  - Makefile
  - README.md
  - docs/ARQUITETURA.md
  - docs/MARCOS.md
  - docs/TESTES.md
  - scripts/build-image.sh
  - scripts/build-packages.fedora
  - scripts/prepare-env.sh
---

## Decided
Valide o M1 em cópias do disco sob QEMU. O GRUB grava a tentativa na partição EFI antes do kernel, e o dinit só limpa a tentativa depois da montagem dos dados e do início do console. Aplique um bundle incompatível e um com corrupção interna antes de testar a promoção. Compare o hash do slot inativo para distinguir rejeição prévia de falha durante a escrita.

## Tried and ruled out
Simular uma candidata sem saúde removendo as dependências do serviço de confirmação impediu a montagem da ESP, o que também impediu inspecionar o estado do GRUB. Mantenha as dependências do serviço ao trocar apenas o comando de confirmação por uma falha deliberada.

## Watch out
O binário RAUC do host chama mksquashfs pelo PATH. Prefira os dois executáveis da mesma saída do Buildroot. A configuração de kernel precisa ativar explicitamente CONFIG_MD antes de CONFIG_DM_VERITY. Para simular perda de energia durante a cópia, reduza a taxa de escrita no disco virtual e observe a mudança do primeiro bloco sem a mudança do bloco final antes de encerrar o QEMU.

In the code: scripts/bundle-m1.py, core/board/m1/linux.fragment, core/board/m1/grub.cfg, tests/m1_update_vm.py, perform-pre-check=true.
