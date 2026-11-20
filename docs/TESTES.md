# Testes e evidências

O Bazzite hospeda o desenvolvimento. Builds limpos produzem imagens que o QEMU inicializa com UEFI/OVMF. Cada cenário começa com disco virtual e variáveis UEFI próprios. Dentro de um cenário de atualização, preserve ambos através de todos os reboots para que o bootloader do Melkor execute a recuperação. Um snapshot restaurado pelo host invalidaria esse resultado.

| Caso | Ação | Resultado observável |
| --- | --- | --- |
| Boot mínimo | Iniciar pelo disco, aguardar login serial e consultar o convidado. | Identidade Melkor, dinit como PID 1 e rootfs somente para leitura. Timeout ou atributo incorreto gera falha. |
| Atualização válida | Partir de A, criar documento persistente, aplicar imagem B assinada e reiniciar. | B inicia, passa a verificação de saúde, confirma o slot e mantém o documento. |
| Imagem inválida | Apresentar imagem adulterada ou de alvo incompatível. | A candidata é rejeitada antes da ativação. O slot inicializável anterior continua selecionável. |
| Interrupção | Desligar a VM durante a gravação de B, antes da ativação. Ligar com o mesmo disco e estado UEFI. | A inicia por conta própria. Documento e perfil ainda são utilizáveis. Uma nova tentativa consegue aplicar a atualização. |
| Falha após ativação | Iniciar B com falha proposital antes da confirmação. Repetir o número de tentativas previsto. | O bootloader volta a A. O executor verifica os identificadores do core e do perfil, faz login e lê o documento. Após M2, repetir com falha exclusiva do login gráfico. |
| Recuperação manual | Selecionar a geração anterior no boot. Em console sem rede, editar com Kibi uma configuração persistente de usuário que quebre o desktop. | A geração escolhida sobe. O arquivo salvo é legível e a sessão volta a iniciar depois da correção. Repetir a edição com a mídia de resgate. |
| Sessão Wayland | Abrir arquivo de teste pelo portal e salvar via Luciole. Abrir Alacritty e Rhun, digitar “ação” e “órgão”, testar clipboard e navegar até definições com rust-analyzer e gopls. | Conteúdo reaberto idêntico, acentos corretos e navegação funcional, com Xwayland ausente. |
| Catálogo | Buscar, compilar, instalar e abrir um app. Repetir com hash inválido, build interrompido ou falta de espaço. Remover o app e coletar temporários. | Sucesso ativa uma geração completa. Falha preserva a anterior. Arquivos protegidos do core conservam seus hashes; gerações retidas continuam utilizáveis. |
| Dependências | Examinar manifestos, scripts, bibliotecas ELF, recursos incorporados e eventos durante boot, sessão ociosa, áudio, autorização e instalação. | Relatório por fluxo diferencia ferramenta do build, biblioteca presente, motor inicializado e script efetivamente executado. Instrumentar motores estáticos quando a lista de processos for insuficiente. |

## Automação

- **Por alteração.** Executar testes unitários dos componentes afetados. Para o instalador, cobrir resolução de dependências, verificação de hashes, ativação de perfis e limpeza. Mudanças na composição do core executam o boot serial.
- **Em imagem candidata.** Executar cenários A/B e testes gráficos. O executor inicial pode controlar QEMU e ler o console serial. Avaliar openQA quando houver interação de login, menus e diálogos que justifique automação visual. [Documentação do openQA](https://open.qa/docs/)
- **Antes do uso diário.** Testar instalação a partir do zero, atualização da versão anterior e documentos representativos no WordCraft e GridCraft. Medir boot, memória ociosa e tempo para abrir terminal e editor em hardware identificado. Manter separadas as medições da VM e do equipamento real.

Cada execução deve registrar commit, hash da imagem, firmware, acelerador, parâmetros relevantes da VM, prazo, resultado e caminho do log serial. Em falha gráfica, capturar a tela e preservar o disco virtual para diagnóstico. Mencionar expressamente testes pendentes quando o ambiente carecer de firmware, rede, dependências de build ou acesso a virtualização.
