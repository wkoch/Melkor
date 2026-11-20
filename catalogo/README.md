# Repositório de receitas do Melkor

O repositório `Melkor-Recipes` usa `main` para as receitas revisadas. Cada aplicativo tem um JSON em `recipes/<nome>.json`. O modelo [validar-receitas.yml](validar-receitas.yml) vai em `.github/workflows/` do repositório de receitas. A CI usa um arquivo de fontes fixado por SHA-256 em uma pré-release privada do próprio catálogo. Com o token de leitura do job ela baixa somente esse arquivo, compila o CLI e testa as receitas alteradas no PR.

O primeiro catálogo pode ser inicializado com [a receita validada do Nushell](../apps/recipes/nushell.json).

A receita descreve a origem oficial, a fonte HTTPS e seu SHA-256, a versão, a toolchain, a imagem OCI por digest, os comandos de build e teste e os executáveis a instalar. O campo opcional `defaults` associa caminhos relativos a conteúdos de configuração reutilizáveis. O launcher disponibiliza esses arquivos em `XDG_CONFIG_DIRS` para aplicativos que leem essa variável. Preferências pessoais ficam em `XDG_CONFIG_HOME`, que tem prioridade para esses aplicativos. Envie ao GitHub só os defaults do aplicativo.

### Atualizar o validador da CI

Com `make test` e `make tools` concluídos no Melkor, empacote a árvore revisada de `apps/`. Publique `melkor-cli.tar.gz` em uma nova pré-release privada, com tag da forma `ci-validador-v0.1.0`. Registre no corpo da pré-release o commit ou a procedência da árvore de fontes. A tag e o hash do arquivo devem ser configurados nas variáveis `MELKOR_VALIDADOR_TAG` e `MELKOR_VALIDADOR_SHA256` do catálogo. [Permissões de leitura dos workflows](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax#permissions)

```sh
mkdir -p out/catalogo-validador
tar --sort=name --mtime=@0 --owner=0 --group=0 --numeric-owner \
    -czf out/catalogo-validador/melkor-cli.tar.gz apps
sha256sum out/catalogo-validador/melkor-cli.tar.gz
```

## Publicar uma versão assinada

Depois de aprovar o PR e integrar o commit em `main`, assine uma tag com a chave de publicação SSH e envie a tag ao GitHub. Use uma chave dedicada e distribua o arquivo local de assinantes autorizados por um canal em que você confia. O arquivo tem um principal seguido da chave pública. [Tags assinadas no Git](https://git-scm.com/docs/git-tag) · [Arquivo de assinantes autorizados](https://git-scm.com/docs/git-config#Documentation/git-config.txt-gpgsshallowedSignersFile)

```sh
git -c gpg.format=ssh -c user.signingkey="$CHAVE_DE_PUBLICACAO" \
    tag -s v0.1.0 -m 'Catálogo Melkor v0.1.0' "$COMMIT_APROVADO"
git push origin refs/tags/v0.1.0
```

Uma máquina pode sincronizar a tag após receber a chave pública confiável. O caminho do arquivo de assinantes é local à máquina.

```sh
melkor catalogo sincronizar ssh://git@github.com/wkoch/Melkor-Recipes.git v0.1.0 ./allowed_signers
melkor instalar <nome>
```

O repositório privado usa uma identidade de leitura configurada no OpenSSH e a chave de host do GitHub já validada. A instalação consulta a versão sincronizada no perfil sem precisar falar com o GitHub para cada busca.
