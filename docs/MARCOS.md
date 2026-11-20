# Marcos de implementação

Registre commit, ambiente, comandos e evidências ao concluir cada marco.

## M0 — Do checkout ao console

**Entrega:** receita mínima de x86-64, compilada a partir das fontes, com kernel, musl, dinit como PID 1, rootfs somente para leitura e acesso por console serial. O identificador do sistema em `/etc/os-release` deve ser `melkor`. Inclua os drivers de armazenamento e console usados pela VM.

1. Fixar release e hash do Buildroot e a configuração do alvo. Manter a árvore externa, patches e configurações no checkout. Identificar ferramentas do host e separar o diretório de saída das fontes.
2. Criar e documentar duas entradas reais: construção da imagem e teste de boot. Escolher uma toolchain de bootstrap compatível com o alvo e registrar compilador, libc e linker efetivamente usados. Se a primeira imagem usar GCC, manter a transição para LLVM/libc++ explícita.
3. Gerar um disco inicializável e iniciá-lo via firmware UEFI/OVMF em QEMU. O teste deve exercer a entrada de boot do disco. Selecionar KVM quando disponível e permitir emulação por software com um prazo de teste apropriado.
4. Usar o console para verificar a identidade da imagem, identificar dinit em `/proc/1/comm` e confirmar as opções da montagem do rootfs. Guardar log serial e código de saída do teste no host.

**Aceite:** um checkout limpo com os pré-requisitos documentados gera a imagem. O executor encontra o login e verifica os três atributos acima dentro de um prazo limitado. Uma imagem incapaz de chegar ao console faz o teste falhar. Neste marco, o primeiro boot pode partir de uma imagem de disco pré-montada.

### Toolchain e interpretadores

O Buildroot 2025.02.20 está fixado no commit `2f9129e7909bff6d78ae8cfda3d0e73503dfcac6`. A receita resolve Linux 6.12.117, GCC 14.4.0, GNU binutils 2.43.1, musl 1.2.6, BusyBox 1.37.0, dinit 0.22.1 e GRUB 2.12. O dinit e o BusyBox usam o carregador `/lib/ld-musl-x86_64.so.1`. O dinit usa `libstdc++` e `libgcc_s` no bootstrap. A transição para LLVM/Clang e libc++ faz parte de M2 e exige um novo build do core e a repetição do teste UEFI.

No boot, o GRUB interpreta `grub.cfg`. O dinit interpreta suas descrições de serviço e inicia `melkor-setup` com `/bin/sh`, implementado pelo ash do BusyBox. No login serial, `getty` e `login` também vêm do BusyBox, e o ash executa os scripts de perfil. Python roda no host para controlar a VM.

### Validação do M0

Em 20 de novembro de 2026, a receita do commit `bd7454b3190ff5801c75f1bee8153e6da356fbd7` passou pelo build completo, pela verificação do SHA-256 e pelo boot UEFI. O teste de boot incluiu a alteração local que abre o disco virtual somente para leitura.

| Comando | Ambiente e resultado |
| --- | --- |
| `make check-env` | O Podman não conseguiu acessar seu diretório de execução em `/run/user/1000/libpod` porque a sessão o montou somente para leitura. O script agora informa a falha de acesso ao Distrobox. |
| `env -u GCONV_PATH make image BUILD_ENV= OUT=out/m0-revalidado` | Bazzite 45 x86-64, com `patch` e `makeinfo` do SDK Freedesktop no `PATH` e o cache preparado em `out/`. Os proxies HTTP e HTTPS apontaram para `127.0.0.1:9`. O build terminou a partir de uma pasta de saída nova. A remoção de `GCONV_PATH` evitou que o `mcopy` usasse os módulos de conversão de caracteres herdados do aplicativo que iniciou a sessão. |
| `QEMU_ACCEL=tcg make test-boot BUILD_ENV= OUT=out/m0-revalidado` | QEMU 11.1.3 fornecido por `QEMU_SYSTEM`, máquina q35, 512 MiB e OVMF do host. Com prazo de 240 s, o teste fez login como `melkor`, confirmou `ID=melkor`, o dinit como PID 1, a raiz SquashFS montada com `ro` e o boot via EFI. Código `0`. |
| `sha256sum -c ../../SHA256SUMS` em `out/m0-revalidado/build/images/` | O arquivo conferiu antes e depois do boot. |
| `python3 scripts/test-boot.py out/m0-teste-negativo/disco-vazio.img --log-dir out/m0-teste-negativo/tests --accel tcg --timeout 10` | Com o mesmo QEMU e OVMF, o disco vazio produziu timeout ao aguardar o login, código `1`. |

O SHA-256 da imagem validada é `22686b9e7aa9579eeb9e27be5ea4fdb5216d6645761df94a1218df15c2c1754b`. O build gravou `build.log`, `packages.json`, `toolchain.txt` e `SHA256SUMS` em `out/m0-revalidado/`. Os logs seriais e os relatórios JSON ficaram em `out/m0-revalidado/tests/boot-1it823va/` e `out/m0-teste-negativo/tests/boot-fcot092u/`.

## M1 — Atualização A/B recuperável

Escolher integração do bootloader, confirmação de saúde e assinatura das imagens. Definir a persistência de dados, configuração local e variáveis UEFI. Demonstrar atualização A → B, fallback automático após falha, seleção manual de A e interrupção da gravação com o mesmo disco. Incluir Kibi no core e preparar mídia de recuperação. [Casos de recuperação](TESTES.md)

### Validação da atualização assinada

Em 20 de novembro de 2026, a receita M1 gerou uma imagem com Linux 6.12.117, dinit, GRUB 2.12 e RAUC 1.13 a partir de um diretório de build limpo no contêiner Fedora 45. Os testes de boot usaram QEMU 11.1.3, KVM, OVMF e discos virtuais descartáveis.

| Comando | Resultado |
| --- | --- |
| `make check-env BOARD=m1` | Contêiner, bundle do Buildroot e fontes do M1 disponíveis. A verificação das fontes passou com os proxies inalcançáveis. |
| `make image BOARD=m1 OUT=out/m1-rauc` | Build concluído com certificado público e credencial root de laboratório passados ao hook de imagem. Inventário, log e hash em `out/m1-rauc/`. |
| `make check-m1 OUT=out/m1-rauc` | GPT, estado inicial do GRUB, configuração RAUC e partições aprovados. |
| `make test-boot BOARD=m1 OUT=out/m1-rauc` | Login pelos dois slots e persistência aprovados. Evidências em `out/m1-rauc/tests/boot-_s6wlrz4/`. |
| `make bundle-m1 OUT=out/m1-rauc … VERSION=1.0-test` | Bundle `verity` criado e assinatura verificada pelo RAUC do build. |
| `make test-update OUT=out/m1-rauc …` | Atualização A → B, retorno manual a A, fallback automático e recuperação após interrupção da escrita aprovados. O mesmo disco aceitou nova instalação do bundle após a interrupção. A suíte conferiu as rejeições e a preservação do slot ativo. Logs em `out/m1-rauc/update-tests/rauc-y25sw5os/`. |
| `sha256sum -c ../../SHA256SUMS` em `out/m1-rauc/build/images/` | A imagem original conferiu depois dos testes. |

O SHA-256 da imagem é `dd0e00611b6afb92cba4802f8f131897fff789935373701645abd1a98bf26f14`. O [procedimento de laboratório](TESTES.md#bundles-assinados-e-recuperação-m1) descreve os parâmetros dos comandos e a geração das credenciais.

`make test` passou com 84,9% das instruções Go, 95,5% das instruções e ramos Python e 96,0% das linhas Shell. Cada script Shell superou 80%. Relatórios em `out/coverage/test-ho6gs7q9/`. `make test-boot OUT=out/m0-revalidado` também passou na regressão de boot do M0.

Faltam Kibi e a mídia de recuperação, o provisionamento de instalações em disco físico e o fluxo de publicação e download de bundles. A validação de saúde do desktop fica para o M2.

### Validação do DNS do sistema

Em 20 de novembro de 2026, o M1 ganhou Quad9 via DoH como resolvedor padrão. A imagem candidata foi construída com o contêiner Fedora 45, as chaves de laboratório do RAUC e uma saída limpa em `out/m1-doh`.

| Comando | Resultado |
| --- | --- |
| `make env BOARD=m1` e `make check-env BOARD=m1` | O Buildroot verificou os hashes das fontes, incluindo o arquivo vendorizado do dnsproxy. A verificação com proxy inalcançável aprovou o cache. |
| `make image BOARD=m1 OUT=out/m1-doh` | Build completo com Linux 6.12.117, dinit, dnsproxy e dhcpcd. Na mesma saída, `dnsproxy-rebuild` seguido de `all` no Buildroot incorporou a identificação da versão no binário e o diretório de isolamento do DHCP. Logs em `out/m1-doh/build.log` e `out/m1-doh/repackage.log`. |
| `make check-m1 OUT=out/m1-doh` | Os dois slots contêm os quatro endpoints DoH do Quad9 e o resolvedor do sistema em loopback. |
| `make test-dns OUT=out/m1-doh` | Em IPv4, o TXT de diagnóstico confirmou `doh`. Após cortar a saída da VM, a consulta falhou e a captura mostrou tentativas apenas nos endpoints HTTPS permitidos. Evidências em `out/m1-doh/dns-tests/doh-_h2dmx3t/`. |
| `make test-dns OUT=out/m1-doh DNS_IPV6_ONLY=1` | Os mesmos cenários passaram com saída apenas por IPv6. Evidências em `out/m1-doh/dns-tests/doh-rm2w6m3v/`. |
| `make test-boot BOARD=m1 OUT=out/m1-doh` | Login em A e B e documento persistente aprovados. `make test-boot OUT=out/m0-revalidado` também passou. |
| `make test-update OUT=out/m1-doh …` | Rejeição dos bundles inválidos, promoção, retorno automático e repetição após interromper a gravação aprovados. Logs em `out/m1-doh/update-tests/rauc-0rvuf56g/`. |
| `make test` | 26 testes Python, testes Go com detector de corridas e cobertura de 84,9% Go, 95,7% Python e 96,0% Shell. Relatórios em `out/coverage/test-0a9vox4u/`. |

O SHA-256 da imagem candidata é `10a34a9ff185e6c04a0714c05b5dfd24c3b19ba8b77f413dda1f9020c55f6349`. A validação usou cópias em QEMU com KVM e OVMF. Wi-Fi, navegadores e contêineres de aplicativos entram nos testes de instalação e sessão.

## M2 — Desktop Wayland utilizável

Integrar a toolchain pretendida, utilitários selecionados, sessão dinit, Mesa, áudio e portais. Fixar versões compatíveis de Umbriel e wlroots. Validar Noctalia, Luciole e Alacritty em musl. Testar ABNT2, acentos, clipboard, bloqueio e recuperação de sessão. Estender o teste de saúde da candidata para detectar uma falha que impeça o login gráfico. Produzir o inventário dos runtimes observados.

## M3 — Perfil nativo e estação de trabalho

Implementar catálogo selecionado, receitas fixadas, build isolado e ativação atômica por perfil. Executar o primeiro ciclo completo com um aplicativo pequeno. Adicionar os aplicativos da tabela de arquitetura, priorizando Rhun, Git e servidores de linguagem. Cobrir falha de build, falta de espaço, instalações concorrentes e retorno a um perfil anterior. A ajuda de IA na criação de receitas entra depois que uma receita conhecida puder percorrer o fluxo sem IA.

### Validação inicial do perfil

Em 20 de novembro de 2026, o fluxo inicial instalou o Nushell 0.117.0 em um perfil descartável no Bazzite 45 x86-64. A compilação usou Podman rootless e a imagem fixada do Rust 1.97.1 para musl. O CLI foi compilado com Go 1.27.4.

| Comando ou cenário | Resultado |
| --- | --- |
| `make tools` e `go test -race ./...` em `apps/` | CLI estático e teste de promoção de geração aprovados. O teste cobre falha após preparar uma candidata, nova tentativa bem-sucedida e atualização sem mudanças. |
| `out/tools/melkor instalar nushell` | Geração ativada após conferir o SHA-256 da fonte, compilar com Cargo e verificar a versão, a expressão de teste e a linkagem estática. Log em `out/apps-validacao.PQ1kND/build.log`. |
| `out/tools/melkor atualizar` com receita candidata cujo build executa `false` | Código de erro no build. O link ativo, o hash do binário e a versão 0.117.0 permaneceram iguais. O launcher voltou a executar a expressão e retornou `6`. |
| `make check-m1` e boot de uma cópia do disco M1 com o perfil na ext4 | GPT verificado. QEMU com KVM iniciou ambos os slots. Em cada um, o launcher retornou `6` com Nushell e executou `/bin/sh` com um perfil vazio. Logs em `out/apps-m1.Ea5SHK/tests/boot-u1f_r69z/`. |

A validação de abertura em uma janela do Alacritty depende da sessão Wayland do M2. O Nushell é a primeira receita do catálogo. A cobertura de falta de espaço e a limpeza de gerações continuam previstas para M3.

### Validação de pacotes importados

Em 20 de novembro de 2026, o Bazzite com Podman rootless 6.1.3 executou a importação, a ativação e a remoção de aplicativos locais. Os testes usaram perfis descartáveis e as bases fixadas de Debian 13 e Fedora 45.

| Comando ou cenário | Resultado |
| --- | --- |
| `make tools` e `melkor importar` com os pacotes oficiais `tree` e `jq` | Compilou o CLI e ativou o `tree` 2.2.1-1 em DEB, o `tree` 2.3.1-2.fc45 em RPM e o `jq` 1.8.2-5.fc45 em RPM. O DNF instalou `oniguruma` junto do `jq`. `melkor executar` rodou os três aplicativos. Logs em `out/importacao-m3.gtMyaJ/`. |
| `make test-packages OUT=out/importacao-m3.gtMyaJ …` | O DEB de teste instalou uma dependência, executou o `postinst` sem rede e reabriu um arquivo salvo entre execuções. Checksum divergente, dependência ausente e assinatura RPM inválida foram rejeitados com a mesma geração ativa. A remoção preservou os dados e o outro aplicativo continuou executável. Logs em `out/importacao-m3.gtMyaJ/package-tests/pacotes-z45i59e7/`. |
| `make test` | 86,2% em Go, 95,7% em Python e 96,0% em Shell. Relatórios em `out/coverage/test-nxkit20w/`. |
| `sha256sum -c ../../SHA256SUMS` em `out/m1-doh/build/images/` | A imagem do core conferiu após os testes de importação. |

O [procedimento de importação](TESTES.md#provas-de-importação-isolada) registra os pré-requisitos e o SHA-256 do RPM usado nos testes. A entrega validada cobre aplicativos de linha de comando. Instalar o runtime Podman no Melkor e integrar a execução gráfica continuam como etapas do projeto.

### Validação do retorno de perfil

| Comando ou cenário | Resultado |
| --- | --- |
| `melkor geracoes` e `melkor voltar <geração>` em uma cópia do perfil Nushell 0.117.0 | A lista identificou a geração ativa. Após remover o Nushell, `melkor shell` abriu `/bin/sh`. A reativação do perfil anterior trouxe o Nushell de volta e a expressão `[1 2 3] \| math sum` retornou `6`. Evidências em `out/retorno-nushell.pVhyn7/retorno.log`. |
| `make test-packages OUT=out/importacao-m3.gtMyaJ/retorno …` | O DEB removido voltou a executar após reativar a geração anterior e leu o dado gravado. Foi possível selecionar de novo a geração mais recente sem o DEB. Evidências em `out/importacao-m3.gtMyaJ/retorno/package-tests/pacotes-1pwwcq44/`. |
| `make test` | Rejeições de manifestos, caminhos, arquivos e imagens ausentes preservaram a geração ativa. Cobertura de 86,7% em Go, 95,7% em Python e 96,0% em Shell. Relatórios em `out/coverage/test-45esy_dp/`. |

O catálogo separado passou pelo teste local de proposta, aceite, sincronização de tag SSH assinada e instalação em um segundo perfil sem IA. O primeiro PR real foi aberto pelo CLI e passou na CI do repositório privado. A primeira versão assinada do catálogo aguarda a revisão do PR. [Testes do catálogo](TESTES.md#publicação-e-consumo-de-receitas) · [PR #1](https://github.com/wkoch/Melkor-Recipes/pull/1)

## M4 — Instalar e usar no computador

Gerar instalador e mídia de resgate. Oferecer imagem previamente construída e modo de compilação local que parta de uma toolchain inicial identificada. Testar instalação em disco virtual vazio e atualização a partir de uma versão anterior com documentos e aplicativos. Validar em hardware próprio por mídia ou SSD de teste: GPU, armazenamento, rede, áudio, suspensão, monitores e latência de abertura dos aplicativos. Documentar a recuperação antes de promover o Melkor a sistema de uso diário.

Aplicar os [padrões da primeira instalação](INSTALACAO.md) e demonstrar disco único, swap em arquivo, conta comum e administração root explícita. Validar cadastro, desbloqueio e recuperação por senha com o sensor do notebook.
