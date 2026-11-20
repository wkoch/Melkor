# Instruções de trabalho do Melkor

Estas instruções abrangem o repositório inteiro. Use português do Brasil para documentação do projeto. Preserve nomes de comandos, APIs e opções no idioma original.

## Ao iniciar uma tarefa

1. Leia o [README](README.md), confira o estado do checkout com `git status --short --branch` e identifique qual marco a tarefa atende. Preserve alterações existentes do mantenedor.
2. Consulte [arquitetura](docs/ARQUITETURA.md) antes de mudar o conteúdo de uma imagem ou o modelo de atualização. Os [marcos](docs/MARCOS.md) identificam propostas ainda em avaliação.
3. Procure as entradas de build e teste já presentes no checkout. A primeira tarefa cria essas entradas. Documente cada comando depois de executá-lo e indique quais pré-requisitos ele usa.

## Ao implementar

- Mantenha no Git as receitas, versões, hashes de fontes, patches, configurações e testes necessários para reconstruir a entrega. Registre a procedência de uma nova dependência e se ela entra no host de build, core ou aplicativo.
- Para ferramentas próprias, prefira Rust, Go ou C conforme a integração. O desktop também inclui C++. Ferramentas auxiliares de build podem executar em ambiente descartável. Audite os interpretadores de boot e sessão, inclusive os embarcados em binários nativos.
- Toda troca de pacote do core passa pela receita, pelo build completo e pelos testes da geração candidata. Instalação e remoção de aplicativos atuam sobre um perfil versionado. Escreva temporários fora das imagens ativas.
- Use caminhos locais ao checkout ou a diretórios temporários para gerar imagens. Testes de particionamento, interrupção e rollback usam discos virtuais identificados pelo executor. O disco físico do host fica fora desses testes.
- Fixe a versão dos componentes no primeiro build validado. Registre desvios temporários de bootstrap e o trabalho necessário para chegar à configuração pretendida.

## Para concluir

Execute os casos pertinentes de [TESTES.md](docs/TESTES.md). Informe o comando, o resultado observado e as limitações reais do ambiente. Um build concluído, um boot até o login e uma recuperação automática são resultados distintos. Atualize o README quando houver um novo comando operacional e o documento de marcos quando seus critérios tiverem sido demonstrados.
