# Melkor Linux

Melkor é a distribuição Linux pessoal de William Koch. Sua estação de trabalho deve poder ser reconstruída a partir de um commit do Git. A receita determina o core, os serviços e o desktop. Os aplicativos nativos têm um catálogo próprio e podem ser instalados pelo fonte sem alterar o core em execução.

O marco M0 entrega uma imagem x86-64 que inicia por UEFI e chega ao console serial em QEMU. Os marcos seguintes cobrem recuperação A/B, desktop Wayland e aplicativos.

## Testar a primeira prévia

O fluxo padrão começa com o PC Core por UEFI, com console, Wi-Fi, Ethernet, Quad9 DoH e atualização A/B. O instalador atualizado usa o número do SSD e a confirmação `APAGAR`. Depois de iniciar pelo SSD, `melkor-finalizar` aplica o bundle assinado do desktop. [Instalação em duas etapas](docs/INSTALACAO.md#instalação-da-base-no-disco-inteiro) · [Publicação do canal do desktop](docs/TESTES.md#canal-do-desktop)

Baixe a imagem x86-64 pronta para QEMU com UEFI e siga as instruções de boot serial. A prévia inclui dinit, slots A/B, armazenamento persistente e DNS do sistema via Quad9 DoH. A release também oferece o CLI de aplicativos para o host Linux x86-64. A instalação em computador físico faz parte do M4. [Melkor v0.1.0-alpha.1](https://github.com/wkoch/Melkor/releases/tag/v0.1.0-alpha.1)

Para conferir o HP ProBook x360 435 G7, use a ISO **Melkor Live** com Ventoy. Ela roda em RAM e abre um console local para conferir tela, teclado e a detecção de dispositivos. [Teste de hardware pelo pendrive](docs/INSTALACAO.md#teste-de-hardware-pelo-pendrive)

Após aplicar o bundle do desktop e reiniciar, o Noctalia Greeter abre automaticamente. Entre como `melkor` com a senha que você definiu no instalador para iniciar Umbriel com Noctalia. Ao encerrar a sessão, a tela de login volta. Nessa tela, `Ctrl+Alt+F2` abre o console de recuperação.

Use `Super+Enter` para abrir Alacritty com Nushell. O Firefox ESR abre links HTTP e HTTPS por padrão. O lançador também inclui Brave Origin e Blanc. Os três navegadores reproduzem áudio pelo PipeWire da sessão. [Evidências do perfil](docs/TESTES.md#perfil-de-aplicativos-do-pc)

## A experiência pretendida

- **Alterar o core.** Editar a receita, recompilar o conjunto completo a partir das fontes e testar a imagem. Ativar a imagem como uma nova geração, mantendo a anterior inicializável.
- **Instalar um aplicativo.** Pesquisar no catálogo, escolher, compilar em ambiente descartável e ativar o resultado no perfil de usuário. Flatpak é dispensável para esse fluxo.
- **Trabalhar.** Iniciar Umbriel e Noctalia, usar Luciole para abrir e salvar documentos, Alacritty com Nushell como terminal e Rhun para código, Git e trabalho com agentes. Kibi permanece acessível pelo console de texto.
- **Recuperar a máquina.** Selecionar a geração anterior pelo bootloader ou iniciar pela mídia de recuperação. Documentos pessoais persistem entre os boots.

## Atualizar depois da instalação

O fluxo previsto é baixar uma geração assinada do Melkor pela própria instalação. O atualizador deve validar a assinatura e a compatibilidade, gravar o slot inativo e confirmar a geração depois que ela iniciar e passar na verificação de saúde. Se a candidata falhar, a instalação deve voltar ao slot anterior. Nesse fluxo, a imagem publicada já vem compilada.

Quem modificar a receita poderá gerar a própria imagem em um ambiente de build, com as fontes e ferramentas necessárias. Isso pode ser feito localmente em vez de depender de outra máquina. Os projetos de origem fornecem as versões e correções usadas pelas receitas. A instalação e suas atualizações dispensam enviar alterações a esses projetos.

No PC, entre com `su -` e execute `melkor-core-update`. O endereço de fábrica é `https://github.com/wkoch/Melkor/releases/download/pc-stable/melkor-pc-channel.json`. O comando consulta a revisão anunciada, valida o manifesto e instala uma versão mais recente no slot inativo. Reinicie para ativá-la. O canal oficial anuncia a revisão 1 do PC Core. [Publicação do canal](docs/TESTES.md#canal-de-atualizações-do-pc)

Para usar outro endereço HTTPS que publique manifestos assinados pela chave do canal, execute `melkor-core-update --channel URL`. A escolha persiste entre atualizações. Use `melkor-core-update --reset-channel` para restaurar o endereço de fábrica. Para instalar um bundle específico, o PC aceita `melkor-core-update --https URL SHA256` e o PC e o M1 aceitam `melkor-core-update /caminho/bundle.raucb`. `melkor atualizar` atende aos aplicativos do catálogo. [Marcos](docs/MARCOS.md)

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
| [Padrões da instalação](docs/INSTALACAO.md) | Disco, swap, contas, biometria e aplicativos adotados na primeira instalação. |

## Ambiente de build sem internet

O host Bazzite é imutável e não tem `sudo` sem senha. As ferramentas de build e de teste ficam no contêiner distrobox `melkor-build` (Fedora 45), criado pelo próprio projeto. Quem programa sem acesso à internet deve usar o cache já preparado em `out/`. A pasta `out/` não vai ao Git: não a apague.

| Comando | Pré-requisitos | Função |
| --- | --- | --- |
| `make env` | Rede quando faltam pacotes ou fontes, `distrobox`, `podman` | Cria `melkor-build`, instala os requisitos de `scripts/build-packages.fedora`, baixa o bundle do Buildroot 2025.02.20 em `out/cache/` e as fontes do alvo em `out/dl/`. |
| `make check-env` ou `make check-env BOARD=m1` | Contêiner `melkor-build` acessível pelo Distrobox, sem necessidade de rede | Confere pacotes, bundle, commit `2f9129e…` e cache de fontes do alvo. Use `make env BOARD=m1` para preparar as fontes do M1. |
| `make image OUT=out/m0` | `make env` concluído | Constrói a imagem no contêiner. Cada build exige uma pasta `OUT` nova. |
| `make test-boot OUT=out/m0` | Imagem gerada, QEMU e OVMF no contêiner | Executa o teste de boot UEFI. `QEMU_SYSTEM` substitui o binário do QEMU. |
| `make image BOARD=m1` | `make env BOARD=m1` e `mke2fs` do e2fsprogs | Gera o disco de laboratório com slots A/B e dados em ext4. Informe o PEM dos certificados públicos em `MELKOR_RAUC_KEYRING` para habilitar atualizações assinadas. |
| `make check-m1` | Imagem M1, Python 3.11 ou posterior e `sfdisk` e `blkid` do util-linux | Lê a tabela GPT e verifica os arquivos de boot, atualização e DNS dos slots. |
| `make test-boot BOARD=m1` | Imagem M1, QEMU e OVMF | Inicia A, grava um documento e o lê após selecionar B, usando a mesma cópia do disco. |
| `make env BOARD=live` e `make image BOARD=live` | Ambiente Buildroot, rede para preparar o cache inicial e saída nova | Verifica as fontes e constrói `melkor-live.iso` e `melkor-live.img`, mídias UEFI x86-64 que carregam o sistema em RAM. |
| `make test-boot BOARD=live` | Imagem Live, QEMU, OVMF, `sfdisk` e `mke2fs` | Inicia pelo USB virtual e confere o login, o framebuffer e a integridade do NVMe virtual. |
| `make test-live-wifi HWSIM=…` | Build Live, QEMU, `cpio` e módulo `mac80211_hwsim.ko` compilado para o mesmo kernel | Exercita a associação WPA2 pelo usuário comum, sem usar a rede do host. [Preparação do módulo](docs/TESTES.md#associação-wi-fi-em-qemu) |
| `make env BOARD=pc` e `make image BOARD=pc` | Contêiner de build, cache de fontes e saída própria para o alvo PC | Gera o core, o kernel e o bootloader do protótipo PC. |
| `make image BOARD=pc OUT=out/pc-minimal` | Ambiente PC preparado e diretório de saída novo | Gera o core mínimo com console, Wi-Fi, Ethernet, Quad9 DoH e RAUC. As camadas de desktop selecionam a edição completa. |
| `make env BOARD=installer` e `make image BOARD=installer` | Contêiner de build, cache preparado e saída nova | Gera o kernel e o initramfs da mídia dedicada de instalação. |
| `make installer-iso PC_OUT=out/pc` e `make test-installer` | Builds do PC e do instalador, `cpio`, `dosfstools`, QEMU e OVMF | Empacota o core na ISO e testa a instalação sem rede em um NVMe virtual. `TEST_SECTOR_SIZE=4096` testa 4Kn. `PC_DESKTOP_BUNDLE=…` aplica o desktop após instalar a base e testa o login. [Validação do instalador](docs/TESTES.md#instalação-de-disco-inteiro) |
| `make pc-disk` e `make test-pc-disk` | Build PC, Go, `sfdisk`, `mke2fs`, `mkfs.vfat`, `mtools`, QEMU e OVMF | Cria um arquivo de disco de laboratório e inicia seus dois slots por NVMe virtual. `PC_DISK_GIB` ajusta os 16 GiB iniciais. |
| `make pc-disk PC_CREDENTIALS=…` | Build PC, `fakeroot` do Buildroot e arquivo privado de hashes | Registra senhas separadas de root e do usuário comum nos dados persistentes. [Validação das contas](docs/TESTES.md#contas-locais-do-pc) |
| `make test-pc-desktop` | Disco virtual PC, QEMU e OVMF | Exercita o login local e o terminal Wayland. `PC_DESKTOP_LOGIN=1` seleciona o teste do login gráfico com Umbriel. `PC_DESKTOP_AUDIO=1` exercita a placa HDA virtual com `PC_DESKTOP_APPS=1`, a camada de aplicativos e o driver WAV do QEMU. |
| `make pc-desktop-layer pc-chromium-layer pc-session-layer` | Podman rootless, Python 3.11+, curl e cache dos pacotes fixados | Prepara os aplicativos e as dependências gráficas e compila Umbriel, Noctalia Shell e Noctalia Greeter. [Build e testes do perfil](docs/TESTES.md#perfil-de-aplicativos-do-pc) |
| `make test-dns OUT=out/m1-doh` | Imagem M1, QEMU, OVMF, `tcpdump` e acesso HTTPS ao Quad9 | Confirma DoH pelo resolvedor da VM e inspeciona o tráfego com e sem acesso à internet. Use `DNS_IPV6_ONLY=1` em um host com acesso IPv6. |
| `make test-dns BOARD=live` | ISO Live, QEMU, OVMF, `tcpdump` e acesso HTTPS ao Quad9 | Executa os mesmos casos de DoH pelo adaptador Ethernet virtual da sessão Live. |
| `make bundle-m1 OUT=out/m1-rauc CERT=… KEY=… KEYRING=… VERSION=1.0` | Build do M1, certificado de assinatura, chave privada e chaveiro público confiável pela imagem | Usa o RAUC do build para gerar `OUT/melkor-m1-1.0.raucb` a partir do SquashFS. |
| `make test-update OUT=out/m1-rauc CERT=… KEY=… ROOT_PASSWORD_FILE=…` | Imagem M1 com certificado e credencial descartáveis, Python 3.11+, QEMU, OVMF, `mke2fs` e `sfdisk` | Testa instalação, rejeições, confirmação e recuperação do core em cópias do disco. [Procedimento de laboratório](docs/TESTES.md#bundles-assinados-e-recuperação-m1) |
| `make bundle-pc CERT=… KEY=… KEYRING=… VERSION=…` e `make test-update BOARD=pc CERT=… KEY=… ROOT_PASSWORD_FILE=…` | Build e disco virtual PC com as credenciais do laboratório | Usa a compatibilidade RAUC própria do PC e testa a atualização em discos descartáveis. |
| `make test-update-https CERT=… KEY=… ROOT_PASSWORD_FILE=…` | Build e disco virtual PC, Python 3.11+, QEMU, OVMF, OpenSSL e `mke2fs` | Serve bundles por TLS local e verifica rejeições, promoção e persistência em uma cópia do disco. [Procedimento](docs/TESTES.md#download-https-de-atualizações-do-pc) |
| `make channel-pc VERSION=… KEYRING=… CHANNEL_KEY=… BUNDLE_URL=…` | Bundle PC já criado, chave Ed25519 privada do canal, chaveiro RAUC e ferramentas do build | Verifica o bundle e gera `OUT/melkor-pc-channel.json` com a revisão extraída do SquashFS. |
| `make test-channel-pc CERT=… KEY=… ROOT_PASSWORD_FILE=… CHANNEL_KEY=… CHANNEL_CANDIDATE=…` | Dois builds PC com revisões crescentes e chaves descartáveis | Exercita a atualização sem argumentos e a troca persistente de canal. [Procedimento](docs/TESTES.md#canal-de-atualizações-do-pc) |
| `make test` | Go 1.24+, compilador C, Python 3.11+, coverage.py e kcov | Executa a suíte local e reprova cobertura abaixo de 80%. [Preparação e critérios](docs/TESTES.md#cobertura-do-código-próprio) |

Em um ambiente com as ferramentas já instaladas, use `BUILD_ENV=` para executar os comandos diretamente. No CI, a execução direta é o padrão. O teste de boot requer Python 3.11 ou posterior, QEMU e um par OVMF em formato raw. Em M0, ele abre a imagem somente para leitura. Em M1, usa uma cópia gravável da imagem. Ambos usam uma cópia privada das variáveis UEFI em `OUT/tests/`.

O build grava `melkor-m0.img` em `OUT/build/images/`. Na pasta da imagem, execute `sha256sum -c ../../SHA256SUMS` para conferir o arquivo. `OUT/packages.json` registra o inventário do Buildroot e `OUT/toolchain.txt` registra o compilador, o linker e o carregador ELF do dinit.

Registro da primeira preparação (host Bazzite, 2026-11-20): `make env` levou cerca de 5 minutos com rede. O bundle tem 114 MB e o cache de fontes, 379 MB. A verificação offline usa `http_proxy` apontando para `127.0.0.1:9`, para que qualquer download falhe.

Na preparação para o protótipo M1, o contêiner recebeu `e2fsprogs` 1.47.4. O requisito `/usr/bin/wget` é satisfeito pelo `wget2-wget` já instalado no Fedora 45. `make check-env` aprovou as ferramentas e o cache do core.

Registro do primeiro build offline (rede bloqueada com `http_proxy` em `127.0.0.1:9`):

- Tentativas com falha de ambiente, já corrigidas: `host-libxcrypt` precisou de `perl-open`; `host-libopenssl` precisou de `perl-Time-Piece` e de outros módulos Perl. Todos estão em `scripts/build-packages.fedora`.
- Build completo até `target-post-image`. A falha seguinte era da receita: o `genimage` resolve `files` em `--inputpath`, não em `--rootpath`. `core/board/m0/post-image.sh` foi corrigido. A etapa foi refeita com `make -C out/m0/buildroot O=$PWD/out/m0/build BR2_EXTERNAL=$PWD/core target-post-image`.
- `make test-boot OUT=out/m0` passou: login serial, `melkor` como identidade, `dinit` como PID 1 e raiz SquashFS somente leitura. O teste também tinha um erro de expressão regular, causado pelo `\r` antes do prompt do console; `scripts/test-boot.py` foi ajustado.
- A validação posterior de ponta a ponta gerou `out/m0-revalidado/build/images/melkor-m0.img` em uma pasta nova e confirmou o boot dessa imagem. Veja o [registro de validação do M0](docs/MARCOS.md#validação-do-m0).
- Pastas `out/m0-falha-*` guardam as tentativas com falha, para diagnóstico.

## Perfil nativo de aplicativos

| Comando | Pré-requisitos | Função |
| --- | --- | --- |
| `make tools` | GNU Make e Go 1.24 ou posterior no host | Gera `out/tools/melkor`, um executável Linux x86-64 estático. |
| `out/tools/melkor instalar nushell` | Podman rootless e rede para obter a imagem de build, a fonte e as dependências | Compila o Nushell 0.117.0 para musl, valida o executável e ativa uma geração do perfil. |
| `out/tools/melkor atualizar` | Catálogo revisado e, quando houver mudanças, os mesmos pré-requisitos de instalação | Constrói receitas alteradas dos aplicativos instalados e ativa o conjunto após validar todas. |
| `out/tools/melkor geracoes` | Perfil de aplicativos | Lista as gerações retidas com as versões dos aplicativos. O asterisco indica a geração ativa. |
| `out/tools/melkor voltar <geração>` | Geração listada pelo comando anterior e seus artefatos locais | Reativa a geração sem reconstruir os aplicativos. Para pacotes importados, verifica se as imagens ainda existem no Podman. |
| `out/tools/melkor shell` | Terminal interativo | Abre o Nushell da geração ativa ou usa `/bin/sh` quando o aplicativo está ausente. |

O catálogo embutido fica em [apps/recipes](apps/recipes). Para experimentar um conjunto completo de receitas revisadas, defina `MELKOR_CATALOGO` com o caminho desse diretório. O catálogo selecionado deve conter as receitas de todos os aplicativos ativos para `melkor atualizar`. A receita do Nushell fixa o arquivo de [crates.io](https://crates.io/crates/nu/0.117.0) e a [imagem oficial do Rust](https://hub.docker.com/_/rust) usada no build. Cargo usa as versões e os checksums do `Cargo.lock` publicado. A compilação e os testes executam sem rede após a obtenção dos insumos. A variante inicial oferece o shell e seus comandos básicos com as funcionalidades opcionais do upstream desabilitadas.

As gerações e seus manifestos ficam em `${XDG_DATA_HOME:-$HOME/.local/share}/melkor/geracoes/`. O link `atual` é trocado depois de validar a geração a ativar. Para testar em um perfil temporário, defina `MELKOR_PERFIL` com o caminho absoluto do perfil de teste.

O retorno troca apenas a geração de aplicativos. Os comandos seguintes usam o conjunto reativado e os dados de usuário continuam na versão atual.

Para usar o Nushell no Alacritty, disponibilize o executável `melkor` no `PATH` da sessão e importe [apps/alacritty/melkor.toml](apps/alacritty/melkor.toml) pelo caminho do arquivo em `general.import` na configuração do terminal. Cada nova janela resolve o executável da geração ativa e inclui os aplicativos do perfil no `PATH`. Sem Nushell no perfil, o launcher usa `/bin/sh`. O console de recuperação usa `/bin/sh` diretamente. [Configuração do Alacritty](https://alacritty.org/config-alacritty.html)

### Receitas novas e catálogo sincronizado

| Comando | Pré-requisitos | Função |
| --- | --- | --- |
| `melkor receita testar <nome> <proposta.json>` | CLI e os pré-requisitos de build da receita | Compila e testa a proposta com Podman rootless e deixa os executáveis e defaults em um diretório de teste. |
| `melkor receita aceitar <teste>` | Identificador da proposta testada e conferida | Salva a receita para `melkor instalar <nome>` no perfil local. |
| `melkor receita publicar <nome> <checkout>` | Receita aceita, clone do repositório do GitHub com `origin` e `main`, Git com identidade e push configurados, `gh` autenticado | Envia uma branch com a receita e abre ou localiza o PR. |
| `melkor catalogo validar <diretório>` | Diretório `recipes/` do checkout | Valida o formato das receitas antes da publicação. |
| `melkor catalogo sincronizar <origem> <tag> <chaves-ssh>` | Git, OpenSSH e arquivo local de assinantes autorizados | Verifica a assinatura da tag, valida as receitas e troca o catálogo local. |

Para um aplicativo ausente, peça à sua IA para gerar uma proposta no formato das receitas existentes e executar `melkor receita testar`. Confira o aplicativo e use o identificador exibido para aceitar a receita. A instalação passa a ser repetível pelo CLI, com ou sem acesso ao repositório de receitas. O agente de IA é externo ao CLI.

Por padrão, `melkor instalar` procura no catálogo assinado sincronizado, nas receitas aceitas localmente e no catálogo embutido, nessa ordem. Cada instalação registra a tag e o commit da receita publicada no manifesto. O arquivo de assinantes fica fora do catálogo baixado. [Preparação do repositório e da CI](catalogo/README.md)

### Pacotes DEB e RPM isolados

Com `make tools` concluído, use `out/tools/melkor` como o executável `melkor` dos exemplos. A importação requer Podman rootless e acesso aos repositórios da distribuição da imagem. Confirme o SHA-256 por uma fonte confiável do pacote.

```sh
melkor importar tree /caminho/tree.deb "$sha256" /usr/bin/tree --version
melkor executar tree --version
melkor remover tree
```

O argumento após o SHA-256 é o caminho do executável dentro da imagem. Os argumentos seguintes, se informados, são usados para testar a candidata antes da ativação. Repita `melkor importar` com o mesmo nome para atualizar um pacote importado. A remoção retira a entrada da geração ativa e preserva seus dados em `perfil/dados/<nome>`. As gerações anteriores continuam referenciando as imagens já importadas.

O M3 usa uma base Debian 13 para `.deb` e Fedora 45 para `.rpm`, ambas x86-64. O RPM precisa de assinatura válida. Para um fornecedor externo, informe `--chave-rpm=/caminho/chave.asc` antes do nome, usando a chave pública validada do fornecedor. O modo atual executa programas de terminal com a rede desligada e monta apenas os dados do aplicativo. [Arquitetura](docs/ARQUITETURA.md#aplicativos-distribuídos-em-deb-ou-rpm) · [Teste com pacotes reais](docs/TESTES.md#provas-de-importação-isolada)

Repositório do projeto: [wkoch/Melkor](https://github.com/wkoch/Melkor).
