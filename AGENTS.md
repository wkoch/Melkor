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
- Priorize os riscos vindos da rede. Cubra com testes a validação de fontes e assinaturas, a rejeição de artefatos e a preservação da geração ativa quando uma atualização falha.

## Compatibilidade das releases

Regra definitiva do projeto: versões finais (`1.0`, `1.1`, `1.1.1` e assim por diante) não podem quebrar versões anteriores que o usuário esteja usando. Uma quebra nessas versões só entra com a concordância explícita do mantenedor, e nesse caso a versão indica a incompatibilidade pelo SemVer, com o incremento de versão maior. Versões alpha e beta (`0.1.0-alpha.N`, `-beta.N`) ficam fora dessa regra e podem quebrar instalações de teste. Antes de publicar uma versão final, confirme que uma instalação da versão anterior continua recebendo a atualização pelo canal sem intervenção manual. Se a migração exigir reinstalação, pare e pergunte ao mantenedor antes de publicar. [Arquitetura](docs/ARQUITETURA.md#compatibilidade-das-releases)

## Para concluir

Execute os casos pertinentes de [TESTES.md](docs/TESTES.md). Informe o comando, o resultado observado e as limitações reais do ambiente. Um build concluído, um boot até o login e uma recuperação automática são resultados distintos. Atualize o README quando houver um novo comando operacional e o documento de marcos quando seus critérios tiverem sido demonstrados.

Execute `make test` nas mudanças de código próprio e mantenha a cobertura mínima de 80% conforme as métricas de [TESTES.md](docs/TESTES.md#cobertura-do-código-próprio).

<!-- trail:start -->
## Trail — repo context graph

This repo is indexed in `.trail/cache/`: small linked markdown nodes that explain each
system and carry exact file:line spans, kept in sync with the code through git.

For ANY task here — understanding how something works, finding where code lives,
or scoping a change — get context from the graph before grepping or opening
source files. Re-ask freely (it's cheap) and reuse literal identifiers you
already have (symbol, error string, file name) as the query. New to this repo?
Run `trail map` first — a token-budgeted orientation (dir clusters, hubs,
hotspots), no LLM, no key.

- Run `trail ask "<your question>" --source` → ranked nodes with the relevant
  code spans inlined (each hit's ≤8-line crux by default; `--full` for whole
  definitions when the crux isn't enough). Match the tool to the task shape:
  for understanding or editing, the top node IS the answer — cite its
  `covers:` file:line spans and edit straight from `--source`. For
  exhaustive tasks ("every occurrence / every caller of this pattern"), ranked
  results are top-N, not complete — run `trail grep "<literal>"` instead
  (exhaustive over indexed files, grouped by enclosing symbol), falling back
  to raw `grep -rn` only for unindexed files.
- `trail skeleton <file>` → every definition's signature + span, ~10× cheaper
  than reading the file; use it to skim an API surface.
- `trail callers <symbol>` gives precomputed, exact edges — who calls this.
  Add `--direction out` for what it calls, or `--depth N` to walk
  transitively for the full blast radius. For structural questions, skip
  ranking and use this directly.
- Or browse: `.trail/cache/INDEX.md` lists every node; follow the links.
- Monorepos and folders of multiple repos rank fairly across sub-projects —
  hits carry `[scope/]` labels naming which one they're from. Narrow with
  `trail ask "<task>" --in <scope>/` once you know where you're working.

If a returned span is truncated ("+N more lines"), open the file at that exact
range before finalizing. Only open source files when a node genuinely lacks a
needed detail, and then at the exact file:line the node points to — never
re-read whole files.

After big code changes, refresh the graph with `trail build` (deterministic,
no API key, $0).
<!-- trail:end -->
