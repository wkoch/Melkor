# Testes e evidências

O Bazzite hospeda o desenvolvimento. Builds limpos produzem imagens que o QEMU inicializa com UEFI/OVMF. Cada cenário começa com disco virtual e variáveis UEFI próprios. Dentro de um cenário de atualização, preserve ambos através de todos os reboots para que o bootloader do Melkor execute a recuperação. Um snapshot restaurado pelo host invalidaria esse resultado.

## Cobertura do código próprio

`make test` usa Go 1.24 ou posterior com compilador C, Python 3.11 ou posterior, GNU Make, Bash, Git, OpenSSH e kcov. Go usa o detector de corridas. `coverage.py` é dependência de testes do host, fixada por versão e SHA-256. O kcov 43 foi validado com o RPM assinado do Fedora 45. [coverage.py](https://pypi.org/project/coverage/7.17.3/) · [kcov](https://github.com/SimonKagstrom/kcov)

```sh
python3 -m venv out/test-venv
out/test-venv/bin/python -m pip install --require-hashes --only-binary=:all: -r tests/requirements.txt
make test TEST_PYTHON=out/test-venv/bin/python
```

Use `KCOV=/caminho/do/kcov` se o executável estiver fora do `PATH`. A suíte e a integração contínua exigem ao menos 80% das instruções Go em `apps/`, 80% na métrica de instruções e ramos dos verificadores Python em `scripts/` e 80% das linhas instrumentadas de cada script Shell próprio em `scripts/` e `core/`. Os relatórios ficam em `out/coverage/test-*/`. As fontes de terceiros e as configurações declarativas ficam fora da métrica. O boot UEFI e a leitura da GPT são exercitados pelos testes de integração.

Os casos de maior risco verificam downloads HTTPS e rejeitam redirecionamento para HTTP, fonte com hash divergente e resposta acima do limite de 512 MiB. O preparo do Buildroot confere o commit antes de publicar o bundle ou executar suas receitas. Também exercitamos rejeição de artefato, perfil concorrente, falta de espaço simulada, preservação da geração ativa e imagens com partições ou slots incorretos. Os testes locais usam fontes e perfis temporários. Os testes de boot operam sobre discos virtuais.

Em 20 de novembro de 2026, `make test` passou no Bazzite com 85,9% em Go, 95,0% em Python e 95,7% em Shell. Relatórios em `out/coverage/test-z0s8sadd/`. A compilação real de `melkor instalar nushell` passou com Podman rootless. Uma segunda execução, com `melkor atualizar` e um SHA-256 divergente no catálogo de teste, rejeitou a fonte antes da extração e preservou a geração e o binário ativos. Logs em `out/nushell-cobertura.UGhjQr/`.

| Caso | Ação | Resultado observável |
| --- | --- | --- |
| Boot mínimo | Iniciar pelo disco, aguardar login serial e consultar o convidado. | Identidade Melkor, dinit como PID 1 e rootfs somente para leitura. Timeout ou atributo incorreto gera falha. |
| Mídia de diagnóstico | Iniciar pelo USB UEFI virtual ao lado de um NVMe virtual gravável. | Login comum, framebuffer e raiz em RAM. O NVMe aparece sem ser montado e o hash integral do disco permanece igual. |
| Atualização válida | Partir de A, criar documento persistente, aplicar imagem B assinada e reiniciar. | B inicia, passa a verificação de saúde, confirma o slot e mantém o documento. |
| Imagem inválida | Apresentar imagem adulterada ou de alvo incompatível. | A candidata é rejeitada antes da ativação. O slot inicializável anterior continua selecionável. |
| Interrupção | Desligar a VM durante a gravação de B, antes da ativação. Ligar com o mesmo disco e estado UEFI. | A inicia por conta própria. Documento e perfil ainda são utilizáveis. Uma nova tentativa consegue aplicar a atualização. |
| Falha após ativação | Iniciar B com falha proposital antes da confirmação. Repetir o número de tentativas previsto. | O bootloader volta a A. O executor verifica os identificadores do core e do perfil, faz login e lê o documento. Após M2, repetir com falha exclusiva do login gráfico. |
| Recuperação manual | Selecionar a geração anterior no boot. Em console sem rede, editar com Kibi uma configuração persistente de usuário que quebre o desktop. | A geração escolhida sobe. O arquivo salvo é legível e a sessão volta a iniciar depois da correção. Repetir a edição com a mídia de resgate. |
| Instalação de disco inteiro | Iniciar a mídia em uma VM com dois discos virtuais, escolher um e confirmar seu identificador. | O alvo contém ESP, slots A/B, dados e swap em arquivo. O segundo disco conserva seu hash. |
| Conta comum e digital | Tentar escrever no core e na ESP como usuário comum. No notebook, cadastrar a digital, bloquear a sessão, desbloquear por digital e repetir com o sensor indisponível. | Escrita negada ao usuário comum. Senha alternativa funciona. A administração exige autenticação root separada. |
| Navegadores de fábrica | Abrir Firefox, Brave Origin e Blanc em Wayland, carregar uma página, tocar áudio e testar um download. | Os três iniciam com sandbox habilitado e aceitam uma versão candidata pelo fluxo do perfil. |
| DNS do sistema | Iniciar com DHCP IPv4 e com IPv6 por anúncios de roteador. Consultar um domínio com um aplicativo nativo e repetir nos navegadores e em um contêiner. Bloquear o DoH e observar o tráfego de saída. | Consultas chegam ao Quad9 via HTTPS autenticado. A falha fica visível sem redirecionamento automático para DNS externo em texto claro. |
| Importação DEB e RPM | Instalar um aplicativo local com dependências e scripts em imagem descartável. Repetir com pacote incompatível ou dependência ausente. | O gerenciador da distribuição resolve o pacote dentro do contêiner. O aplicativo inicia com seus próprios dados, o core mantém os hashes e uma rejeição preserva a versão ativa. |
| Shell interativo | Abrir Alacritty com o perfil padrão e executar no Nushell a expressão `[1 2 3] \| math sum`. Em separado, executar um script POSIX com `/bin/sh`. | A expressão retorna 6 e o script termina com o status esperado. O login de recuperação funciona sem o perfil de aplicativos. |
| Sessão Wayland | Abrir arquivo de teste pelo portal e salvar via Luciole. Abrir Alacritty e Rhun, digitar “ação” e “órgão”, testar clipboard e navegar até definições com rust-analyzer e gopls. | Conteúdo reaberto idêntico, acentos corretos e navegação funcional, com Xwayland ausente. |
| Atualização do Nushell | Provocar uma falha na construção ou validação de uma nova versão. Abrir uma nova janela do Alacritty. | A geração anterior continua ativa e o shell interativo inicia com a versão anterior. |
| Catálogo | Buscar, compilar, instalar e abrir um app. Repetir com hash inválido, build interrompido ou falta de espaço. Remover o app e coletar temporários. | Sucesso ativa uma geração completa. Falha preserva a anterior. Arquivos protegidos do core conservam seus hashes; gerações retidas continuam utilizáveis. |
| Dependências | Examinar manifestos, scripts, bibliotecas ELF, recursos incorporados e eventos durante boot, sessão ociosa, áudio, autorização e instalação. | Relatório por fluxo diferencia ferramenta do build, biblioteca presente, motor inicializado e script efetivamente executado. Instrumentar motores estáticos quando a lista de processos for insuficiente. |

## Verificação do boot M0

Execute `make test-boot OUT=out/m0` com a imagem já gerada. O executor usa QEMU com OVMF, KVM quando `/dev/kvm` está acessível e TCG como alternativa. A imagem de disco é aberta somente para leitura. Cada execução guarda `serial.log` e `result.json` em uma nova pasta de `OUT/tests/`.

Para exercitar o caso de falha, use um ambiente com Python 3.11 ou posterior, QEMU e OVMF disponíveis. Crie um disco vazio descartável dentro do checkout e informe um prazo curto em TCG.

```sh
mkdir -p out
cenario=$(mktemp -d "$PWD/out/m0-negativo.XXXXXX")
truncate -s 72M "$cenario/disco-vazio.img"
python3 scripts/test-boot.py "$cenario/disco-vazio.img" \
    --log-dir "$cenario/tests" --accel tcg --timeout 10
```

O processo deve terminar com código `1`. Em `result.json`, verifique a mensagem de timeout ao aguardar o prompt de login. A saída que o firmware enviar ao console fica em `serial.log`.

## Boot da mídia de diagnóstico

Execute `make check-env BOARD=live`, `make image BOARD=live OUT=out/live-hp` e `make test-boot BOARD=live OUT=out/live-hp`. O teste usa QEMU com TCG, OVMF, `sfdisk` e `mke2fs`. O pendrive virtual abre em modo somente leitura. O NVMe virtual gravável contém uma GPT e um volume ext4 válido. A sessão verifica raiz em RAM, framebuffer, detecção dos dispositivos, permissões do usuário comum e escrita no home volátil. O executor calcula o hash do NVMe antes e depois da sessão.

Para iniciar a ISO como CD UEFI, execute `distrobox enter -n melkor-build -- python3 scripts/test-boot.py out/live-hp/build/images/melkor-live.iso --live --log-dir out/live-hp/iso-tests`. Em um host que já tem os pré-requisitos do QEMU e OVMF, execute o mesmo comando a partir de `python3`. Para testar o Ventoy, copie essa mesma ISO para a partição de dados de um pendrive Ventoy virtual, selecione o modo normal no menu e confirme o login pelo console de vídeo da VM.

O teste Live verifica a presença do firmware da AX200 e dos dois arquivos da base regulatória. O executor consulta o domínio mundial com `iw reg get` e examina `dmesg` para detectar falhas de carregamento ou de assinatura da base. Para validar a incorporação no kernel, compare a seção `.rodata` de `drivers/base/firmware_loader/builtin/iwlwifi-cc-a0-77.ucode.gen.o` à fonte verificada e cheque o símbolo `_fw_iwlwifi_cc_a0_77_ucode_bin` em `vmlinux`. A inicialização da interface Wi-Fi é exercitada no HP.

Depois da validação virtual, use a mídia no notebook para conferir o console com tela e teclado locais e obter os IDs dos componentes. [Procedimento para o HP](INSTALACAO.md#teste-de-hardware-pelo-pendrive)

## Protótipo A/B do M1

Execute `make check-m1` com a imagem já gerada, Python, `sfdisk` e `blkid`. A verificação inspeciona o GPT, os quatro sistemas de arquivos, o estado inicial do GRUB, a associação entre os `PARTUUIDs` e os slots do RAUC, os kernels e o UUID usado para montar os dados. Ela lê o arquivo de disco sem criar dispositivos de bloco no host.

Execute `make test-boot BOARD=m1` com QEMU e OVMF. O teste cria uma cópia gravável do disco. Em A, faz login, grava um documento no home e sincroniza o disco. Em seguida, inicia outra VM com o mesmo disco e as mesmas variáveis UEFI, confirma que B está realçado no menu do GRUB antes de iniciar e confere o conteúdo. Os logs ficam em `serial-a.log` e `serial-b.log`, junto de `result.json`.

A imagem de fábrica contém a mesma geração nos dois slots. Esse teste verifica a seleção manual e a persistência.

### Bundles assinados e recuperação M1

Execute os comandos a seguir com o ambiente M1 preparado e o OpenSSL disponível para gerar as credenciais. O teste precisa de Python 3.11 ou posterior, QEMU, OVMF, `mke2fs` e `sfdisk`. A chave e a senha temporárias servem exclusivamente para a imagem de laboratório. Builds de uso normal mantêm o login root bloqueado até o provisionamento pelo instalador.

```sh
make check-env BOARD=m1
umask 077
lab=$(mktemp -d "$PWD/out/rauc-lab.XXXXXX")
openssl req -x509 -newkey rsa:2048 -nodes -days 7 -subj '/CN=Melkor M1 Lab/' \
    -keyout "$lab/key.pem" -out "$lab/keyring.pem"
openssl rand -hex 16 > "$lab/root-password"
openssl passwd -6 -stdin < "$lab/root-password" > "$lab/root-hash"

MELKOR_RAUC_KEYRING="$lab/keyring.pem" MELKOR_LAB_ROOT_HASH_FILE="$lab/root-hash" \
    make image BOARD=m1 OUT=out/m1-rauc
make check-m1 OUT=out/m1-rauc
make test-boot BOARD=m1 OUT=out/m1-rauc
make bundle-m1 OUT=out/m1-rauc CERT="$lab/keyring.pem" KEY="$lab/key.pem" \
    KEYRING="$lab/keyring.pem" VERSION=1.0-test
make test-update OUT=out/m1-rauc CERT="$lab/keyring.pem" KEY="$lab/key.pem" \
    ROOT_PASSWORD_FILE="$lab/root-password"
```

O executor usa cópias graváveis do disco e das variáveis UEFI, sem interface de rede. Entrega os bundles por um segundo disco virtual somente para leitura. Verifica a recusa de um usuário comum, a exclusão mútua do atualizador, a assinatura inválida, o conteúdo corrompido e o identificador incompatível. Nos casos de rejeição, compara o hash do slot inativo e o estado do GRUB antes e depois.

O cenário válido instala em B, mantém o hash de A, confirma B no primeiro boot e verifica uma segunda inicialização em B. Depois seleciona A manualmente. Outro cenário instala uma candidata assinada cujo serviço de confirmação falha e verifica o retorno automático a A no próximo boot. O cenário de interrupção encerra o QEMU assim que identifica uma escrita parcial no slot B e verifica o boot em A, o hash desse slot e o documento persistente. Em seguida, o executor reaplica o bundle e confirma B. Os logs e o relatório JSON ficam em `OUT/update-tests/`.

## Perfil do Nushell

Execute `go test -race ./...` em `apps/` com Go 1.24 ou posterior e o compilador C exigido pelo detector de corridas. O teste verifica a troca de geração, a preservação da anterior após uma falha e a ausência de reconstrução quando a receita segue idêntica.

Com `make tools` concluído, use um perfil temporário para testar a receita real. A instalação requer Podman rootless e rede para preparar as fontes.

```sh
cenario=$(mktemp -d "$PWD/out/nushell-teste.XXXXXX")
export MELKOR_PERFIL="$cenario/perfil"
out/tools/melkor shell -c 'test "$((2 + 4))" -eq 6'
out/tools/melkor instalar nushell
out/tools/melkor shell --no-config-file -c '[1 2 3] | math sum'
```

Para testar a falha de atualização, copie a receita para um catálogo temporário, altere `version`, retire a etapa `fetch` e troque o comando de `build` por `["false"]`. Execute `melkor atualizar` com `MELKOR_CATALOGO` apontando para essa cópia. Confira o código de erro, o destino de `atual`, a saída de `melkor shell --version` e o resultado `6` da expressão. Repita a abertura do shell pelo Alacritty quando a sessão gráfica estiver disponível.

## Publicação e consumo de receitas

A suíte cria repositórios Git temporários e chaves SSH descartáveis. Ela cobre o teste e aceite de uma proposta, a cópia dos defaults para a geração seguinte, o push de uma branch contendo só a receita, a retomada do mesmo PR e a instalação a partir de uma tag assinada. Tags sem assinatura, com chave desconhecida, reapontadas para outra versão ou com receitas inválidas preservam o catálogo sincronizado anterior. O GitHub CLI é simulado nos testes e o Git faz o push para um repositório bare local.

Em um catálogo publicado, a CI valida todos os JSONs e roda os testes de build e execução das receitas alteradas em Podman rootless. A prova local de ponta a ponta de 20 de novembro de 2026 usou HTTPS local, um aplicativo C compilado para musl, uma proposta aceita e um segundo perfil vazio que instalou o mesmo aplicativo e seus defaults a partir de uma tag assinada. O manifesto da segunda instalação está em `out/catalogo-real.YY5Efp/perfil-novo/atual/manifest.json`.

O primeiro PR real, aberto e localizado novamente com `melkor receita publicar nushell`, alterou apenas `recipes/nushell.json`. A CI baixou e verificou o validador privado por SHA-256, compilou o CLI e passou pela compilação e pelos testes da receita Nushell 0.117.0 em Podman rootless. [PR #1](https://github.com/wkoch/Melkor-Recipes/pull/1) · [Execução no GitHub](https://github.com/wkoch/Melkor-Recipes/actions/runs/44789861496)

## Quad9 no resolvedor do sistema

Prepare a imagem com `make env BOARD=m1` e `make image BOARD=m1 OUT=out/m1-doh`. Execute `make check-m1 OUT=out/m1-doh` para verificar os DNS stamps e o resolvedor do sistema nos dois slots.

Execute `make test-dns OUT=out/m1-doh` com Python, QEMU, OVMF, `tcpdump` e acesso direto ao TCP 443 do Quad9. O executor usa rede IPv4, o login comum de laboratório e duas cópias da imagem. Confirma a rota da VM, os sockets UDP e TCP em loopback e a identidade e capacidades do processo. Na primeira VM, o TXT `proto.on.quad9.net` precisa responder `doh` pelo resolvedor padrão e pelos dois endereços de loopback. Na segunda, o monitor QMP ativa o filtro depois de uma consulta bem-sucedida. A próxima consulta deve falhar com a rota já configurada. A captura depois de ativar o filtro deve mostrar a tentativa HTTPS ao Quad9. Nenhuma captura deve conter tráfego DNS para as portas 53, 853 e 9953. [Consulta de diagnóstico do Quad9](https://docs.quad9.net/FAQs/#protocol-test-confirm-on-which-protocol-quad9-received-your-query) · [Filtros do QEMU](https://www.qemu.org/docs/master/system/invocation.html)

Para verificar o cenário sem IPv4, use `make test-dns OUT=out/m1-doh DNS_IPV6_ONLY=1` em um host com acesso IPv6 funcional ao Quad9. Os logs seriais, as capturas e o relatório ficam em `OUT/dns-tests/`.

## Provas de importação isolada

Execute `make test-packages OUT=out/importacao RPM="$rpm" RPM_SHA256="$sha256"` com Go 1.24+, Python 3.11+, Podman rootless, acesso aos repositórios oficiais e o RPM `tree` x86-64 assinado pelo Fedora. O Make compila o CLI e usa um perfil temporário via `MELKOR_PERFIL`. O teste validado usou `tree-2.3.1-2.fc45.x86_64.rpm`, com SHA-256 `eae4afd17fd2ead221eb201aa265983c3e7f418c45c978c18813c1be63b9ddaa`.

O executor gera um DEB de teste com uma dependência real e um `postinst` que exige apenas loopback. Testa a execução como usuário comum em uma raiz somente leitura, grava e reabre dados do aplicativo e verifica a preservação da geração ativa quando o checksum, a resolução de dependências ou a verificação do RPM falha. Também remove o DEB do perfil e verifica que seus dados persistem e que o RPM continua executável. Os logs e o relatório ficam em `OUT/package-tests/`.

Para garantir isolamento também nos testes manuais, mantenha o armazenamento rootless normal do Podman e mude apenas `MELKOR_PERFIL`. A importação foi exercitada no host Bazzite. A inclusão de Podman e do CLI na imagem instalada e o lançamento gráfico ainda dependem dos respectivos marcos.

## Retorno de gerações do perfil

A suíte Go cobre o retorno de um perfil de testes para uma geração com binário nativo e configuração. Também rejeita imagens OCI ausentes, executáveis ou configurações substituídos por links, manifestos inválidos e identificadores que escapam da pasta de gerações. Cada falha deve preservar o link `atual`.

`make test-packages` lista o perfil após remover o DEB, volta à geração anterior e reabre o dado gravado no aplicativo. O mesmo teste reativa a geração posterior e confirma que o DEB voltou a ficar ausente. Use os mesmos pré-requisitos do teste de importação.

Com uma cópia de um perfil que já contém Nushell, use `melkor geracoes` e guarde o identificador da geração ativa. Execute `melkor remover nushell` e confira que `melkor shell -c 'printf "%s\n" "$0"'` abre `/bin/sh`. Use `melkor voltar <geração>` e execute `melkor shell --no-config-file -c '[1 2 3] | math sum'`. O shell reativado deve retornar `6`.

## Automação

- **Por alteração.** Executar testes unitários dos componentes afetados. Para o instalador, cobrir resolução de dependências, verificação de hashes, ativação de perfis e limpeza. Mudanças na composição do core executam o boot serial.
- **Em imagem candidata.** Executar cenários A/B e testes gráficos. O executor inicial pode controlar QEMU e ler o console serial. Avaliar openQA quando houver interação de login, menus e diálogos que justifique automação visual. [Documentação do openQA](https://open.qa/docs/)
- **Antes do uso diário.** Testar instalação a partir do zero, atualização da versão anterior e documentos representativos no WordCraft e GridCraft. Medir boot, memória ociosa e tempo para abrir terminal e editor em hardware identificado. Manter separadas as medições da VM e do equipamento real.

Cada execução deve registrar commit, hash da imagem, firmware, acelerador, parâmetros relevantes da VM, prazo, resultado e caminho do log serial. Em falha gráfica, capturar a tela e preservar o disco virtual para diagnóstico. Mencionar expressamente testes pendentes quando o ambiente carecer de firmware, rede, dependências de build ou acesso a virtualização.
