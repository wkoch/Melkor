package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitTest(t *testing.T, dir string, args ...string) string {
	t.Helper()
	output, err := commandOutput(context.Background(), dir, nil, "git", args...)
	if err != nil {
		t.Fatalf("git %s: %v", strings.Join(args, " "), err)
	}
	return output
}

func catalogFixture(t *testing.T) (string, recipe) {
	t.Helper()
	t.Setenv("MELKOR_CATALOGO", "")
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_AUTHOR_NAME", "Teste Melkor")
	t.Setenv("GIT_AUTHOR_EMAIL", "teste@example.invalid")
	t.Setenv("GIT_COMMITTER_NAME", "Teste Melkor")
	t.Setenv("GIT_COMMITTER_EMAIL", "teste@example.invalid")
	repo := t.TempDir()
	gitTest(t, repo, "init", "--quiet", "-b", "main")
	app, err := lookup("nushell")
	if err != nil {
		t.Fatal(err)
	}
	app.Version = "teste-1"
	if err := writeJSONAtomic(filepath.Join(repo, "recipes/nushell.json"), app); err != nil {
		t.Fatal(err)
	}
	gitTest(t, repo, "add", "recipes")
	gitTest(t, repo, "commit", "-qm", "Receita inicial")
	return repo, app
}

func generateKey(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	key := filepath.Join(dir, "signing")
	out, err := exec.Command("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", key).CombinedOutput()
	if err != nil {
		t.Fatalf("ssh-keygen: %v: %s", err, out)
	}
	pub, err := os.ReadFile(key + ".pub")
	if err != nil {
		t.Fatal(err)
	}
	trust := filepath.Join(dir, "allowed_signers")
	if err := os.WriteFile(trust, append([]byte("teste@example.invalid "), pub...), 0o600); err != nil {
		t.Fatal(err)
	}
	return key, trust
}

func signTag(t *testing.T, repo, key, tag string) {
	t.Helper()
	gitTest(t, repo, "-c", "gpg.format=ssh", "-c", "user.signingkey="+key, "tag", "-s", tag, "-m", tag)
}

func TestCatalogoAssinadoFixaProcedenciaEConservaVersaoEmFalha(t *testing.T) {
	repo, app := catalogFixture(t)
	key, trust := generateKey(t)
	otherKey, _ := generateKey(t)
	profile := t.TempDir()
	signTag(t, repo, key, "v1.0.0")
	signTag(t, repo, otherKey, "v1.0.1")
	gitTest(t, repo, "tag", "sem-assinatura")
	gitTest(t, repo, "update-ref", "refs/tags/tag-trocada", "refs/tags/v1.0.0")
	if err := catalogCommand(profile, []string{"sincronizar", repo, "v1.0.0", trust}); err != nil {
		t.Fatal(err)
	}
	snapshot, err := readCatalog(profile)
	if err != nil || snapshot.Tag != "v1.0.0" || snapshot.Recipes["nushell"].Version != app.Version {
		t.Fatalf("snapshot inválido: %+v (%v)", snapshot, err)
	}
	build := func(name string, candidate recipe, bin string) error {
		return os.WriteFile(filepath.Join(bin, "nu"), []byte(candidate.Version), 0o755)
	}
	if err := update(profile, "nushell", build); err != nil {
		t.Fatal(err)
	}
	_, installed, err := readProfile(profile)
	if err != nil || installed["nushell"].Catalog != "v1.0.0@"+snapshot.Commit {
		t.Fatalf("origem ausente do manifesto: %+v (%v)", installed, err)
	}

	file := filepath.Join(repo, "recipes/nushell.json")
	if err := os.WriteFile(file, []byte(`{"version":"incorreta"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	gitTest(t, repo, "add", "recipes")
	gitTest(t, repo, "commit", "-qm", "Receita inválida")
	signTag(t, repo, key, "receita-invalida")
	gitTest(t, repo, "rm", "recipes/nushell.json")
	if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../destino", file); err != nil {
		t.Fatal(err)
	}
	gitTest(t, repo, "add", "recipes")
	gitTest(t, repo, "commit", "-qm", "Entrada simbólica")
	signTag(t, repo, key, "link-invalido")
	for _, tc := range []struct{ tag, expected string }{
		{"v1.0.1", "assinatura"},
		{"sem-assinatura", "assinatura"},
		{"tag-trocada", "não corresponde"},
		{"receita-invalida", "receita nushell"},
		{"link-invalido", "entrada inválida"},
		{"inexistente", "fetch"},
		{"../refs", "tag inválida"},
	} {
		t.Run(tc.tag, func(t *testing.T) {
			_, err := syncCatalog(profile, repo, tc.tag, trust)
			if err == nil || !strings.Contains(err.Error(), tc.expected) {
				t.Fatalf("esperava %q: %v", tc.expected, err)
			}
			after, err := readCatalog(profile)
			if err != nil || after.Commit != snapshot.Commit || after.Tag != snapshot.Tag {
				t.Fatalf("snapshot ativo foi substituído: %+v (%v)", after, err)
			}
		})
	}
}

func TestOrigensDoCatalogo(t *testing.T) {
	for _, origin := range []string{"/tmp/catalogo", "https://github.com/u/repo.git", "ssh://git@github.com/u/repo.git"} {
		if !validCatalogOrigin(origin) {
			t.Errorf("origem válida rejeitada: %q", origin)
		}
	}
	for _, origin := range []string{"http://example.invalid", "https://user@github.com/u/repo",
		"https://github.com/u/repo?x=y", "ssh://root@github.com/repo", "ssh://git@outro/repo", "ext::x", "relativo"} {
		if validCatalogOrigin(origin) {
			t.Errorf("origem inválida aceita: %q", origin)
		}
	}
	if err := catalogCommand(t.TempDir(), nil); err == nil {
		t.Fatal("comando incompleto aceito")
	}
	repo, _ := catalogFixture(t)
	if err := catalogCommand(t.TempDir(), []string{"validar", filepath.Join(repo, "recipes")}); err != nil {
		t.Fatal(err)
	}
	if _, err := syncCatalog(t.TempDir(), repo, "v1", filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("sincronização sem chaves autorizadas")
	}
}

func TestSnapshotInvalidoNaoViraCatalogo(t *testing.T) {
	t.Setenv("MELKOR_CATALOGO", "")
	profile := t.TempDir()
	path := filepath.Join(profile, "catalogo", "atual.json")
	if err := writeJSONAtomic(path, json.RawMessage(`{}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := readCatalog(profile); err == nil {
		t.Fatal("catálogo sem procedência foi aceito")
	}
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readCatalog(profile); err == nil {
		t.Fatal("JSON inválido foi aceito")
	}
	if _, err := syncCatalog(profile, "http://example.invalid", "v1", "chaves"); err == nil {
		t.Fatal("origem HTTP foi aceita")
	}
}
