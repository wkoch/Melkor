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

Faltam Kibi, a mídia de recuperação, a publicação de bundles assinados e o teste de instalação em disco físico. O download explícito de bundles pelo PC está descrito adiante. A validação de saúde do desktop fica para o M2.

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

O catálogo separado passou pelo teste local de proposta, aceite, sincronização de tag SSH assinada e instalação em um segundo perfil sem IA. O primeiro PR real foi aberto pelo CLI, passou na CI e foi integrado. A tag assinada `v0.1.0` aponta para o merge `0e936c1`. A sincronização por HTTPS ativou a receita do Nushell em um perfil descartável com a chave pública fixada em `catalogo/allowed_signers`. [Testes do catálogo](TESTES.md#publicação-e-consumo-de-receitas) · [PR #1](https://github.com/wkoch/Melkor-Recipes/pull/1)

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

### Atualização por HTTPS no PC

O Buildroot recompilou a receita `melkor-storage` e montou a raiz do PC em um diretório vazio com a toolchain já verificada. O SquashFS recebeu o certificado público descartável do RAUC. A verificação com `sha256sum -c ../../SHA256SUMS` aprovou o SquashFS, o kernel e o GRUB em `out/pc-https/build/images/`. `make pc-disk OUT=out/pc-https PC_CREDENTIALS=…` preparou o disco virtual com senhas separadas.

Com KVM, `make test-update-https OUT=out/pc-https CERT=… KEY=… ROOT_PASSWORD_FILE=…` verificou a trava de atualização e a recusa de certificado TLS desconhecido, SHA-256 divergente e assinatura RAUC inválida. Nenhuma recusa mudou a ordem do GRUB ou o slot inativo. A atualização válida preservou A, iniciou B com o marcador da geração candidata e manteve o documento do usuário. Os logs estão em `out/pc-https/https-tests/https-fdtkvvy2/`. `make test-pc-desktop OUT=out/pc-https-desktop` também passou com o mesmo SquashFS. [Procedimento de laboratório](TESTES.md#download-https-de-atualizações-do-pc)

`make test` passou com 41 testes Python e cobertura de 80,2% em Go, 92,4% em Python e 96,4% em Shell. O teste usou certificados descartáveis e o canal público de bundles assinados segue pendente.

O Buildroot gerou os SquashFS das revisões de laboratório 1 e 2, com a URL estável da fábrica e a chave Ed25519 pública de teste. `make channel-pc` extraiu o bundle RAUC verificado e assinou o manifesto com a revisão 2. Em QEMU com KVM, `make test-channel-pc OUT=out/pc-channel-lab …` provou a recusa de assinatura alterada, a instalação pela chamada sem argumentos e o boot em B. A seleção de canal persistiu, a mesma revisão foi reconhecida como atual e o reset restaurou a configuração de fábrica. `make test` passou com 45 testes Python e cobertura de 80,1% em Go, 92,4% em Python e 96,7% em Shell. Os registros estão em `out/pc-channel-lab/`. [Procedimento](TESTES.md#canal-de-atualizações-do-pc)

### Instalação offline pela ISO

A mídia dedicada empacota o core PC e prepara uma GPT pelo tamanho e pelo setor lógico do disco selecionado. O fluxo pede confirmação pelo ID estável e provisiona as duas contas antes de abrir o destino para gravação. A ISO foi validada com o NVMe que recebe o sistema e outro NVMe gravável. Os testes comparam os hashes do vizinho e iniciam os dois slots com a mídia removida. A instalação física no HP, a sessão definitiva Umbriel/Noctalia, o perfil dos navegadores e a autenticação por digital continuam pendentes.

`make check-env BOARD=installer` confirmou o cache offline. A mídia de laboratório usou a toolchain do build do PC, com reinstalação dos pacotes selecionados em uma raiz vazia e recompilação do util-linux, kmod e eudev. `make installer-iso OUT=out/installer-clean PC_OUT=out/pc-clean-auth` conferiu os arquivos dos dois builds e produziu a ISO. O catálogo El Torito contém a entrada UEFI, com a imagem EFI de 16 MiB.

`make test` passou com 40 testes Python e cobertura de 80,7% em Go, 92,4% em Python e 96,3% em Shell. [Roteiro de validação](TESTES.md#instalação-de-disco-inteiro)

A mesma ISO passou pela instalação offline em QEMU 11.1.3 com OVMF e TCG, como CD UEFI com setores lógicos de 512 bytes, pelo modo normal do Ventoy 1.1.18 com 512 bytes e pelo modo GRUB2 com 4096 bytes. Em cada um, a confirmação inválida preservou o destino. A instalação válida fez a GPT até a reserva final, protegeu as senhas e chegou ao login nos dois slots, com swap ativo e o mesmo documento. O disco vizinho de 13 GiB manteve o SHA-256 integral. Os relatórios estão em `out/installer-clean/evidence/`.

`make installer-iso OUT=out/installer-https-lab PC_OUT=out/pc-https` reempacotou a mídia com o core que inclui o atualizador. `make test-installer BUILD_ENV= OUT=out/installer-https-lab` passou pelo CD UEFI com o QEMU 11.1.3 do host em TCG. Os dois slots iniciaram e o disco vizinho manteve seu SHA-256. O relatório está em `out/installer-https-lab/tests/install-ac346fvy/result.json`.

### Prévia com aplicativos

O core de laboratório em `out/pc-apps-release` inclui Nushell, Alacritty, Firefox ESR, Brave Origin, Blanc, Umbriel e Noctalia. O build reutilizou a toolchain do PC, recompilou o kernel com namespaces de usuário e manteve dinit como PID 1. As receitas e os hashes do SDK e das camadas ficam em `apps/desktop/`.

O executor `tests/pc_desktop_vm.py --apps --chromium --umbriel` iniciou a sessão gráfica com a conta comum. Noctalia abriu o painel e o atalho `Super+Enter` iniciou o Nushell no Alacritty. O Firefox renderizou uma página local e abriu por Wayland. Brave Origin e Blanc criaram processos de renderização com seccomp ativo. O registro e as capturas estão em `out/pc-apps-release/desktop-tests/desktop-9u5v96ak/`. A ISO de 945 MiB passou pelo CD UEFI e pelo modo normal do Ventoy, com 8 GiB na VM. O cancelamento preservou o disco de destino e a instalação válida iniciou os dois slots com o NVMe vizinho intacto. Relatórios em `out/pc-apps-iso/tests/install-cagrlxdw/` e `out/pc-apps-iso/ventoy/install-jqfzqjq_/`.

`make test` passou com 48 testes Python e cobertura de 80,1% em Go, 92,0% em Python e 97,2% em Shell. [Comandos](TESTES.md#perfil-de-aplicativos-do-pc)

### Desktop padrão e reprodução de áudio

`make pc-desktop-layer pc-chromium-layer` montou as camadas com Podman rootless, APKs e DEBs verificados pelos hashes fixados. O Buildroot executou `linux-reconfigure` no contêiner Fedora 45 e gerou o core com as três camadas, reutilizando a toolchain do PC e a camada já compilada de Umbriel e Noctalia. `make pc-disk OUT=out/pc-desktop-next/core` gravou somente o disco virtual de laboratório.

`make test-pc-desktop OUT=out/pc-desktop-next/core PC_DESKTOP_APPS=1 PC_DESKTOP_CHROMIUM=1 PC_DESKTOP_UMBRIEL=1 PC_DESKTOP_AUDIO=1` confirmou Umbriel e Noctalia pelo comando `melkor-desktop`, Alacritty com Nushell e Firefox como padrão HTTP e HTTPS. Firefox ESR, Brave Origin e Blanc abriram fluxos de reprodução. O QEMU gravou amostras da saída HDA virtual, e o Pulse rejeitou captura e carregamento de módulos. A sessão Weston também iniciou com `--weston`. Logs e gravação em `out/pc-desktop-next/core/desktop-tests/desktop-6ywl500p/`.

`make installer-iso OUT=out/pc-desktop-next/ventoy-ready PC_OUT=out/pc-desktop-next/core` gerou a ISO de 951 MiB com SHA-256 `91ff6077fac4f03280ff78f9222f09c191d642365c14b721c724e14e7d921fcb`. O teste `make test-installer TEST_VM_MEMORY=8192` e a variante `tests/installer_vm.py --ventoy-drive` usaram a mesma ISO, com QEMU, OVMF, TCG e setores lógicos de 512 bytes. A mídia inicializou por CD UEFI e pelo modo normal do Ventoy. Nos dois casos a instalação confirmou A e B, e os hashes do NVMe vizinho conferiram. Relatórios em `out/pc-desktop-next/ventoy-ready/tests/`.

`make test` passou com 49 testes Python e cobertura de 80,1% em Go, 92,0% em Python e 97,0% em Shell. O teste de áudio no HP, os portais de arquivos, o desbloqueio no Noctalia e a biometria continuam pendentes.

### Candidata do PC com as chaves oficiais

O Buildroot recompôs o core de revisão 1 com a toolchain PC já validada e as três camadas de aplicativos. Os arquivos públicos do RAUC e do canal Ed25519 foram comparados com os PEM extraídos do SquashFS. `make installer-iso OUT=out/pc-official-prep/installer PC_OUT=out/pc-official-prep/verified-candidate` gerou a candidata para uso local. `make bundle-pc` e `make channel-pc` assinaram os artefatos. O SHA-256 da ISO é `c50bcd588127e4a5cbd7c8b411854f1be85116d4a4b121a3326064913f7a2057` e tem assinatura Ed25519 separada.

Com OVMF e TCG, `tests/installer_vm.py --ventoy-drive` exercitou o pendrive virtual em modo normal, confirmou a instalação e os dois boots e preservou o disco vizinho. `make test-pc-desktop` verificou Umbriel, Noctalia, Alacritty, Nushell e os três navegadores com áudio em KVM. Os registros estão em `out/pc-official-prep/`. A publicação da imagem com os navegadores permanece pendente da revisão dos termos de redistribuição.

O teste de atualização por TLS local em QEMU reconheceu a revisão oficial no manifesto Ed25519, rejeitou uma assinatura adulterada e instalou o bundle RAUC oficial pela opção `--https`. O slot B iniciou e preservou o documento gravado em A. Os registros estão em `out/pc-official-prep/signed-update-verified/`.

### Primeira publicação do PC Core

O Buildroot reconfigurou o alvo PC com `MELKOR_PC_MINIMAL=1`, recompôs a raiz de destino e gerou a revisão 1 com as chaves oficiais. A ISO de 157 MiB passou pelo Ventoy em modo normal no QEMU. O instalador iniciou os slots A e B e preservou o disco vizinho. O SHA-256 da ISO é `394676632894fa711b2364462b257a27356798ae0a2e5bc497ad2224d495dd7c`. [Release](https://github.com/wkoch/Melkor/releases/tag/pc-v0.1.0-alpha.4-core)

`make bundle-pc` produziu o bundle RAUC assinado. A publicação conferiu seu download anônimo antes de publicar o manifesto gerado por `make channel-pc`. A VM do PC Core confirmou Quad9 DoH e reconheceu a revisão 1 pelo endereço oficial do GitHub. `make test` passou na CI com 80,1% em Go, 92,0% em Python e 97,3% em Shell. [Canal estável](https://github.com/wkoch/Melkor/releases/tag/pc-stable) · [Execução da CI](https://github.com/wkoch/Melkor/actions/runs/44941139935/job/136958394728)

### Login gráfico automático na edição completa

`make pc-desktop-layer pc-session-layer` montou o runtime Alpine com APKs fixados e compilou Umbriel, Noctalia e gtkgreet em Podman rootless. O Buildroot reaproveitou a toolchain PC do contêiner Fedora 45 e gerou o SquashFS com as três camadas e as chaves públicas oficiais. `make installer-iso OUT=out/pc-graphical-login/installer PC_OUT=out/pc-graphical-login/core` gerou a ISO local de 951 MiB. O SHA-256 `ff946fd8bb5bbc5ca7cc8b4ba18864b533c4c2af8ddea652f9c17f7eaeb75aa7` foi verificado com assinatura Ed25519.

`make pc-disk OUT=out/pc-graphical-login/core` gerou o disco virtual, e `make test-pc-desktop OUT=out/pc-graphical-login/core PC_DESKTOP_LOGIN=1 PC_DESKTOP_APPS=1` passou em QEMU com KVM e OVMF. A tela apareceu no VT1, a senha errada e o login root foram recusados e Umbriel e Noctalia abriram como usuário comum. Alacritty com Nushell e Firefox ESR abriram. Encerrar Umbriel trouxe a tela de volta. O teste alternou para o console no VT2 e retornou ao VT1.

`tests/installer_vm.py --ventoy-drive … --memory-mib 8192 --graphical-login` iniciou a mesma ISO pelo Ventoy 1.1.18 em modo normal com QEMU, OVMF e TCG. A confirmação inválida preservou o destino, a instalação usou o NVMe virtual e o disco vizinho manteve seu SHA-256. Os slots A e B iniciaram com a mídia removida. No teste gráfico em KVM, a senha de 128 bytes criada na instalação abriu a sessão. Firefox ESR, Brave Origin e Blanc reproduziram áudio na HDA virtual, e a política Pulse bloqueou captura e carregamento de módulos. Registros e capturas em `out/pc-graphical-login/installer/ventoy-tests/install-4e0jyrh3/`.

`make test` passou com 52 testes Python e cobertura de 80,8% das instruções Go, 92,0% da métrica Python e 97,5% das linhas Shell. Relatórios em `out/coverage/`. A validação física no HP continua pendente.

### Noctalia Greeter na edição desktop

Em 22 de novembro de 2026, `make pc-desktop-layer pc-session-layer` montou as camadas com Podman rootless. `make image BOARD=pc` compilou o core no contêiner Fedora 45. A finalização do Buildroot foi retomada quando a camada de sessão ficou disponível. O gerador de disco precisou de `GCONV_PATH=/usr/lib64/gconv` no contêiner para o mcopy converter a página de código 850. Os três artefatos do core passaram pela conferência de SHA-256.

O comando `make test-pc-desktop OUT=out/pc-noctalia-greeter/core PC_DESKTOP_LOGIN=1 PC_DESKTOP_APPS=1 PC_DESKTOP_CHROMIUM=1 PC_DESKTOP_AUDIO=1` verificou no QEMU com KVM e OVMF a recusa de senha incorreta, o login no Noctalia Greeter, o retorno após encerrar Umbriel e a alternância entre login e console de recuperação. Alacritty abriu com Nushell, e Firefox ESR, Brave Origin e Blanc reproduziram áudio na placa HDA virtual. Evidências em `out/pc-noctalia-greeter/core/desktop-tests/desktop-geg2a36_/`.

Em uma imagem de laboratório recomposta, a mesma combinação de Cage e Noctalia Greeter passou com uma senha de 128 bytes e completou onze idas e voltas entre VT1 e VT2 em outro teste. `make test` passou com 81,0% em Go, 92,1% em Python e 97,7% em Shell.

### Luciole na edição desktop

Em 22 de novembro de 2026, `make pc-desktop-layer pc-session-layer` montou o runtime e compilou os quatro componentes do Noctalia em Podman rootless. O Buildroot recompôs o SquashFS com a toolchain do PC já validada. `make pc-disk OUT=out/pc-noctalia-luciole/core` com `GCONV_PATH=/usr/lib64/gconv` no contêiner gerou o disco de laboratório.

`make test-pc-desktop OUT=out/pc-noctalia-luciole/core PC_DESKTOP_LOGIN=1 PC_DESKTOP_APPS=1 PC_DESKTOP_CHROMIUM=1 PC_DESKTOP_AUDIO=1` passou em QEMU com OVMF e KVM. O executor confirmou o login, Umbriel e Noctalia Shell, a abertura de Luciole por `Super+E`, o MIME de diretórios, `FileManager1.ShowFolders` e a ativação do backend `FileChooser`. Também verificou o terminal, os três navegadores, o áudio e o retorno do greeter. Evidências em `out/pc-noctalia-luciole/core/desktop-tests/desktop-_g0phblz/`. `make test` passou com 81,0% em Go, 92,1% em Python e 97,8% em Shell.

Uma sondagem separada no disco virtual consultou `/sys/power/state` e `/sys/power/mem_sleep`, escolheu `deep` e escreveu `mem` como root. O QMP confirmou o estado suspenso e aceitou `system_wakeup`. O convidado manteve a tela preta e o console serial não confirmou a retomada no prazo do executor. O acionamento automático pela tampa e a hibernação continuam pendentes. Registros em `out/pc-noctalia-luciole/probe-suspend-retry.log`.

### Primeira publicação do canal de desktop

Em 22 de novembro de 2026, `make bundle-pc` assinou o SquashFS testado da revisão 2. `make channel-pc CHANNEL_DESKTOP=1` verificou o bundle e assinou o manifesto publicado em `publicacao/canal/`. A ISO mínima foi recomposta a partir do instalador com seleção numérica. A revisão 1 recebeu a configuração do canal, `melkor-finalizar` e os dois executáveis do fluxo de atualização do build testado. O SHA-256 da ISO foi assinado com Ed25519. [Release do PC Desktop](https://github.com/wkoch/Melkor/releases/tag/pc-v0.1.0-alpha.5-desktop)

O executor `tests/installer_vm.py --desktop-bundle … --memory-mib 8192` instalou a ISO, iniciou os dois slots e aplicou o bundle assinado. O teste gráfico usou a senha criada na instalação e abriu o Luciole. Duas VMs adicionais baixaram o pacote público, uma via `melkor-finalizar` e outra via `melkor-core-update --https` da revisão pública anterior. As duas aplicaram a revisão 2. O RAUC confirmou o slot B após a reinicialização. Evidências em `out/pc-noctalia-luciole/core/` nos relatórios `release-iso-tests/install-dcqf4c95/result.json`, `rede-final.log` e `rede-legado.log`.

### Instalação mínima com seleção numérica

`make image BOARD=pc` passou a selecionar o core mínimo quando não há camadas gráficas. O candidato local reutilizou o kernel, o initramfs e a entrada UEFI da ISO mínima assinada. Uma camada cpio inseriu `melkor-instalar` recompilado com o Go do host e o SquashFS base recebeu `melkor-finalizar`. O SquashFS foi reempacotado com `unsquashfs -pf` e `mksquashfs -pf`, com metadados e chaves de confiança conferidos. O xorriso 1.5.8 do Alpine remontou a ISO de teste. O SHA-256 firmado dessa mídia é `ea2b0de13beb7734fcdb8be6b4b2f9be3a2bfb652fc6f81de02940c94f2ac433`.

`scripts/bundle-m1.py` gerou `melkor-desktop.raucb` com o RAUC 1.13 extraído do core já validado. O artefato local reutiliza o SquashFS do desktop aprovado no teste anterior. `rauc info --keyring` verificou a assinatura oficial e o formato verity.

`tests/installer_vm.py --ventoy-drive … --desktop-bundle …` passou com a ISO final no QEMU, com OVMF e Ventoy 1.1.18. A confirmação inválida preservou o destino e o NVMe vizinho manteve seu SHA-256. A base iniciou por A e por B. `melkor-finalizar` promoveu o desktop pelo RAUC. Após o reboot, a senha provisionada de 128 bytes abriu Umbriel e Noctalia, e os três navegadores reproduziram áudio na HDA virtual. A mesma ISO passou também como CD UEFI com setores lógicos de 4096 bytes. Registros em `out/pc-instalacao-minima/evidence-final/` e `out/pc-instalacao-minima/evidence-4kn/`.

`make test` passou com 53 testes Python e cobertura de 80,8% em Go, 92,0% em Python e 97,6% em Shell. O novo comando de finalização atingiu 100% das linhas instrumentadas.

### Preparação do canal HTTPS do desktop

`melkor-finalizar` ganhou um canal exclusivo com manifesto assinado. O teste TLS local confirmou o download do bundle pelo SHA-256 e a recusa de manifestos trocados entre a base e o desktop. `make test` passou com 81,0% em Go, 92,1% em Python e 97,7% em Shell. O diretório `publicacao` ficou configurado para o Netlify. A ativação depende do domínio, da publicação do bundle desktop e de uma imagem que inclua o cliente atualizado.
