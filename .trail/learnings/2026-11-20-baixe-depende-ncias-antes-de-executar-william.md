---
title: Baixe dependências antes de executar os scripts de pacotes importados
author: William
date: 2026-11-20
branch: main
id: l-202746c7
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
Baixe dependências e verifique os metadados na base fixada. Passe os arquivos baixados ao gerenciador em uma transação offline com todos os caminhos absolutos. As duas fases usam a mesma base e a rede só fica disponível na primeira. RPMs locais e dependências RPM passam pela verificação de assinatura na transação offline.

## Tried and ruled out
O apt-get --no-download com apenas o DEB principal falhou por caminho de arquivo não absoluto quando havia uma dependência a instalar. APT funcionou com os DEBs baixados passados como entradas locais e com as fontes desativadas. No DNF5, o comando install --downloadonly rejeitou --destdir. O cache configurado contém os RPMs da transação para copiar para a fase offline.

## Watch out
O RPM tree sozinho pode passar sem baixar dependências. O RPM jq do Fedora precisou de oniguruma. O fixture DEB exige tree e confirma que seu postinst só vê loopback.

In the code: apps/package_import.go, system_cachedir, keepcache, Dir::Etc::sourcelist, Dir::Etc::sourceparts, tests/m3_package_smoke.py.
