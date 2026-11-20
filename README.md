# Melkor Linux

Melkor é a distribuição Linux pessoal de William Koch. Sua estação de trabalho deve poder ser reconstruída a partir de um commit do Git. A receita determina o core, os serviços e o desktop. Os aplicativos nativos têm um catálogo próprio e podem ser instalados pelo fonte sem alterar o core em execução.

**Estágio atual deste material:** especificação inicial para desenvolvimento. O primeiro resultado a construir é uma imagem x86-64 que inicia por UEFI e chega ao console em QEMU. Os marcos seguintes cobrem recuperação A/B, desktop Wayland e aplicativos.

## A experiência pretendida

- **Alterar o core.** Editar a receita, recompilar o conjunto completo a partir das fontes e testar a imagem. Ativar a imagem como uma nova geração, mantendo a anterior inicializável.
- **Instalar um aplicativo.** Pesquisar no catálogo, escolher, compilar em ambiente descartável e ativar o resultado no perfil de usuário. Flatpak é dispensável para esse fluxo.
- **Trabalhar.** Iniciar Umbriel e Noctalia, usar Luciole para abrir e salvar documentos, Alacritty como terminal e Rhun para código, Git e trabalho com agentes. Kibi permanece acessível pelo console de texto.
- **Recuperar a máquina.** Selecionar a geração anterior pelo bootloader ou iniciar pela mídia de recuperação. Documentos pessoais persistem entre os boots.

## Começar com o Codex

Abra no Codex a pasta de trabalho do repositório. Estes arquivos devem estar nela, com `AGENTS.md` e este README na raiz. Quem ainda precisa de um checkout pode obtê-lo com `git clone https://github.com/wkoch/Melkor.git`. Mantenha o Bazzite como host de desenvolvimento e destine discos virtuais descartáveis aos testes.

**Primeira tarefa para colar no Codex**

> Leia AGENTS.md e os critérios de M0 em docs/MARCOS.md. Inspecione o checkout e as ferramentas disponíveis. Implemente a menor receita versionada que gere uma imagem Melkor x86-64 com musl, dinit e console serial. Adicione uma entrada de build e um teste automatizado de boot UEFI em QEMU. Execute o que o ambiente permitir, registre os comandos reais no README e identifique qualquer critério pendente. Trabalhe somente em M0.

| Documento | Para que serve |
| --- | --- |
| [AGENTS.md](AGENTS.md) | Instruções para as sessões do Codex neste repositório. |
| [Arquitetura](docs/ARQUITETURA.md) | Contrato da receita, limites do core, política de dependências e catálogo inicial. |
| [Marcos](docs/MARCOS.md) | Sequência de implementação, entregas e decisões pendentes. |
| [Testes](docs/TESTES.md) | Casos de aceitação em VM, recuperação e validação no hardware real. |

## Ambiente de build sem internet

O host Bazzite é imutável e não tem `sudo` sem senha. As ferramentas de build e de teste ficam no contêiner distrobox `melkor-build` (Fedora 45), criado pelo próprio projeto. Quem programa sem acesso à internet deve usar o cache já preparado em `out/`. A pasta `out/` não vai ao Git: não a apague.

| Comando | Pré-requisitos | Função |
| --- | --- | --- |
| `make env` | Rede (só na primeira vez), `distrobox`, `podman` | Cria `melkor-build`, instala `scripts/build-packages.fedora`, baixa o bundle do Buildroot 2025.02.20 em `out/cache/` e as fontes do alvo em `out/dl/`. |
| `make check-env` | Nenhum (roda com a rede bloqueada) | Confere pacotes, bundle, commit `2f9129e…` e cache de fontes. Falha se algo faltar. |
| `make image OUT=out/m0` | `make env` concluído | Constrói a imagem no contêiner. Cada build exige uma pasta `OUT` nova. |
| `make test-boot OUT=out/m0` | Imagem gerada, QEMU e OVMF no contêiner | Executa o teste de boot UEFI. `QEMU_SYSTEM` substitui o binário do QEMU. |

Sem distrobox, `make image` e `make test-boot` rodam direto no PATH atual, como no CI.

Registro da primeira preparação (host Bazzite, 2026-11-20): `make env` levou cerca de 5 minutos com rede. O bundle tem 114 MB e o cache de fontes, 379 MB. A verificação offline usa `http_proxy` apontando para `127.0.0.1:9`, para que qualquer download falhe.

Registro do primeiro build offline (rede bloqueada com `http_proxy` em `127.0.0.1:9`):

- Tentativas com falha de ambiente, já corrigidas: `host-libxcrypt` precisou de `perl-open`; `host-libopenssl` precisou de `perl-Time-Piece` e de outros módulos Perl. Todos estão em `scripts/build-packages.fedora`.
- Build completo até `target-post-image`. A falha seguinte era da receita: o `genimage` resolve `files` em `--inputpath`, não em `--rootpath`. `core/board/m0/post-image.sh` foi corrigido. A etapa foi refeita com `make -C out/m0/buildroot O=$PWD/out/m0/build BR2_EXTERNAL=$PWD/core target-post-image`.
- `make test-boot OUT=out/m0` passou: login serial, `melkor` como identidade, `dinit` como PID 1 e raiz SquashFS somente leitura. O teste também tinha um erro de expressão regular, causado pelo `\r` antes do prompt do console; `scripts/test-boot.py` foi ajustado.
- Ainda não foi feito um build limpo de ponta a ponta com o `post-image.sh` corrigido. Para isso, rode `make image OUT=out/m0-N` com rede bloqueada e depois `make test-boot OUT=out/m0-N`.
- Pastas `out/m0-falha-*` guardam as tentativas com falha, para diagnóstico.

Repositório do projeto: [wkoch/Melkor](https://github.com/wkoch/Melkor).
