---
title: Provisionar credenciais exige gravar o dono no ext4
author: William
date: 2026-11-21
branch: codex/provisionamento-pc
id: l-956ed63c
scope: repo
status: active
src: agent
touches:
  - Makefile
  - README.md
  - apps/cmd/melkor-disco/main.go
  - apps/cmd/melkor-storage/main.go
  - apps/internal/pcdisk/image.go
  - apps/internal/pcdisk/image_test.go
  - core/board/pc/rootfs-overlay/usr/libexec/melkor-pc-persist
  - docs/ARQUITETURA.md
  - docs/MARCOS.md
  - docs/TESTES.md
---

## Decided

Crie a árvore de provisão em um diretório de trabalho privado e use fakeroot com mke2fs. O ext4 recebe UID e GID zero para os hashes. O boot compara dono e modo antes de usar qualquer hash para autenticar.

## Tried and ruled out

A versão do mke2fs usada pelo contêiner anunciava o parâmetro para tar, mas a variante instalada havia sido compilada sem suporte a libarchive. O formatador recusou o tar ao popular a ext4. Copiar uma árvore comum da conta do host também preservaria o UID do mantenedor.

## Watch out

In the code: CreateImage em apps/internal/pcdisk/image.go usa o fakeroot do Buildroot somente quando recebe credenciais. ApplyCredentials em apps/internal/pcdisk/credentials.go exige 0700 no diretório e 0600 no arquivo, ambos com dono root, e combina as senhas com as contas do shadow da geração. Confirme a propriedade na VM e inicie os dois slots para testar as credenciais persistentes.
