# Marcos de implementação

Comece por M0. Os critérios abaixo descrevem entregas futuras. Registre commit, ambiente, comandos e evidências ao concluir cada marco.

## M0 — Do checkout ao console

**Entrega:** receita mínima de x86-64, compilada a partir das fontes, com kernel, musl, dinit como PID 1, rootfs somente para leitura e acesso por console serial. O identificador do sistema em `/etc/os-release` deve ser `melkor`. Inclua os drivers de armazenamento e console usados pela VM.

1. Fixar release e hash do Buildroot e a configuração do alvo. Manter a árvore externa, patches e configurações no checkout. Identificar ferramentas do host e separar o diretório de saída das fontes.
2. Criar e documentar duas entradas reais: construção da imagem e teste de boot. Escolher uma toolchain de bootstrap compatível com o alvo e registrar compilador, libc e linker efetivamente usados. Se a primeira imagem usar GCC, manter a transição para LLVM/libc++ explícita.
3. Gerar um disco inicializável e iniciá-lo via firmware UEFI/OVMF em QEMU. O teste deve exercer a entrada de boot do disco. Selecionar KVM quando disponível e permitir emulação por software com um prazo de teste apropriado.
4. Usar o console para verificar a identidade da imagem, identificar dinit em `/proc/1/comm` e confirmar as opções da montagem do rootfs. Guardar log serial e código de saída do teste no host.

**Aceite:** um checkout limpo com os pré-requisitos documentados gera a imagem. O executor encontra o login e verifica os três atributos acima dentro de um prazo limitado. Uma imagem incapaz de chegar ao console faz o teste falhar. Neste marco, o primeiro boot pode partir de uma imagem de disco pré-montada.

## M1 — Atualização A/B recuperável

Escolher integração do bootloader, confirmação de saúde e assinatura das imagens. Definir a persistência de dados, configuração local e variáveis UEFI. Demonstrar atualização A → B, fallback automático após falha, seleção manual de A e interrupção da gravação com o mesmo disco. Incluir Kibi no core e preparar mídia de recuperação. [Casos de recuperação](TESTES.md)

## M2 — Desktop Wayland utilizável

Integrar a toolchain pretendida, utilitários selecionados, sessão dinit, Mesa, áudio e portais. Fixar versões compatíveis de Umbriel e wlroots. Validar Noctalia, Luciole e Alacritty em musl. Testar ABNT2, acentos, clipboard, bloqueio e recuperação de sessão. Estender o teste de saúde da candidata para detectar uma falha que impeça o login gráfico. Produzir o inventário dos runtimes observados.

## M3 — Perfil nativo e estação de trabalho

Implementar catálogo selecionado, receitas fixadas, build isolado e ativação atômica por perfil. Executar o primeiro ciclo completo com um aplicativo pequeno. Adicionar os aplicativos da tabela de arquitetura, priorizando Rhun, Git e servidores de linguagem. Cobrir falha de build, falta de espaço, instalações concorrentes e retorno a um perfil anterior. A ajuda de IA na criação de receitas entra depois que uma receita conhecida puder percorrer o fluxo sem IA.

## M4 — Instalar e usar no computador

Gerar instalador e mídia de resgate. Oferecer imagem previamente construída e modo de compilação local que parta de uma toolchain inicial identificada. Testar instalação em disco virtual vazio e atualização a partir de uma versão anterior com documentos e aplicativos. Validar em hardware próprio por mídia ou SSD de teste: GPU, armazenamento, rede, áudio, suspensão, monitores e latência de abertura dos aplicativos. Documentar a recuperação antes de promover o Melkor a sistema de uso diário.
