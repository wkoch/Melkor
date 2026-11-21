# Padrões da primeira instalação

Este documento define o perfil de instalação para os PCs pessoais do William. O instalador de disco real pertence ao M4. A integração do leitor de digitais e dos navegadores depende da sessão Wayland do M2 e do perfil de aplicativos do M3.

## Teste de hardware pelo pendrive

Use a ISO Melkor Live em um pendrive com Ventoy. O sistema carrega o kernel e a raiz em RAM e perde os arquivos feitos durante a sessão ao desligar. Ele enumera os dispositivos sem montar os discos. Esta etapa identifica quais drivers o notebook precisa para a futura instalação. [Melkor Live v0.1.0-alpha.3](https://github.com/wkoch/Melkor/releases/tag/v0.1.0-alpha.3)

1. Baixe `melkor-live-v0.1.0-alpha.3-x86_64.iso` e `SHA256SUMS`. Confira a ISO com `sha256sum --ignore-missing -c SHA256SUMS` no diretório do download.
2. Copie o arquivo `.iso` para a partição de dados do pendrive Ventoy, como um arquivo comum. [Como copiar imagens para o Ventoy](https://www.ventoy.net/en/doc_start.html)
3. No HP ProBook x360 435 G7, ligue o notebook, pressione `Esc` e depois `F9` e escolha a entrada UEFI do pendrive. O GRUB desta mídia é sem assinatura e requer Secure Boot desativado. Se usar Windows com BitLocker, tenha acesso à chave de recuperação antes de alterar a configuração do firmware. Preserve o modo UEFI e restaure a configuração anterior depois do teste. [Secure Boot no HP](https://support.hp.com/in-en/document/ish_6930187-6931079-16) · [Recuperação do BitLocker](https://learn.microsoft.com/windows/security/operating-system-security/data-protection/bitlocker/recovery-overview)
4. No Ventoy, selecione a ISO e escolha **Boot in normal mode**, se aparecer o menu de modos. Se o modo normal não iniciar nessa unidade, o menu oferece **Boot in grub2 mode**. Selecione **Melkor Linux - diagnostico em RAM**. No console local, entre com `melkor` como usuário e senha. `id` deve indicar UID 1000 e `cat /proc/mounts` mostra a raiz em memória.
5. Execute `ls /sys/block`, `lspci`, `lsusb` e `/usr/sbin/iw dev`. Confira se aparecem o SSD, a GPU, a placa de rede e o leitor de digitais. Registre o ID USB do leitor no formato `vvvv:pppp`, que determina o suporte pelo libfprint.
6. Fotografe ou anote as saídas relevantes e encerre a sessão. Desligue pelo botão de energia, mantendo-o pressionado se necessário, e remova o pendrive para voltar ao sistema habitual.

Nesta mídia, a verificação de vídeo usa o framebuffer do firmware e o Wi-Fi se limita à detecção da interface. A autenticação por digital, a conexão Wi-Fi, os testes de suspensão e a sessão gráfica fazem parte das próximas validações no notebook.

Na unidade de referência, a placa Wi-Fi é uma Intel AX200 (`8086:2723`). Quando `/usr/sbin/iw dev` sair vazio, fotografe a saída abaixo. O log indica se o driver encontrou o firmware e se registrou a interface.

```sh
dmesg | grep -Ei 'iwlwifi|iwlmvm|01:00[.]0|rfkill|regulatory'
```

## Instalação da base no disco inteiro

Inicie a ISO mínima pelo Ventoy e escolha **Instalar Melkor**. A lista mostra um número, modelo, capacidade e setor lógico para cada disco interno elegível. Digite o número do disco e confira o modelo e a capacidade repetidos na confirmação. Digite `APAGAR` somente se você tiver cópia dos dados e quiser apagar todas as partições daquele disco. Defina senhas distintas para `root` e `melkor`.

Ao concluir, reinicie sem a mídia e entre como `melkor`. A base abre no console e permite configurar Wi-Fi com `iwctl`.

### Instalar o desktop depois

Copie o arquivo local assinado `melkor-desktop.raucb` para um pendrive FAT32 com rótulo `MELKOR_APPS`. Após iniciar pelo SSD, conecte o pendrive e execute `su -` com a senha de root. Em seguida, rode `melkor-finalizar`. Se a partição de dados do Ventoy já estiver em FAT32 e tiver rótulo `Ventoy`, use a mesma mídia com `melkor-finalizar Ventoy`. O comando aceita o rótulo da mídia como argumento.

Aguarde o RAUC concluir com sucesso e reinicie. O RAUC grava o desktop no slot inativo e preserva o volume com os dados, as senhas e a configuração do Wi-Fi. No próximo boot, o login gráfico abre Umbriel e Noctalia. O bundle inclui Alacritty, Nushell, Firefox ESR, Brave Origin e Blanc.

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

O perfil inicial dos navegadores está disponível no primeiro login gráfico após a aplicação do bundle. A compilação local segue sendo uma opção para quem quiser alterar as receitas.

### DNS do sistema

O core M1 usa o `dnsproxy` para servir DNS por UDP e TCP em `127.0.0.1` e `::1`. Ele se conecta aos quatro endereços publicados do Quad9 seguro e valida o certificado para `dns.quad9.net`. As consultas ficam indisponíveis quando nenhum endereço DoH responde. O `dhcpcd` configura endereços e rotas, e o `/etc/resolv.conf` somente leitura mantém o resolvedor local para os aplicativos que usam a configuração do sistema. A configuração de Wi-Fi e de exceções explícitas por conexão para portais cativos ou DNS interno depende do gerenciador de rede da instalação. [Endereços e modalidades do Quad9](https://docs.quad9.net/services/)

A validação da instalação inclui reconexão, comportamento com o relógio incorreto e consultas pelos navegadores de fábrica e pelos contêineres de aplicativos. Aplicativos com resolvedor próprio precisam de configuração que preserve a política da instalação. [Casos de teste do DNS](TESTES.md#quad9-no-resolvedor-do-sistema)

### Compatibilidade a demonstrar

O catálogo atual foi validado com executáveis estáticos x86-64. Os navegadores exigem empacotamento de binários e dependências privadas ou builds compatíveis com a base musl. A receita de Firefox do Alpine dá um ponto de partida para validar as dependências em musl. O Brave Origin distribui pacotes Linux para as famílias Debian e RPM, e o Blanc distribui AppImages com Electron. Cada receita deve preservar o sandbox suportado pelo navegador, abrir uma página em Wayland e validar downloads, áudio e arquivos locais antes de entrar no perfil de fábrica. [Receita do Firefox no Alpine](https://github.com/alpinelinux/aports/blob/master/community/firefox/APKBUILD) · [Brave Origin para Linux](https://brave.com/origin/linux/) · [Blanc e sandbox no Linux](https://github.com/bnfy/blanc)

O notebook de referência é o **HP ProBook x360 435 G7 (18Z98LA)**. O `lsusb` na unidade do William confirmou o leitor Synaptics `06cb:00df`, listado pelo libfprint. A validação biométrica inclui cadastro, desbloqueio, alternativa por senha e retorno da suspensão. [Suporte HP](https://support.hp.com/us-en/product/product-specs/hp-probook-x360-435-g7-notebook-pc/32611820) · [IDs suportados pelo libfprint](https://fprint.freedesktop.org/supported-devices.html)

O Buildroot fixado pelo projeto já oferece receitas de Linux-PAM e D-Bus. A integração do M2 precisa incluir receitas para libfprint e fprintd, habilitar a ativação do serviço pelo D-Bus de sistema e validar o módulo PAM no login e no desbloqueio.

## Outras escolhas propostas

- **Swap inicial.** Reservar 4 GiB nas máquinas de 16 e 32 GiB e deixar a hibernação desativada por padrão. Reavaliar o tamanho quando houver pressão de memória observável.
- **Criptografia no notebook.** Usar LUKS2 para os dados persistentes, com o arquivo de swap dentro desse volume e um caminho de recuperação testado. Definir separadamente como o volume é desbloqueado durante o boot.
- **Rede e correções.** Deixar SSH e outros servidores de entrada desativados na primeira instalação e restringir o tráfego de entrada ao necessário para a rede e às exceções escolhidas. Consultar atualizações assinadas regularmente e pedir a reinicialização em uma janela adequada. Priorizar correções dos navegadores, que processam conteúdo da internet.
- **Idioma e horário.** Começar com português do Brasil e teclado ABNT2, confirmando o layout antes de cadastrar as senhas. Usar o fuso escolhido na instalação.
- **Modo notebook e tablet.** Suspender ao fechar a tampa. No HP x360, testar rotação automática e desativação do teclado físico ao dobrar para o modo tablet.
- **Navegação.** Usar Firefox como manipulador inicial de links e manter os três navegadores disponíveis no lançador. O usuário pode trocar o padrão depois.
- **Recuperação.** Manter o console de resgate disponível mesmo quando o desktop ou o leitor de digitais estiver indisponível. Testar a recuperação antes de declarar a máquina pronta para uso diário.
