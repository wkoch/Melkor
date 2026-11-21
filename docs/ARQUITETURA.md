# Arquitetura e decisões do Melkor

## Receita e geração

O repositório do Melkor define quais fontes originais usar, como compilá-las e quais artefatos publicar. A inspiração no Chimera abrange musl, LLVM/Clang, libc++, utilitários de origem FreeBSD e dinit. O primeiro construtor a avaliar é o Buildroot com receitas próprias em `BR2_EXTERNAL`. Seu formato nativo basta para o protótipo. A seleção exata de utilitários, o bootstrap LLVM e as receitas do desktop serão validados por etapas. [Chimera](https://chimera-linux.org/about/) · [Manual do Buildroot](https://buildroot.org/downloads/manual/manual.html)

| Entrada versionada | Conteúdo |
| --- | --- |
| Receita do core | Arquitetura x86-64, configuração do kernel, libc, init, serviços, firmware necessário ao alvo, desktop e arquivos de configuração de fábrica. |
| Entradas fixadas | Versões e hashes das fontes, patches, ferramentas de montagem, toolchain, opções de compilação e dependências transitivas resolvidas. O formato pode acompanhar o construtor. |
| Perfil de trabalho | Catálogo autorizado, aplicativos instalados por padrão e versões de ferramentas auxiliares, como servidores de linguagem. |
| Saída do build | Imagem inicializável, manifesto de pacotes e arquivos, hashes dos artefatos e vínculo com o commit de origem. Chaves privadas de assinatura ficam fora do repositório. |

A atualização do core parte de um diretório de compilação limpo. Um cache de fontes verificadas pode evitar novos downloads. Fixar entradas permite investigar divergências; a reprodução byte a byte requer duas construções independentes e comparação dos artefatos antes da assinatura.

## Atualizar, instalar e voltar

1. **Core.** O slot ativo contém uma geração montada somente para leitura. O atualizador valida a autenticidade e a compatibilidade da imagem antes de ativar o outro slot. Kernel, initramfs, módulos e rootfs precisam permanecer associados à mesma geração.
2. **Boot.** O GRUB mantém a seleção manual das gerações e registra a tentativa da candidata. O dinit é o PID 1. Os serviços de boot montam os dados, iniciam o console e confirmam o slot ativo com RAUC. Quando houver desktop, a confirmação dependerá também de uma sessão utilizável. [Integração do RAUC 1.13](https://rauc.readthedocs.io/en/v1.13/integration.html)
3. **Aplicativos.** Uma receita conhecida baixa fontes verificadas, resolve as dependências, compila sem privilégios em área isolada e testa o resultado. O instalador prepara os arquivos nativos e troca o perfil ativo em uma operação atômica. `melkor voltar` verifica os arquivos e as imagens locais da geração escolhida antes de trocar o mesmo link, sob o mesmo lock de instalações e remoções. Os dados de aplicativos ficam fora das gerações. A limpeza preserva artefatos referenciados pelas gerações retidas.
4. **Receitas novas.** A IA pode pesquisar e propor uma receita para revisão e teste. Depois de aceita, a receita entra no catálogo e pode ser executada sem o modelo. Drivers, módulos de kernel e serviços de login entram pela receita do core.
5. **Dados.** Cada geração referencia um perfil compatível. Documentos e preferências do usuário persistem. Configurações essenciais ao boot precisam de defaults por geração e de um tratamento explícito para alterações locais. Migrações de formato em dados persistentes requerem compatibilidade de leitura ou cópia recuperável antes da migração.

### Disco de laboratório do M1

| Partição GPT | Conteúdo | Uso |
| --- | --- | --- |
| `esp` | VFAT, 64 MiB | GRUB x86-64 UEFI, menu de seleção e bloco de estado persistente em `/EFI/BOOT/grubenv`. |
| `core-a` | SquashFS, 128 MiB | Kernel em `/boot/bzImage`, configurações de fábrica e restante do core. A raiz é montada somente para leitura. |
| `core-b` | SquashFS, 128 MiB | Segundo slot com o mesmo formato. A imagem de fábrica inicializa A e B com a mesma geração. |
| `persist` | ext4, 128 MiB | Montada por UUID em `/persist`. O diretório `/persist/melkor` fica acessível em `/home/melkor`. |

O GRUB usa o disco da ESP e seleciona a partição do kernel. A mesma entrada passa o `PARTUUID` do slot ao kernel e informa `rauc.slot=A` ou `rauc.slot=B`. O dinit monta os dados antes de iniciar o login serial. Esses caminhos e os UUIDs fixos compõem a receita da VM de disco único. A atribuição de UUIDs a instalações reais fica para o instalador.

A imagem de fábrica inicia A por padrão. Antes de carregar o kernel, o GRUB registra a tentativa em um bloco prealocado na ESP. No boot seguinte, escolhe o primeiro slot de `ORDER` com `<slot>_OK=1` e `<slot>_TRY=0`. Quando nenhum slot está apto, mantém o menu aberto para seleção manual. A confirmação do dinit limpa a tentativa após montar os dados e iniciar o console.

O RAUC 1.13 do core valida a assinatura CMS e o conteúdo do bundle no formato `verity`, verifica `compatible=melkor-m1-x86_64` e grava o SquashFS no slot inativo. Só altera a prioridade de boot ao concluir a instalação. A assinatura protege o processo de atualização. A verificação da cadeia de confiança do firmware até o boot do core é uma etapa distinta. [Configuração do RAUC 1.13](https://rauc.readthedocs.io/en/v1.13/reference.html) · [Bloco de ambiente do GRUB](https://www.gnu.org/software/grub/manual/grub/html_node/Environment-block.html)

O Buildroot 2025.02.20 fornece a receita, o hash de fonte e os patches do RAUC 1.13, do GRUB 2.12 e do util-linux 2.40.4. RAUC entra no core como atualizador e no ambiente de build como assinador. `grub-editenv` escreve o estado de boot e `blkid` identifica as partições na VM. A imagem confia nos certificados públicos fornecidos a `MELKOR_RAUC_KEYRING` no build. A chave privada fica no ambiente de assinatura.

### Disco de laboratório do PC

O alvo `pc` usa uma ESP de 128 MiB, dois slots de 2 GiB e ext4 no espaço disponível para dados. O gerador Go cria UUIDs novos, valida o SHA-256 do SquashFS e publica o arquivo de disco após completar a preparação. O GRUB encontra a raiz pelo `PARTUUID` do slot. No boot, `melkor-storage` encontra o disco-pai da raiz montada no `sysfs` e resolve os quatro dispositivos pela ordem e pelos nomes GPT. O dinit monta os dados antes do console e ativa um arquivo de swap de 4 GiB. O identificador RAUC do alvo é `melkor-pc-x86_64`.

Na edição com Umbriel, o dinit inicia o greetd depois de montar os dados e iniciar o seatd. A conta de serviço `greeter` executa o compositor Cage e o gtkgreet no VT1. O greetd usa PAM com o `/etc/shadow` provisionado pelo instalador, aceita a conta comum `melkor` e abre Umbriel com Noctalia. O VT2 mantém o console de recuperação. A edição PC Core usa o console no VT1. A confirmação RAUC do PC ainda usa o console como critério de saúde.

O atualizador HTTPS do PC usa a biblioteca padrão da mesma toolchain Go da receita `melkor-storage` e as autoridades de certificação do core. A sessão root informa uma URL HTTPS e o SHA-256 do bundle. O comando mantém a trava de atualização durante o download e a instalação, limita o tamanho recebido e exige HTTPS também nos redirecionamentos. O arquivo temporário fica na área privada de root em `/persist/rauc`. Após validar o SHA-256, o comando entrega o arquivo ao RAUC para verificar a assinatura, a compatibilidade e o conteúdo antes da promoção. O download expõe ao operador a seleção explícita da versão a instalar.

Sem argumentos, o comando lê a URL de fábrica de `/etc/melkor-update/channel` ou a substituição administrada por root em `/persist/rauc/channel`. O manifesto tem um envelope JSON com os bytes do conteúdo e a assinatura Ed25519, ambos em Base64. O conteúdo assinado define o esquema, o alvo PC, a revisão inteira, a validade, a URL HTTPS e o SHA-256 do bundle. O cliente valida a assinatura antes de interpretar esses campos. A revisão do core é gravada no SquashFS e segue o slot que iniciou. O canal rejeita revisões anteriores à que está rodando e relata que a instalação está atualizada quando as revisões coincidem. A troca de URL preserva a chave pública do canal e o chaveiro RAUC da imagem. O comando explícito para um bundle permanece disponível para seleção manual.

Para completar uma instalação mínima, `melkor-finalizar` consulta o manifesto indicado em `/etc/melkor-update/desktop`. O manifesto precisa incluir o campo assinado `profile: desktop`. A chave do canal, o controle de revisão, o limite de download e a validação RAUC são os mesmos do atualizador. Se receber um rótulo de mídia como argumento, o comando monta a partição FAT32 somente para leitura e copia o bundle para a área privada antes de chamar o RAUC. O desktop entra no slot inativo, com o core mínimo disponível na geração anterior.

Ao provisionar contas, o gerador grava os hashes de root e de `melkor` em um diretório privado de root na ext4. Antes do login, `melkor-storage` verifica a propriedade e as permissões, compõe o shadow com as contas de serviço da geração e monta o arquivo resultante sobre `/etc/shadow`. A troca de slot usa os mesmos dados. Uma provisão presente e inválida impede o login.

Para exercitar vídeo e entrada no PC, o core de laboratório inclui eudev 3.2.14, seatd 0.9.1, Weston 14.0.1 e foot 1.16.2, das receitas do Buildroot fixado. A sessão começa como usuário comum e usa DRM com Pixman. O módulo AMDGPU e o firmware do kernel entram no core para os testes do HP. A sessão prevista para uso diário integra Umbriel e Noctalia. O alvo PC atual usa credenciais de laboratório e é preparado em um arquivo regular para QEMU. [Requisitos de empacotamento do Umbriel](https://github.com/noctalia-dev/umbriel/blob/main/PACKAGING.md)

### Mídia de instalação do PC

O alvo `installer` gera uma mídia UEFI separada que inclui kernel, dinit e `melkor-instalar`. O gerador da ISO compara o SHA-256 dos componentes dos dois builds. O SquashFS do PC, o GRUB do disco e seu modelo de configuração ficam dentro do initramfs. A instalação usa esses arquivos em RAM e funciona sem conexão de rede ou montagem da partição de dados do Ventoy.

O instalador lista discos internos identificados pelo eudev. A seleção numérica resolve para a mesma entrada com ID persistente, modelo, capacidade e sequência do disco. A confirmação `APAGAR` se aplica à unidade exibida nessa etapa. A imagem temporária usa a capacidade e o tamanho de setor do destino e recebe as duas credenciais antes da gravação. A abertura exclusiva do dispositivo recusa volumes em uso. O instalador reconfirma a sequência do disco, a capacidade e o setor lógico pelo descritor aberto antes de invalidar a GPT anterior. A GPT nova só é publicada após a cópia e a sincronização dos dados. [Abertura exclusiva de dispositivos de bloco no Linux](https://man7.org/linux/man-pages/man2/open.2.html)

O instalador reutiliza util-linux 2.40.4 e as receitas fixadas de e2fsprogs 1.47.2, dosfstools 4.2 e GNU Mtools 4.0.48 do Buildroot. Esses formatadores entram na mídia dedicada. A ESP tem 128 MiB para setores de 512 bytes e 512 MiB para 4Kn, para acomodar FAT32. Um loop de setor lógico de 4096 bytes é usado exclusivamente na imagem intermediária 4Kn e removido logo após escrever a GPT.

### Rede e DNS do core

O dinit ativa a interface de loopback e supervisiona o `dhcpcd` 10.5.2, que configura endereços e rotas com separação de privilégios e guarda as concessões em `/run`. O cliente roda com os hooks de DHCP desativados. O `/etc/resolv.conf` faz parte do SquashFS de cada slot e aponta para o resolvedor de loopback.

O `dnsproxy` 0.86.0 roda com a conta dedicada `melkor-dns`. `setpriv`, do util-linux 2.40.4, limita o processo a `CAP_NET_BIND_SERVICE` para abrir a porta 53 local. O proxy autentica `dns.quad9.net` com o conjunto de certificados do Buildroot. Os DNS stamps incluem os IPs de conexão e o caminho `/dns-query`, eliminando a consulta DNS antes de abrir a conexão HTTPS. O resolvedor do sistema mantém essa configuração se o acesso ao Quad9 cair. A toolchain Go 1.27.3 fica no ambiente de build. O executável estático incorpora as dependências Go vendorizadas do upstream. [dnsproxy](https://github.com/AdguardTeam/dnsproxy/releases/tag/v0.86.0) · [dhcpcd](https://github.com/NetworkConfiguration/dhcpcd/releases/tag/v10.5.2) · [Quad9](https://docs.quad9.net/services/)

O core inclui `ca-certificates` 20260816+nmu1 e `libcap-ng` 0.8.5 das receitas do Buildroot. Python 3.12.16 é uma dependência do host para montar o conjunto de certificados. O `tcpdump` do Fedora é usado no ambiente de testes para ler as capturas do QEMU.

O Live e o PC usam a mesma configuração de Quad9 do M1. A receita fixada do Buildroot acrescenta IWD 3.0 ao Live e ao PC para associação Wi-Fi via D-Bus. A associação é acessível a usuários do grupo `netdev`, e o DHCP continua a cargo do `dhcpcd`. O Live guarda as redes em `/run/iwd`. O disco PC guarda as redes em `/persist/iwd`, diretório acessível apenas a root.

### Mídia de diagnóstico em RAM

O alvo `live` gera uma ISO com entrada UEFI El Torito e uma imagem GPT com uma única ESP. O GRUB carrega o kernel e um initramfs comprimido. O dinit abre o login em `tty1` e no primeiro console serial. O sistema monta os sistemas de arquivos virtuais necessários para o console e mantém o home em RAM. Root permanece bloqueado. O kernel disponibiliza framebuffer EFI, armazenamento USB e NVMe, AHCI e HID para sondar os dispositivos.

O GNU xorriso 1.5.4.pl02 da receita do Buildroot roda no host de build. A imagem de boot EFI na ISO tem 30 MiB para que o catálogo El Torito registre seu tamanho em setores de 512 bytes. O kernel, o initramfs e a configuração do GRUB também estão no sistema de arquivos ISO 9660. [Formato El Torito no xorriso](https://www.gnu.org/software/xorriso/man_1_xorrisofs.html)

As receitas do Buildroot fixado fornecem `iw` 6.9 e `libnl` 3.11.0 para a enumeração das interfaces. `linux-firmware` 20250211 entra na mídia com a seleção `22260` para o Intel AX200 e os arquivos `rtw88` para placas Realtek. O Buildroot verifica os hashes das fontes. O kernel e os utilitários do Live entram apenas nessa mídia. [Receitas no Buildroot fixado](https://gitlab.com/buildroot.org/buildroot/-/tree/2025.02.20/package) · [Opções Intel na família HP](https://jp.ext.hp.com/notebooks/business/probook_x360_435_g7/) · [Suporte Realtek listado pela HP](https://support.hp.com/soar-attachment/977/col94858-ob-276296-1-ob-276296-1_sp121426_releasedoc.html)

O kernel Live habilita interrupções PCI MSI e incorpora `iwlwifi-cc-a0-77.ucode`, a versão requerida pelo `iwlwifi` 6.12 para o AX200. A extensão de build copia o arquivo da receita `linux-firmware` após a verificação da fonte. Ela também incorpora `regulatory.db` e `regulatory.db.p7s`, fornecidos pela receita `wireless-regdb` 2026.09.03. O `cfg80211` valida a assinatura da base. Os três arquivos também ficam acessíveis no initramfs. [Firmware incorporado no Linux](https://docs.kernel.org/6.12/driver-api/firmware/built-in-fw.html)

## Política de dependências

| Camada | Política |
| --- | --- |
| Build e testes | Python e outras ferramentas exigidas por projetos originais podem rodar no ambiente descartável. Registre essas dependências. A imagem final leva apenas componentes necessários ao uso. |
| Core e sessão padrão | Usar musl como libc. Priorizar binários nativos e buscar zero execução obrigatória de runtimes de linguagens dinâmicas no boot, sessão ociosa e administração normal. Auditar scripts de inicialização, plugins e VMs embarcadas. Registrar cada dependência restante, o fluxo que a aciona e o teste para sua remoção. |
| Gráficos | Usar Wayland, sem servidor Xorg, com Xwayland desativado. O build atual do Umbriel exige componentes XCB e suporte a Xwayland em wlroots. Registrar essas bibliotecas no manifesto e validar a sessão com o servidor Xwayland ausente. [Empacotamento do Umbriel](https://github.com/noctalia-dev/umbriel/blob/main/PACKAGING.md) |
| Aplicativos | Um aplicativo pode ter dependências privadas declaradas. O motor JavaScript do navegador é um exemplo. Identificar plugins ou ferramentas que façam downloads na primeira execução. |

Noctalia usa Luau para extensões. WirePlumber pode usar Lua no roteamento de áudio e polkit avalia regras em JavaScript. A auditoria deve verificar o que efetivamente roda no perfil selecionado. O teste de um recurso continua obrigatório quando se propõe remover seu interpretador. [Plugins do Noctalia](https://docs.noctalia.dev/) · [WirePlumber](https://pipewire.pages.freedesktop.org/wireplumber/scripting/custom_scripts.html) · [polkit](https://polkit.pages.freedesktop.org/polkit/polkit.8.html)

### Construção e linkagem de aplicativos

Cada receita identifica a origem oficial, a versão ou o commit, o hash verificável, o construtor, a toolchain e os arquivos a ativar. Cargo atende executáveis Rust obtidos de crates.io ou Git. Projetos Go, Zig, C/C++ e assembly seguem o build suportado em cada versão. A descoberta de uma versão inicia um build candidato. A geração ativa só muda depois da validação.

A interface proposta expõe `melkor instalar <nome>` e `melkor atualizar`. Para uma entrada desconhecida, a descoberta pode sugerir a origem e gerar uma receita para revisão e teste antes de incorporá-la ao catálogo. Manter o Nushell como shell interativo do Alacritty e um `/bin/sh` compatível com POSIX para scripts e recuperação, disponível fora do perfil de aplicativos.

O PC entrega Nushell 0.117.0 pelo binário estático musl do upstream com hash fixado no Buildroot. O terminal gráfico usa Nushell e o login de console mantém `/bin/sh`. As camadas opcionais do perfil PC entram no SquashFS em `/opt`. A camada Alpine 3.24 inclui Alacritty 0.17.0 e Firefox ESR 153.7.0. A camada Debian 13 fornece dependências glibc privadas a Brave Origin 1.100.58 e Blanc 1.51.0. Bubblewrap monta a raiz de cada navegador somente para leitura, com um home gravável exclusivo, o socket Wayland e o resolvedor do sistema. O Firefox usa esse resolvedor para seguir o Quad9 DoH do core. As duas camadas são montadas no ambiente de build com o acesso de rede do instalador de pacotes desligado. [Nushell](https://github.com/nushell/nushell/releases/tag/0.117.0) · [Política de DNS do Firefox](https://mozilla.github.io/enterprise-admin-reference/reference/policies/dnsoverhttps/)

A camada de sessão compila Umbriel no commit `c634d946` e Noctalia 5.6.0 com o SDK Alpine fixado. O compositor usa seatd, Xwayland desativado e layout de teclado brasileiro. A montagem de `/dev/shm` no PC permite que o compositor publique o formato de buffers Wayland. Weston permanece disponível na imagem. [Empacotamento de Umbriel](https://github.com/noctalia-dev/umbriel/blob/main/PACKAGING.md) · [Noctalia 5.6.0](https://github.com/noctalia-dev/noctalia/releases/tag/v5.6.0)

A camada Alpine também fixa PipeWire 1.6.8 e WirePlumber 0.5.15. Os processos de áudio acompanham a sessão do usuário comum e usam ALSA pelas permissões do grupo `audio`. Os navegadores acessam o socket Unix do servidor Pulse da sessão. O servidor bloqueia a criação de fluxos de captura e o carregamento de módulos pelo protocolo. O Firefox usa a biblioteca cliente Pulse da camada Alpine e os navegadores Chromium usam a da camada Debian. [Protocolo Pulse do PipeWire](https://docs.pipewire.org/page_module_protocol_pulse.html)

LLVM/Clang é a escolha padrão para builds C/C++. A receita pode selecionar Zig para facilitar a compilação cruzada com musl ou por um ganho demonstrado. Ao comparar compiladores, manter versão do programa, alvo de CPU e otimizações equivalentes. Medir tempo de build separadamente de tempo de abertura, memória e duração de uma tarefa representativa.

Preferir linkagem estática para ferramentas de console e resgate, quando os recursos usados forem preservados. Para o desktop, declarar as bibliotecas e módulos carregados em execução, inclusive drivers gráficos. Registrar as bibliotecas incorporadas ao binário para identificar quais aplicativos reconstruir após uma correção.

O primeiro fluxo do M3 usa o CLI estático escrito em Go e uma receita de Cargo para o Nushell. Go e Podman são ferramentas do host. A imagem OCI fixada fornece a toolchain descartável. O catálogo inicial ativa executáveis ELF x86-64 estáticos no perfil. A receita enumera os arquivos e os comandos de validação. O CLI serializa as alterações do perfil, retém o manifesto de cada geração e só muda o link ativo após validar a candidata inteira.

O catálogo compartilhado vive em outro repositório. A máquina sincroniza uma tag assinada com Git e OpenSSH e usa seu conteúdo local para resolução. A ativação do catálogo é atômica e independente da troca de geração dos aplicativos. A IA pode produzir e testar uma proposta local. Após a confirmação do usuário, a receita aceita pode ser publicada em um PR. O cliente de publicação usa Git e GitHub CLI para enviar apenas o JSON da receita. [Repositório de receitas](../catalogo/README.md)

### Aplicativos distribuídos em DEB ou RPM

O CLI copia e confere o SHA-256 do arquivo local antes de chamar Podman rootless. `dpkg-deb` ou `rpm` consultam nome, versão e arquitetura dentro de uma imagem da distribuição compatível. `apt-get` e `dnf` baixam as dependências dos repositórios da base fixada sem executar a transação. Uma segunda construção parte da mesma base e instala a entrada e as dependências já obtidas, com a saída de rede desligada e os repositórios desativados. O DNF verifica a assinatura dos RPMs locais, usando a chave do Fedora ou uma chave pública indicada pelo usuário. [apt-get](https://manpages.debian.org/stable/apt/apt-get.8.en.html) · [Assinaturas no DNF5](https://dnf5.readthedocs.io/en/latest/dnf5.conf.5.html) · [Podman build](https://docs.podman.io/en/stable/markdown/podman-build.1.html)

As bases OCI usadas pelos aplicativos são `docker.io/library/python@sha256:d98cc29d3266e535dd9da1099688d9cb3ca4753813571e071763d17725460962` para Debian 13 e `registry.fedoraproject.org/fedora@sha256:130bdf7ec68a9834ae377c574c3f22992ae103197ef951deaac48e1e1eb7cf2d` para Fedora 45. As dependências resolvidas vêm do estado dos repositórios no momento da importação. O manifesto guarda o digest da base, o SHA-256 da entrada, o ID da imagem exata produzida e o caminho do executável.

O importador verifica o executável e pode rodar um teste fornecido pelo usuário em um home temporário antes de ativar a geração. Na execução, o CLI usa a raiz somente para leitura, sem rede, sem capacidades e com o diretório de dados exclusivo do aplicativo. A geração e suas imagens permanecem disponíveis após uma falha ou remoção. A integração gráfica depende do desktop e concederá Wayland, áudio e portais por aplicativo. [Podman run](https://docs.podman.io/en/latest/markdown/podman-run.1.html)

O primeiro alvo são aplicativos de usuário para uma distribuição e arquitetura declaradas. A base precisa ser compatível até dentro da mesma família de empacotamento. Dependências indisponíveis e recursos incompatíveis devem produzir erro. Drivers, módulos do kernel e serviços que precisam integrar o boot do Melkor seguem pelas receitas do core.

## Seleção inicial

| Destino | Componentes | Uso que valida a seleção |
| --- | --- | --- |
| Core de desktop | Umbriel, Noctalia, Luciole e Alacritty com backend Wayland | Login, lançador, notificações, abrir e salvar pelo portal, terminal com teclado ABNT2. |
| Core e mídia de resgate | Kibi como editor de terminal | Editar e reabrir uma configuração em UTF-8, no console, sem rede ou desktop. |
| Perfil padrão | Nushell (shell interativo do Alacritty), Git, Firefox, Brave Origin, Blanc, Rhun, WordCraft e GridCraft | Revisar diffs, navegar na web, editar código e trabalhar com arquivos DOCX e XLSX reais. |
| Ferramentas de desenvolvimento | rust-analyzer e gopls | Definição e diagnóstico no Rhun para projetos Rust e Go. |
| Catálogo opcional | Fresh compilado sem plugins, aplicativos próprios e clientes de IA selecionados | Ativar e remover pelo perfil, mantendo os hashes do core. |

O desktop completo em musl e a integração do portal do Luciole são hipóteses a testar. Para o Rhun, valide o executável e as dependências das ferramentas externas que ele aciona. [Noctalia](https://noctalia.dev/) · [Rhun](https://github.com/vshvedov/rhun) · [Kibi](https://github.com/ilai-deutel/kibi) · [Fresh](https://github.com/sinelaw/fresh)

O instalador aplica os [padrões da primeira instalação](INSTALACAO.md) ao criar o disco, as contas e o perfil inicial. O catálogo de aplicativos versiona os navegadores. O driver de digital, a autenticação e os serviços de sessão integram o core.
