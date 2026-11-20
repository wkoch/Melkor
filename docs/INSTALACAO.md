# Padrões da primeira instalação

Este documento define o perfil de instalação para os PCs pessoais do William. O instalador de disco real pertence ao M4. A integração do leitor de digitais e dos navegadores depende da sessão Wayland do M2 e do perfil de aplicativos do M3.

## Decisões

| Área | Padrão |
| --- | --- |
| Disco | Instalação UEFI como sistema único. Usar todo o disco selecionado, com GPT, ESP, dois slots do core e dados persistentes no espaço restante. Identificar o disco por modelo, capacidade e ID estável e exigir confirmação da seleção antes de apagar partições. |
| Swap | Usar `/persist/.swap/swapfile` em ext4, fora do diretório pessoal, com acesso exclusivo de root. Alocar os blocos antes de ativá-lo. O tamanho pode ser ajustado depois sem reparticionar o disco. [Restrições de arquivos de swap](https://www.man7.org/linux/man-pages/man8/swapon.8.html) |
| Conta de trabalho | Criar um usuário comum com diretório pessoal próprio. Aplicativos escrevem no perfil do usuário. A conta fica sem acesso de escrita ao core, à ESP e aos dispositivos de bloco. |
| Administração | Usar uma sessão root explícita em console local, via `su -` com a senha de root definida na instalação. O perfil padrão dispensa `sudo`, `doas` e mecanismos equivalentes de delegação geral. Alterações do core continuam pelo fluxo de geração e atualização assinada. |
| Digital | Cadastrar a digital localmente e usá-la para login ou desbloqueio nos leitores suportados. Preservar a senha do usuário como alternativa. A autenticação de root usa sua própria senha. Validar o sensor e a versão do driver no notebook. [fprintd e PAM](https://fprint.freedesktop.org/) · [IDs de leitores suportados](https://fprint.freedesktop.org/supported-devices.html) |
| Navegadores | Incluir Firefox, Brave Origin e Blanc no perfil padrão versionado. Validar separadamente a origem, as dependências, o sandbox, o Wayland e as atualizações de segurança de cada navegador. [Firefox para Linux](https://support.mozilla.org/en-US/kb/install-firefox-linux) · [Brave Origin para Linux](https://brave.com/origin/linux/) · [Blanc para Linux](https://blanc.day/download) |
| DNS | Usar Quad9 seguro com DNS sobre HTTPS para todo o sistema, pelo endpoint `https://dns.quad9.net/dns-query`. Os navegadores de fábrica usam o resolvedor do sistema por padrão. [Serviços do Quad9](https://docs.quad9.net/services/) |

A atualização do core trabalha com a GPT existente e preserva o volume que contém os dados e o swap. O instalador gera identificadores próprios para cada máquina. Uma instalação completa e uma atualização A/B têm caminhos e testes distintos.

O perfil inicial dos navegadores deve estar pronto no primeiro login. A mídia padrão pode levar o perfil já compilado ou obtê-lo como artefato assinado durante a instalação. A compilação local segue sendo uma opção para quem quiser alterar as receitas.

### DNS do sistema

O core M1 usa o `dnsproxy` para servir DNS por UDP e TCP em `127.0.0.1` e `::1`. Ele se conecta aos quatro endereços publicados do Quad9 seguro e valida o certificado para `dns.quad9.net`. As consultas ficam indisponíveis quando nenhum endereço DoH responde. O `dhcpcd` configura endereços e rotas, e o `/etc/resolv.conf` somente leitura mantém o resolvedor local para os aplicativos que usam a configuração do sistema. A configuração de Wi-Fi e de exceções explícitas por conexão para portais cativos ou DNS interno depende do gerenciador de rede da instalação. [Endereços e modalidades do Quad9](https://docs.quad9.net/services/)

A validação da instalação inclui reconexão, comportamento com o relógio incorreto e consultas pelos navegadores de fábrica e pelos contêineres de aplicativos. Aplicativos com resolvedor próprio precisam de configuração que preserve a política da instalação. [Casos de teste do DNS](TESTES.md#quad9-no-resolvedor-do-sistema)

### Compatibilidade a demonstrar

O catálogo atual foi validado com executáveis estáticos x86-64. Os navegadores exigem empacotamento de binários e dependências privadas ou builds compatíveis com a base musl. A receita de Firefox do Alpine dá um ponto de partida para validar as dependências em musl. O Brave Origin distribui pacotes Linux para as famílias Debian e RPM, e o Blanc distribui AppImages com Electron. Cada receita deve preservar o sandbox suportado pelo navegador, abrir uma página em Wayland e validar downloads, áudio e arquivos locais antes de entrar no perfil de fábrica. [Receita do Firefox no Alpine](https://github.com/alpinelinux/aports/blob/master/community/firefox/APKBUILD) · [Brave Origin para Linux](https://brave.com/origin/linux/) · [Blanc e sandbox no Linux](https://github.com/bnfy/blanc)

O notebook de referência é o **HP ProBook x360 435 G7 (18Z98LA)**. O sensor Synaptics `06cb:00df` aparece em algumas configurações dessa família e consta na lista do libfprint. É o primeiro ID a conferir na unidade do William. O host de desenvolvimento consultado é um desktop e devolveu `No devices available` com `fprintd-list`. A validação do notebook inclui detecção do leitor, cadastro, desbloqueio, alternativa por senha e retorno da suspensão. [Suporte HP](https://support.hp.com/us-en/product/product-specs/hp-probook-x360-435-g7-notebook-pc/32611820) · [Documentação do modelo no Arch Linux](https://wiki.archlinux.org/title/HP_ProBook_x360_435_G7) · [IDs suportados pelo libfprint](https://fprint.freedesktop.org/supported-devices.html)

O Buildroot fixado pelo projeto já oferece receitas de Linux-PAM e D-Bus. A integração do M2 precisa incluir receitas para libfprint e fprintd, habilitar a ativação do serviço pelo D-Bus de sistema e validar o módulo PAM no login e no desbloqueio.

## Outras escolhas propostas

- **Swap inicial.** Reservar 4 GiB nas máquinas de 16 e 32 GiB e deixar a hibernação desativada por padrão. Reavaliar o tamanho quando houver pressão de memória observável.
- **Criptografia no notebook.** Usar LUKS2 para os dados persistentes, com o arquivo de swap dentro desse volume e um caminho de recuperação testado. Definir separadamente como o volume é desbloqueado durante o boot.
- **Rede e correções.** Deixar SSH e outros servidores de entrada desativados na primeira instalação e restringir o tráfego de entrada ao necessário para a rede e às exceções escolhidas. Consultar atualizações assinadas regularmente e pedir a reinicialização em uma janela adequada. Priorizar correções dos navegadores, que processam conteúdo da internet.
- **Idioma e horário.** Começar com português do Brasil e teclado ABNT2, confirmando o layout antes de cadastrar as senhas. Usar o fuso escolhido na instalação.
- **Modo notebook e tablet.** Suspender ao fechar a tampa. No HP x360, testar rotação automática e desativação do teclado físico ao dobrar para o modo tablet.
- **Navegação.** Usar Firefox como manipulador inicial de links e manter os três navegadores disponíveis no lançador. O usuário pode trocar o padrão depois.
- **Recuperação.** Manter o console de resgate disponível mesmo quando o desktop ou o leitor de digitais estiver indisponível. Testar a recuperação antes de declarar a máquina pronta para uso diário.
