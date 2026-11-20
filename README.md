# Melkor Linux

Melkor é a distribuição Linux pessoal de William Koch. Sua estação de trabalho deve poder ser reconstruída a partir de um commit do Git. A receita determina o core, os serviços e o desktop. Os aplicativos nativos têm um catálogo próprio e podem ser instalados pelo fonte sem alterar o core em execução.

**Estágio atual deste material:** especificação inicial para desenvolvimento. O primeiro resultado a construir é uma imagem x86-64 que inicia por UEFI e chega ao console em QEMU. Os marcos seguintes cobrem recuperação A/B, desktop Wayland e aplicativos.

## A experiência pretendida

- **Alterar o core.** Editar a receita, recompilar o conjunto completo a partir das fontes e testar a imagem. Ativar a imagem como uma nova geração, mantendo a anterior inicializável.
- **Instalar um aplicativo.** Pesquisar no catálogo, escolher, compilar em ambiente descartável e ativar o resultado no perfil de usuário. Flatpak é dispensável para esse fluxo.
- **Trabalhar.** Iniciar Umbriel e Noctalia, usar Luciole para abrir e salvar documentos, Alacritty como terminal e Rhun para código, Git e trabalho com agentes. Kibi permanece acessível pelo console de texto.
- **Recuperar a máquina.** Selecionar a geração anterior pelo bootloader ou iniciar pela mídia de recuperação. Documentos pessoais persistem entre os boots.

## Começar com o Codex

Abra no Codex a pasta de trabalho do repositório. Estes arquivos devem estar nela, com `AGENTS.md` e este README na raiz. Quem ainda precisa de um checkout pode obtê-lo com `git clone https://github.com/wkoch/Melkor.git`. Mantenha o Bazzite como host de desenvolvimento e destine discos virtuais descartáveis aos testes.

**Primeira tarefa para colar no Codex**

> Leia AGENTS.md e os critérios de M0 em docs/MARCOS.md. Inspecione o checkout e as ferramentas disponíveis. Implemente a menor receita versionada que gere uma imagem Melkor x86-64 com musl, dinit e console serial. Adicione uma entrada de build e um teste automatizado de boot UEFI em QEMU. Execute o que o ambiente permitir, registre os comandos reais no README e identifique qualquer critério pendente. Trabalhe somente em M0.

| Documento | Para que serve |
| --- | --- |
| [AGENTS.md](AGENTS.md) | Instruções para as sessões do Codex neste repositório. |
| [Arquitetura](docs/ARQUITETURA.md) | Contrato da receita, limites do core, política de dependências e catálogo inicial. |
| [Marcos](docs/MARCOS.md) | Sequência de implementação, entregas e decisões pendentes. |
| [Testes](docs/TESTES.md) | Casos de aceitação em VM, recuperação e validação no hardware real. |

Repositório do projeto: [wkoch/Melkor](https://github.com/wkoch/Melkor).
