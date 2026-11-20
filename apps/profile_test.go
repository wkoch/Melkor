package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestAtualizacaoAtivaApenasGeracaoValidada(t *testing.T) {
	profile := t.TempDir()
	catalog := t.TempDir()
	t.Setenv("MELKOR_CATALOGO", "")
	app, err := lookup("nushell")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("MELKOR_CATALOGO", catalog)
	writeRecipe := func(version string) {
		t.Helper()
		app.Version = version
		data, err := json.Marshal(app)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(catalog, "nushell.json"), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	build := func(_ string, candidate recipe, bin string) error {
		return os.WriteFile(filepath.Join(bin, "nu"), []byte(candidate.Version), 0o755)
	}

	writeRecipe("v1")
	if err := update(profile, "nushell", build); err != nil {
		t.Fatal(err)
	}
	first, _, err := readProfile(profile)
	if err != nil {
		t.Fatal(err)
	}

	writeRecipe("v2")
	rejected := errors.New("falha de validação")
	for _, failure := range []error{rejected, syscall.ENOSPC} {
		failBuild := func(name string, candidate recipe, bin string) error {
			if err := build(name, candidate, bin); err != nil {
				return err
			}
			return failure
		}
		if err := update(profile, "", failBuild); !errors.Is(err, failure) {
			t.Fatalf("esperava %v, obtive %v", failure, err)
		}
		if active, _, err := readProfile(profile); err != nil || active != first {
			t.Fatalf("esperava preservar %s, obtive %s (%v)", first, active, err)
		}
		entries, err := os.ReadDir(filepath.Join(profile, "geracoes"))
		if err != nil || len(entries) != 1 {
			t.Fatalf("esperava apenas a geração inicial, obtive %v (%v)", entries, err)
		}
	}

	if err := update(profile, "", build); err != nil {
		t.Fatal(err)
	}
	active, packages, err := readProfile(profile)
	if err != nil || active == first || packages["nushell"].Version != "v2" {
		t.Fatalf("esperava uma geração v2, obtive %s %v (%v)", active, packages, err)
	}
	if data, err := os.ReadFile(filepath.Join(first, "bin", "nu")); err != nil || string(data) != "v1" {
		t.Fatalf("esperava preservar o executável v1, obtive %q (%v)", data, err)
	}
	if err := update(profile, "", func(string, recipe, string) error { return rejected }); err != nil {
		t.Fatalf("receita já ativa foi reconstruída: %v", err)
	}
}

func TestInstalacoesConcorrentesPreservamOsDoisAplicativos(t *testing.T) {
	profile, catalog := t.TempDir(), t.TempDir()
	t.Setenv("MELKOR_CATALOGO", "")
	app, err := lookup("nushell")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"alpha", "beta"} {
		app.Files = map[string]string{name: "program"}
		app.Checks = []check{{Run: []string{name}}}
		data, err := json.Marshal(app)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(catalog, name+".json"), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("MELKOR_CATALOGO", catalog)

	release := make(chan struct{})
	started := make(chan string, 2)
	done := make(chan error, 2)
	build := func(name string, _ recipe, bin string) error {
		started <- name
		if name == "alpha" {
			<-release
		}
		return os.WriteFile(filepath.Join(bin, name), []byte(name), 0o755)
	}
	go func() { done <- update(profile, "alpha", build) }()
	select {
	case name := <-started:
		if name != "alpha" {
			t.Fatalf("primeira instalação inesperada: %s", name)
		}
	case err := <-done:
		t.Fatalf("primeira instalação terminou antes do build: %v", err)
	case <-time.After(time.Second):
		t.Fatal("primeira instalação não iniciou")
	}
	go func() { done <- update(profile, "beta", build) }()
	select {
	case <-started:
		t.Error("a segunda instalação iniciou antes de liberar a primeira")
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	for range 2 {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
	current, packages, err := readProfile(profile)
	if err != nil || len(packages) != 2 {
		t.Fatalf("instalação perdida: %v (%v)", packages, err)
	}
	for _, name := range []string{"alpha", "beta"} {
		data, err := os.ReadFile(filepath.Join(current, "bin", name))
		if err != nil || string(data) != name {
			t.Fatalf("executável %s: %q (%v)", name, data, err)
		}
	}
}

func TestPerfilRejeitaManifestoEConflitoDeExecutaveis(t *testing.T) {
	profile := t.TempDir()
	active := filepath.Join(profile, "atual")
	target := filepath.Join("geracoes", "g-1")
	if err := os.MkdirAll(filepath.Join(profile, target), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, active); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(profile, target, "manifest.json")
	for _, content := range []string{"{", "null"} {
		if err := os.WriteFile(manifest, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, _, err := readProfile(profile); err == nil {
			t.Fatal("manifesto inválido foi aceito")
		}
	}
	if err := os.Remove(active); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../outro-perfil", active); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readProfile(profile); err == nil {
		t.Fatal("link ativo fora do perfil foi aceito")
	}
	for _, packages := range []map[string]installed{
		{"a": {Files: []string{"nu"}}, "b": {Files: []string{"nu"}}},
		{"a": {Files: []string{"../nu"}}},
	} {
		if err := checkOwners(packages); err == nil {
			t.Fatalf("conjunto inválido foi aceito: %v", packages)
		}
	}
}
