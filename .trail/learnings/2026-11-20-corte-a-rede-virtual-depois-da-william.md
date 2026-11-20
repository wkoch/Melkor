---
title: Corte a rede virtual depois da rota e capture antes de descartar
author: William
date: 2026-11-20
branch: main
id: l-40f94e90
scope: repo
status: active
src: agent
touches:
  - ".github/workflows/m0.yml"
  - AGENTS.md
  - Makefile
  - README.md
  - core/Config.in
  - core/external.mk
  - docs/ARQUITETURA.md
  - docs/MARCOS.md
  - docs/TESTES.md
  - scripts/build-image.sh
---

## Decided
Nos testes de DNS, deixe a VM configurar sua rede e fazer uma consulta autenticada antes de ativar o corte. Ative a captura no mesmo ponto para distinguir os pacotes da consulta durante a falha dos pacotes usados para preparar a conexão.

## Tried and ruled out
Com restrict=on desde o boot, o DHCP IPv4 do slirp omitiu o roteador padrão. A VM não chegava ao caso de um resolvedor em funcionamento cujo HTTPS foi bloqueado. Ao adicionar o filtro de descarte na cauda da lista, a captura mostrou pacotes vindos da rede e perdeu as tentativas do convidado.

## Watch out
A saída do convidado é a fila RX do backend. O QEMU percorre a lista ao contrário nessa direção. Coloque o descarte na cabeça e a captura depois dele na lista, de modo que a captura veja as tentativas antes do descarte.

In the code: tests/m1_dns_vm.py, filter-redirector queue=rx position=head, filter-dump, qom-set.
