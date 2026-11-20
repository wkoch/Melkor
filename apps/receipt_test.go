package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReceitaAssistidaReutilizaDefaultsEAtivaSomenteDepois(t *testing.T) {
	t.Setenv("MELKOR_CATALOGO", "")
	profile := t.TempDir()
	path := filepath.Join(t.TempDir(), "proposta.json")
	app, err := lookup("nushell")
	if err != nil {
		t.Fatal(err)
	}
	app.Defaults = map[string]string{"exemplo/config.toml": "theme = \"escuro\"\n"}
	if err := writeJSONAtomic(path, app); err != nil {
		t.Fatal(err)
	}
	rejected := errors.New("build reprovado")
	if _, err := testRecipe(profile, "editor", path, func(string, recipe, string) error { return rejected }); !errors.Is(err, rejected) {
		t.Fatalf("falha de build: %v", err)
	}
	dir := filepath.Join(profile, "catalogo", "testes")
	if files, err := os.ReadDir(dir); err != nil || len(files) != 0 {
		t.Fatalf("proposta reprovada vazou: %v (%v)", files, err)
	}
	build := func(name string, candidate recipe, bin string) error {
		for file := range candidate.Files {
			if err := os.WriteFile(filepath.Join(bin, file), []byte(name), 0o755); err != nil {
				return err
			}
		}
		return nil
	}
	if err := recipeCommand(profile, []string{"testar", "editor", path}, build); err != nil {
		t.Fatal(err)
	}
	trials, err := os.ReadDir(dir)
	if err != nil || len(trials) != 1 {
		t.Fatalf("resultado do teste: %v (%v)", trials, err)
	}
	config := "config/exemplo/config.toml"
	if data, err := os.ReadFile(filepath.Join(dir, trials[0].Name(), config)); err != nil || string(data) != app.Defaults["exemplo/config.toml"] {
		t.Fatalf("configuração do preview: %q (%v)", data, err)
	}
	if _, _, err := resolveRecipe(profile, "editor", catalogSnapshot{}); err == nil {
		t.Fatal("proposta de teste foi aceita automaticamente")
	}
	if err := recipeCommand(profile, []string{"aceitar", trials[0].Name()}, build); err != nil {
		t.Fatal(err)
	}
	if err := update(profile, "editor", build); err != nil {
		t.Fatal(err)
	}
	first, packages, err := readProfile(profile)
	if err != nil || packages["editor"].Catalog != "local" {
		t.Fatalf("manifesto inesperado: %v (%v)", packages, err)
	}
	// Instalar outro aplicativo deve preservar a configuração já ativa.
	other := app
	other.Files = map[string]string{"outro": "target/outro"}
	other.Checks = []check{{Run: []string{"outro"}}}
	other.Defaults = nil
	if err := writeJSONAtomic(filepath.Join(profile, "catalogo", "locais", "outro.json"), other); err != nil {
		t.Fatal(err)
	}
	if err := update(profile, "outro", build); err != nil {
		t.Fatal(err)
	}
	current, _, err := readProfile(profile)
	if err != nil || current == first {
		t.Fatalf("geração não avançou: %q (%v)", current, err)
	}
	if data, err := os.ReadFile(filepath.Join(current, config)); err != nil || string(data) != app.Defaults["exemplo/config.toml"] {
		t.Fatalf("configuração ativa: %q (%v)", data, err)
	}
	if err := checkOwners(map[string]installed{"a": {Config: []string{"x/a"}}, "b": {Config: []string{"x/a"}}}); err == nil {
		t.Fatal("dois aplicativos declarando a mesma configuração foram aceitos")
	}
}

func TestReceitasInvalidasSaoRejeitadas(t *testing.T) {
	t.Setenv("MELKOR_CATALOGO", "")
	profile, dir := t.TempDir(), t.TempDir()
	path := filepath.Join(dir, "teste.json")
	for _, data := range []string{`{"secret":"fora do schema"}`, `{} {}`, strings.Repeat(" ", maxRecipeSize+1)} {
		if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := testRecipe(profile, "teste", path, nil); err == nil {
			t.Fatalf("receita inválida de %d bytes aceita", len(data))
		}
	}
	if _, err := testRecipe(profile, "../teste", path, nil); err == nil {
		t.Fatal("nome inválido aceito")
	}
	for _, id := range []string{"../teste", "teste-ausente"} {
		if _, err := acceptRecipe(profile, id); err == nil {
			t.Fatalf("aceite inválido: %s", id)
		}
	}
	if _, err := validateRecipes(dir); err == nil {
		t.Fatal("catálogo com receita inválida aceito")
	}
	if _, err := validateRecipes(t.TempDir()); err == nil {
		t.Fatal("catálogo vazio aceito")
	}
	if err := recipeCommand(profile, nil, nil); err == nil {
		t.Fatal("comando incompleto aceito")
	}
	app, err := lookup("nushell")
	if err != nil {
		t.Fatal(err)
	}
	app.Defaults = map[string]string{"../fora": "inválido"}
	if err := app.validate(); err == nil {
		t.Fatal("configuração fora do perfil aceita")
	}
}
