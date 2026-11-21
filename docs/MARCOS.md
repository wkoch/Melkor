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

### Primeira prévia para QEMU

A versão `v0.1.0-alpha.1` publica o disco M1 comprimido e o CLI de aplicativos estático para o host Linux x86-64. O manifesto vincula os artefatos ao commit `fcb32e6bf1e0574f9345f20e942ca3d3d36a510f`. A conta root está bloqueada e a imagem vem sem chaveiro do RAUC. O login serial de laboratório usa `melkor` como usuário e senha. [Release e instruções de teste](https://github.com/wkoch/Melkor/releases/tag/v0.1.0-alpha.1)

| Comando | Pré-requisito e resultado |
| --- | --- |
| `make check-env BOARD=m1` | O cache e as ferramentas do contêiner Fedora 45 passaram na verificação offline. |
| `MELKOR_RAUC_KEYRING= MELKOR_LAB_ROOT_HASH_FILE= make image BOARD=m1 OUT=out/v0.1.0-alpha.1-build` | Build completo a partir do cache verificado, em uma pasta nova e sem credenciais nem chaveiro do laboratório. |
| `make check-m1 OUT=out/v0.1.0-alpha.1-build` | A GPT, os slots e a configuração DoH passaram. Uma inspeção com `grub-fstest` confirmou o bloqueio de root e a ausência do runtime Python e do chaveiro em ambos os slots. |
| `make test-boot BOARD=m1 OUT=out/v0.1.0-alpha.1-build` | Com QEMU, OVMF e KVM, o teste confirmou login nos dois slots e reabriu o documento gravado em A. O teste atualizado também passou com `QEMU_ACCEL=tcg`. |
| `make test-dns OUT=out/v0.1.0-alpha.1-build` e o mesmo comando com `DNS_IPV6_ONLY=1` | Em QEMU, com OVMF, `tcpdump` e acesso ao Quad9 nas duas famílias, o TXT confirmou DoH. Após interromper a saída HTTPS da VM, as consultas falharam sem cair para DNS aberto. |
| `KCOV=out/test-tools/fedora/usr/bin/kcov make test TEST_PYTHON=out/test-venv/bin/python` | A suíte com Go, coverage.py e kcov registrou 86,7% em Go, 95,7% em Python e 96,0% em Shell no commit da prévia. Após o ajuste do teste de boot, Python ficou em 95,3%. |
| `gzip -n -6` e `sha256sum -c SHA256SUMS` | O disco comprimido foi conferido após baixar os artefatos do GitHub. O comando QEMU das instruções da release chegou ao login usando o disco distribuído. |

O SHA-256 do disco descomprimido é `2107eeaf9660b402247012cba8f0379d840df151ac6da565eb953a1f316d19fb`. No GitHub Actions, o runner de M1 da revisão publicada esgotou o disco durante o build. A cobertura e o boot de M0 passaram nessa execução. O workflow seguinte libera espaço retirando os SDKs de Android e .NET do runner hospedado de M1. [Execução da revisão publicada](https://github.com/wkoch/Melkor/actions/runs/44828807728)

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

### Prévia de diagnóstico para o notebook

O primeiro teste físico usou a ISO `v0.1.0-alpha.2` pelo Ventoy. A mídia abre um console local sem montar o SSD. A instalação em disco inteiro e o provisionamento de contas e swap são etapas pendentes. [Release Live](https://github.com/wkoch/Melkor/releases/tag/v0.1.0-alpha.2) · [Procedimento para o HP](INSTALACAO.md#teste-de-hardware-pelo-pendrive)

| Comando ou cenário | Pré-requisito e resultado |
| --- | --- |
| `make env BOARD=live` e `make check-env BOARD=live` | No Fedora 45 do contêiner `melkor-build`, as fontes do kernel, do firmware, dos utilitários e do xorriso passaram pela verificação de hashes e pelo teste de cache offline. |
| `make image BOARD=live OUT=out/live-hp-preview` | Build limpo da mídia. Para adicionar a ISO na mesma saída, o Buildroot executou `melkor_live_defconfig`, `grub2-rebuild`, `all` e `target-post-image`. `xorriso -report_el_torito plain` confirmou o registro de boot UEFI. |
| `make test-boot BOARD=live OUT=out/live-hp-preview` | Com TCG e OVMF, boot pelo USB virtual, login comum e NVMe ext4 intacto. O boot do pendrive com KVM nesse host não encontrou a entrada USB no OVMF. O executor Live usa TCG por padrão. |
| `distrobox enter -n melkor-build -- python3 scripts/test-boot.py out/live-hp-preview/build/images/melkor-live.iso --live --log-dir out/live-hp-preview/iso-tests` | Boot da ISO como CD UEFI com a mesma verificação de login, console, raiz em RAM e hash integral do NVMe. |
| Ventoy 1.1.18 em QEMU com TCG e OVMF | O pendrive virtual recebeu a ISO em uma partição de dados FAT32. O SHA-256 do arquivo copiado conferiu. Os modos normal e GRUB2 chegaram ao console de vídeo. O login pelo teclado autenticou o usuário comum em ambos. O NVMe foi detectado e seu hash permaneceu igual. |
| `KCOV=out/test-tools/fedora/usr/bin/kcov make test TEST_PYTHON=out/test-venv/bin/python` | Com Go, coverage.py e kcov, a suíte passou com 86,7% em Go, 95,3% em Python e 96,9% em Shell. |

O SHA-256 da ISO é `3a855577a11f6f4418a24f9cc9a6863b136db8ab4a80975ab2d4fcd080010b93`. A edição do M1 para QEMU também passou pela revalidação de boot A/B e persistência. Na imagem separada de laboratório, o procedimento de `make test-update OUT=out/m1-doh …` passou nos cenários de candidata saudável, falha antes da confirmação e interrupção da gravação.

### Teste físico da AX200

A alpha.2 iniciou pelo Ventoy no HP ProBook x360 435 G7, com teclado e vídeo, e detectou o SSD NVMe e o leitor Synaptics `06cb:00df`. No Fedora Atomic COSMIC da mesma unidade, a Intel AX200 (`8086:2723`) registrou `wlp1s0` via `iwlwifi` e se associou em 5 GHz. No Live, o driver solicitou `iwlwifi-cc-a0-77.ucode` e recebeu `-ENOENT`. A extração da ISO e o console do QEMU confirmaram o arquivo no initramfs. A origem da falha de busca no boot físico ainda não foi isolada. O mesmo log registrou `pci_enable_msi failed -38` e `failed to load regulatory.db`.

A alpha.3 embute o firmware no kernel, habilita PCI MSI e inclui a base regulatória assinada do `cfg80211`. No novo boot do HP, `/usr/sbin/iw dev` listou a interface `wlan0` em `phy#0`, no modo `managed`. Isso confirma a inicialização da AX200 no Melkor Live. Associação a uma rede, obtenção de endereço e consultas Quad9 via DoH pelo Wi-Fi ainda precisam de teste no notebook. [Release Live](https://github.com/wkoch/Melkor/releases/tag/v0.1.0-alpha.3)

| Comando ou cenário | Resultado |
| --- | --- |
| `make env BOARD=live` e `make image BOARD=live OUT=out/live-hp-wifi` | O Buildroot verificou os hashes das fontes e concluiu o build limpo no contêiner Fedora 45. `nm` confirmou os símbolos de firmware em `vmlinux`. `objcopy --dump-section` extraiu os bytes para comparação com os arquivos das receitas fixadas. Os kernels de ISO 9660 e El Torito conferiram com o binário auditado. |
| `make test-boot BOARD=live OUT=out/live-hp-wifi` e `python3 scripts/test-boot.py out/live-hp-wifi/build/images/melkor-live.iso --live --log-dir out/live-hp-wifi/iso-tests` | USB e CD inicializaram em QEMU com OVMF e TCG. O teste aprovou o firmware, a base regulatória e o hash do NVMe gravável. O mesmo executor rejeitou a alpha.2 pela ausência da base. |
| Boot direto em QEMU com initramfs descartável sem `regulatory.db` e `.p7s` | Com o mesmo initramfs nos dois boots, o kernel da alpha.3 usou a base incorporada. O kernel da alpha.2 registrou a falha esperada. |
| Ventoy 1.1.18 no QEMU | Os modos normal e GRUB2 chegaram ao login com teclado local. O hash do NVMe virtual permaneceu igual em ambos. |
| `KCOV=out/test-tools/fedora/usr/bin/kcov make test TEST_PYTHON=out/test-venv/bin/python` | Cobertura de 86,7% em Go, 95,3% em Python e 96,9% em Shell. Os boots de regressão do M0 e do M1 passaram. |

O SHA-256 da ISO é `b1570905e948e0c20a21fb194b625f0c884336afea637669da49ca320795570b`.

### Integração em QEMU do disco PC

O alvo `pc` exercita a distribuição de espaço em disco, a atualização do core e uma sessão Wayland com Weston. O ambiente Fedora 45 compilou os novos pacotes após configurar o alvo PC e reutilizou a toolchain do Buildroot já verificada. O gerador atua sobre arquivos regulares e a instalação física continua pendente.

| Comando ou cenário | Resultado |
| --- | --- |
| `make check-env BOARD=pc` | O contêiner `melkor-build`, o bundle fixado e as fontes do alvo passaram na verificação offline. |
| `make pc-disk OUT=out/pc-validated` | O gerador Go preparou a GPT, a ESP, os dois slots SquashFS e a ext4. Os testes de unidade cobrem a geometria com setores lógicos de 512 e 4096 bytes, a recusa de um SHA-256 divergente e a preservação de arquivos existentes. |
| `QEMU_ACCEL=tcg make test-pc-disk OUT=out/pc-validated` | Com OVMF, os dois slots iniciaram no mesmo NVMe virtual. O slot B reabriu o documento criado em A e os dois ativaram o swap de 4 GiB. Evidências em `out/pc-validated/tests/boot-oypthttp/`. |
| `make test-pc-desktop OUT=out/pc-validated` | Com KVM, VGA virtual e OVMF, a conta comum entrou pelo console local e iniciou Weston com DRM. O teste abriu foot pelo painel e capturou as telas. Evidências em `out/pc-validated/desktop-tests/desktop-r6ugrtse/`. |
| `make test-live-wifi OUT=out/pc-live-validated HWSIM=…` | Com QEMU e dois rádios `mac80211_hwsim`, a conta comum criou um ponto de acesso com WPA2 e se conectou a ele. Evidências em `out/pc-live-validated/wifi-tests/wifi-901hh8xs/`. |
| Ventoy 1.1.18 por UEFI e TCG | A ISO com o rótulo `MELKOR_LIVE_x86_64` e os arquivos usados pelo GRUB no início passou pelos modos normal e GRUB2. O arquivo ISO ocupava dois trechos na partição FAT32 do pendrive virtual. A conta local autenticou nos dois modos e o hash do NVMe permaneceu igual. Evidências em `out/ventoy-lab/pc-layout-normal/` e `out/ventoy-lab/pc-layout-grub2/`. |
| `make test-dns BOARD=live OUT=out/pc-live-validated` e `make test-dns BOARD=pc OUT=out/pc-validated`, ambos com e sem `DNS_IPV6_ONLY=1` | O Quad9 respondeu por HTTPS. As capturas de rede do QEMU também confirmaram a falha da consulta sem recurso a DNS em texto claro quando o acesso ao Quad9 foi interrompido. O enlace dos testes de DNS foi Ethernet virtual. |
| `make bundle-pc OUT=out/pc-rauc-lab …` e `QEMU_ACCEL=tcg make test-update BOARD=pc OUT=out/pc-rauc-lab …` | O core recusou bundle adulterado, assinatura inválida e uma imagem para M1. O teste confirmou a atualização A → B, a volta automática a A após falha de confirmação e nova instalação após interromper a gravação. O documento do usuário persistiu. Evidências em `out/pc-rauc-lab/update-tests/rauc-i5b9nxu6/`. |
| `KCOV=out/test-tools/fedora/usr/bin/kcov make test TEST_PYTHON=out/test-venv/bin/python` | 36 testes Python e cobertura de 84,9% em Go, 94,5% em Python e 96,9% em Shell. |

Em um disco de laboratório criado com `make pc-disk OUT=out/pc-credentials-lab PC_CREDENTIALS=…`, `make test-pc-disk` recusou a senha antiga e verificou a proteção das credenciais, a sessão comum e o `su -` com senha separada nos dois slots. O cenário usou o SquashFS anterior com o novo executável e o script de persistência empacotados na VM. Evidências em `out/pc-credentials-lab/tests/boot-0kh1y6vq/`.

O build limpo `make image BOARD=pc OUT=out/pc-clean` gerou o SquashFS, o kernel e o GRUB. `sha256sum --check` confirmou os três arquivos. A sessão Weston e o terminal foot passaram novamente em `make test-pc-desktop OUT=out/pc-clean`. O teste de DoH passou sobre Ethernet virtual em IPv4 e IPv6 com o novo disco. A validação do disco provisionado em `out/pc-clean-auth` repetiu a recusa da senha antiga, a autenticação separada de root, os dois boots por NVMe, o swap e a leitura do documento em B. Evidências em `out/pc-clean-auth/tests/boot-t7umowp3/`. A suíte final registrou 37 testes Python e cobertura de 84,8% em Go, 93,3% em Python e 96,9% em Shell.

O mesmo core, reempacotado com o certificado público de laboratório, repetiu os três cenários de `make test-update BOARD=pc` em KVM. As credenciais de root vieram do volume persistente. O RAUC rejeitou os três bundles de teste inválidos, promoveu a candidata saudável, voltou ao slot A após falha de confirmação e aceitou nova instalação após a interrupção de escrita. Evidências em `out/pc-clean-rauc-evidence/`.

A instalação em disco real, a sessão definitiva Umbriel/Noctalia, o perfil dos navegadores e a autenticação por digital continuam em desenvolvimento.
