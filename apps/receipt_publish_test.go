package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPublicacaoEmpurraSomenteReceitaERetomaPR(t *testing.T) {
	checkout, app := catalogFixture(t)
	remote := t.TempDir()
	gitTest(t, remote, "init", "--quiet", "--bare")
	gitTest(t, checkout, "push", remote, "main:main")
	origin := "https://github.com/example/melkor-recipes.git"
	gitTest(t, checkout, "remote", "add", "origin", origin)

	work := t.TempDir()
	gitConfig := filepath.Join(work, "gitconfig")
	config := "[url \"file://" + remote + "\"]\n    insteadOf = " + origin + "\n"
	if err := os.WriteFile(gitConfig, []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", gitConfig)
	t.Setenv("PATH", work+":"+os.Getenv("PATH"))
	t.Setenv("GH_TEST_PR", filepath.Join(work, "pr"))
	stub := `#!/bin/sh
set -eu
test "$1" = pr
if [ "$2" = view ]; then
    test -f "$GH_TEST_PR"
else
    test "$2" = create
    test "$3 $4" = "--repo example/melkor-recipes"
    test "$5" = --head
    test "$7 $8" = "--base main"
    test ! -e "$GH_TEST_PR"
    touch "$GH_TEST_PR"
fi
printf '%s\n' https://github.com/example/melkor-recipes/pull/1
`
	if err := os.WriteFile(filepath.Join(work, "gh"), []byte(stub), 0o700); err != nil {
		t.Fatal(err)
	}
	profile := t.TempDir()
	app.Version = "novo"
	file := filepath.Join(profile, "catalogo", "locais", "editor.json")
	if err := writeJSONAtomic(file, app); err != nil {
		t.Fatal(err)
	}
	if err := recipeCommand(profile, []string{"publicar", "editor", checkout}, nil); err != nil {
		t.Fatal(err)
	}
	hash, err := app.identity()
	if err != nil {
		t.Fatal(err)
	}
	branch := "refs/heads/melkor/receita-editor-" + hash[:12]
	commit := gitTest(t, remote, "rev-parse", branch)
	if diff := gitTest(t, remote, "diff", "--name-only", "main", commit); diff != "recipes/editor.json" {
		t.Fatalf("a publicação levou arquivos inesperados: %s", diff)
	}
	gitTest(t, checkout, "commit", "--allow-empty", "-qm", "main avançou")
	gitTest(t, checkout, "push", remote, "main:main")
	t.Setenv("GIT_INDEX_FILE", filepath.Join(work, "indice-externo"))
	if err := publishRecipe(profile, "editor", checkout); err != nil {
		t.Fatal(err)
	}
	if current := gitTest(t, remote, "rev-parse", branch); current != commit {
		t.Fatal("o retry regravou a branch de publicação")
	}
	gitTest(t, remote, "update-ref", branch, "main")
	if err := publishRecipe(profile, "editor", checkout); err == nil || !strings.Contains(err.Error(), "conteúdo diferente") {
		t.Fatalf("branch divergente deveria ser recusada: %v", err)
	}
	if changes := gitTest(t, checkout, "status", "--porcelain"); changes != "" {
		t.Fatalf("checkout de publicação foi modificado: %s", changes)
	}
}

func TestPublicacaoValidaRepositorioETipoDeOrigem(t *testing.T) {
	for _, origin := range []string{"https://github.com/wkoch/melkor-recipes.git",
		"ssh://git@github.com/wkoch/melkor-recipes", "git@github.com:wkoch/melkor-recipes.git"} {
		if repo, err := githubRepo(origin); err != nil || repo != "wkoch/melkor-recipes" {
			t.Errorf("%s: %s (%v)", origin, repo, err)
		}
	}
	for _, origin := range []string{"https://github.com/u", "https://other/u/repo.git",
		"https://github.com/u/repo/sub", "https://secret@github.com/u/repo.git", "git@other:w/u"} {
		if _, err := githubRepo(origin); err == nil {
			t.Errorf("origem inválida aceita: %s", origin)
		}
	}
	if err := publishRecipe(t.TempDir(), "sem-receita", t.TempDir()); err == nil {
		t.Fatal("publicação sem aceite local")
	}
}
