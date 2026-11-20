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
2. **Boot.** O bootloader mantém a seleção manual da geração anterior e um contador de tentativas para a candidata. A confirmação depende da saúde dos serviços essenciais e, quando houver desktop, de a máquina chegar a uma sessão utilizável. RAUC é o candidato para integrar instalação e confirmação. A escolha do bootloader e de seu armazenamento de estado pertence a M1. [Integração do RAUC](https://rauc.readthedocs.io/en/latest/integration.html)
3. **Aplicativos.** Uma receita conhecida baixa fontes verificadas, resolve as dependências, compila sem privilégios em área isolada e testa o resultado. O instalador prepara os arquivos nativos e troca o perfil ativo em uma operação atômica. Em caso de falha, conserva o perfil anterior. A limpeza preserva artefatos referenciados pelas gerações retidas.
4. **Receitas novas.** A IA pode pesquisar e propor uma receita para revisão e teste. Depois de aceita, a receita entra no catálogo e pode ser executada sem o modelo. Drivers, módulos de kernel e serviços de login entram pela receita do core.
5. **Dados.** Cada geração referencia um perfil compatível. Documentos e preferências do usuário persistem. Configurações essenciais ao boot precisam de defaults por geração e de um tratamento explícito para alterações locais. Migrações de formato em dados persistentes requerem compatibilidade de leitura ou cópia recuperável antes da migração.

## Política de dependências

| Camada | Política |
| --- | --- |
| Build e testes | Python e outras ferramentas exigidas por projetos originais podem rodar no ambiente descartável. Registre essas dependências. A imagem final leva apenas componentes necessários ao uso. |
| Core e sessão padrão | Usar musl como libc. Priorizar binários nativos e buscar zero execução obrigatória de runtimes de linguagens dinâmicas no boot, sessão ociosa e administração normal. Auditar scripts de inicialização, plugins e VMs embarcadas. Registrar cada dependência restante, o fluxo que a aciona e o teste para sua remoção. |
| Gráficos | Usar Wayland, sem servidor Xorg, com Xwayland desativado. O build atual do Umbriel exige componentes XCB e suporte a Xwayland em wlroots. Registrar essas bibliotecas no manifesto e validar a sessão com o servidor Xwayland ausente. [Empacotamento do Umbriel](https://github.com/noctalia-dev/umbriel/blob/main/PACKAGING.md) |
| Aplicativos | Um aplicativo pode ter dependências privadas declaradas. O motor JavaScript do navegador é um exemplo. Identificar plugins ou ferramentas que façam downloads na primeira execução. |

Noctalia usa Luau para extensões. WirePlumber pode usar Lua no roteamento de áudio e polkit avalia regras em JavaScript. A auditoria deve verificar o que efetivamente roda no perfil selecionado. O teste de um recurso continua obrigatório quando se propõe remover seu interpretador. [Plugins do Noctalia](https://docs.noctalia.dev/) · [WirePlumber](https://pipewire.pages.freedesktop.org/wireplumber/scripting/custom_scripts.html) · [polkit](https://polkit.pages.freedesktop.org/polkit/polkit.8.html)

## Seleção inicial

| Destino | Componentes | Uso que valida a seleção |
| --- | --- | --- |
| Core de desktop | Umbriel, Noctalia, Luciole e Alacritty com backend Wayland | Login, lançador, notificações, abrir e salvar pelo portal, terminal com teclado ABNT2. |
| Core e mídia de resgate | Kibi como editor de terminal | Editar e reabrir uma configuração em UTF-8, no console, sem rede ou desktop. |
| Perfil padrão | Git, Firefox, Rhun, WordCraft e GridCraft | Revisar diffs, navegar na web, editar código e trabalhar com arquivos DOCX e XLSX reais. |
| Ferramentas de desenvolvimento | rust-analyzer e gopls | Definição e diagnóstico no Rhun para projetos Rust e Go. |
| Catálogo opcional | Fresh compilado sem plugins, aplicativos próprios e clientes de IA selecionados | Ativar e remover pelo perfil, mantendo os hashes do core. |

O desktop completo em musl e a integração do portal do Luciole são hipóteses a testar. Para o Rhun, valide o executável e as dependências das ferramentas externas que ele aciona. [Noctalia](https://noctalia.dev/) · [Rhun](https://github.com/vshvedov/rhun) · [Kibi](https://github.com/ilai-deutel/kibi) · [Fresh](https://github.com/sinelaw/fresh)
