package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCatalogoRejeitaReceitaSemPinosOuVerificacoes(t *testing.T) {
	t.Setenv("MELKOR_CATALOGO", "")
	for _, tc := range []struct {
		name   string
		change func(*recipe)
	}{
		{"hash de fonte", func(r *recipe) { r.SHA256 = "sem hash" }},
		{"fonte sem TLS", func(r *recipe) { r.Source = "http://example.invalid" }},
		{"credencial na origem", func(r *recipe) { r.Origin = "https://usuario:senha@example.invalid" }},
		{"imagem sem digest", func(r *recipe) { r.Image = "rust:latest" }},
		{"verificação ausente", func(r *recipe) { r.Checks = nil }},
		{"verificação sem artefato", func(r *recipe) { r.Checks[0].Run = []string{"outro"} }},
		{"comando inválido", func(r *recipe) { r.Fetch = []string{""} }},
		{"saída fora do build", func(r *recipe) { r.Files["nu"] = "../perfil/bin/nu" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app, err := lookup("nushell")
			if err != nil {
				t.Fatal(err)
			}
			tc.change(&app)
			if err := app.validate(); err == nil {
				t.Fatal("receita inválida foi aceita")
			}
		})
	}
}

func TestCatalogoExternoRejeitaArquivoJSONInvalido(t *testing.T) {
	catalog := t.TempDir()
	t.Setenv("MELKOR_CATALOGO", catalog)
	file := filepath.Join(catalog, "nushell.json")
	for _, content := range []string{"{", `{"version":"sem hash"}`} {
		if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := lookup("nushell"); err == nil || !strings.Contains(err.Error(), "receita nushell") {
			t.Fatalf("catálogo inválido: %v", err)
		}
	}
}
