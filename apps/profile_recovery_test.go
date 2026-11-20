package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func retainedGeneration(t *testing.T, profile, name string, packages map[string]installed) string {
	t.Helper()
	dir := filepath.Join(profile, "geracoes", name)
	for _, part := range []string{"bin", "config"} {
		if err := os.MkdirAll(filepath.Join(dir, part), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := writeProfile(dir, packages); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestVoltaParaGeracaoNativaSemTocarDados(t *testing.T) {
	profile := t.TempDir()
	packages := map[string]installed{"editor": {Version: "1.0", Files: []string{"editor"}, Config: []string{"editor.conf"}}}
	old := retainedGeneration(t, profile, "g-antiga", packages)
	for _, file := range []string{"bin/editor", "config/editor.conf"} {
		if err := os.WriteFile(filepath.Join(old, file), []byte("anterior"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	newer := retainedGeneration(t, profile, "g-vazia", map[string]installed{})
	if err := os.Chtimes(old, time.Unix(111, 0), time.Unix(111, 0)); err != nil {
		t.Fatal(err)
	}
	if _, err := activateGeneration(profile, filepath.Base(newer)); err != nil {
		t.Fatal(err)
	}
	data := filepath.Join(profile, "documento")
	if err := os.WriteFile(data, []byte("permanece"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MELKOR_PERFIL", profile)
	if err := run([]string{"voltar", "g-antiga"}, nil); err != nil {
		t.Fatal(err)
	}
	path, got, err := readProfile(profile)
	if err != nil || path != old || got["editor"].Version != "1.0" {
		t.Fatalf("perfil ativo: %v %v %v", path, got, err)
	}
	var listing bytes.Buffer
	if err := listGenerations(profile, &listing); err != nil {
		t.Fatal(err)
	}
	if listing.String() != "  g-vazia  perfil vazio\n* g-antiga  editor 1.0\n" {
		t.Fatalf("lista de gerações: %q", listing.String())
	}
	if err := run([]string{"geracoes"}, nil); err != nil {
		t.Fatal(err)
	}
	if text, err := os.ReadFile(data); err != nil || string(text) != "permanece" {
		t.Fatalf("dados: %q %v", text, err)
	}
	entries, err := os.ReadDir(filepath.Join(profile, "geracoes"))
	if err != nil || len(entries) != 2 {
		t.Fatalf("o retorno criou uma geração: %v %v", entries, err)
	}
}

func TestRecusaGeracaoIncompletaEMantemAtual(t *testing.T) {
	profile := t.TempDir()
	retainedGeneration(t, profile, "g-atual", map[string]installed{})
	if _, err := activateGeneration(profile, "g-atual"); err != nil {
		t.Fatal(err)
	}
	image := &binaryPackage{Image: testPackageImage, Entry: "/usr/bin/tree"}
	native := retainedGeneration(t, profile, "g-bin", map[string]installed{"editor": {Files: []string{"editor"}}})
	config := retainedGeneration(t, profile, "g-config", map[string]installed{"editor": {Config: []string{"editor.conf"}}})
	retainedGeneration(t, profile, "g-imagem", map[string]installed{"tree": {Binary: image}})
	retainedGeneration(t, profile, "g-tag", map[string]installed{"tree": {Binary: &binaryPackage{Image: "latest"}}})
	retainedGeneration(t, profile, "g-invasor", map[string]installed{"../invalido": {}})
	retainedGeneration(t, profile, "g-arquivo", map[string]installed{"editor": {Files: []string{"../editor"}}})
	retainedGeneration(t, profile, "g-rotulo", map[string]installed{"editor": {Version: "\x1b[2J"}})
	if err := os.Symlink("/bin/sh", filepath.Join(native, "bin/editor")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/etc/passwd", filepath.Join(config, "config/editor.conf")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(native, filepath.Join(profile, "geracoes/g-link")); err != nil {
		t.Fatal(err)
	}
	broken := retainedGeneration(t, profile, "g-sem-manifesto", nil)
	if err := os.Remove(filepath.Join(broken, "manifest.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/etc/passwd", filepath.Join(broken, "manifest.json")); err != nil {
		t.Fatal(err)
	}
	podman := func(_ context.Context, args ...string) (string, error) {
		if !slices.Equal(args, []string{"image", "exists", testPackageImage}) {
			t.Fatalf("consulta inesperada ao Podman: %v", args)
		}
		return "", errors.New("imagem ausente")
	}
	for _, name := range []string{"../g-atual", "g-ausente", "g-bin", "g-config", "g-imagem", "g-invasor", "g-tag",
		"g-link", "g-sem-manifesto", "g-arquivo"} {
		if err := restoreGeneration(profile, name, podman); err == nil {
			t.Fatalf("reativou geração inválida %q", name)
		}
	}
	active, _, err := readProfile(profile)
	if err != nil || filepath.Base(active) != "g-atual" {
		t.Fatalf("o perfil mudou após rejeição: %q %v", active, err)
	}
	var listing bytes.Buffer
	if err := listGenerations(profile, &listing); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(listing.String(), "g-sem-manifesto  manifesto indisponível") ||
		strings.Contains(listing.String(), "g-link") || strings.Contains(listing.String(), "\x1b") ||
		!strings.Contains(listing.String(), "editor \\x1b[2J") {
		t.Fatalf("listagem inesperada: %s", listing.String())
	}
	if err := restoreGeneration(profile, "g-imagem", func(context.Context, ...string) (string, error) {
		return "", nil
	}); err != nil {
		t.Fatalf("imagem disponível não voltou a ser ativa: %v", err)
	}
}

func TestListagemDePerfilNovoEAusenciaDeGeracao(t *testing.T) {
	profile := filepath.Join(t.TempDir(), "perfil")
	var listing bytes.Buffer
	if err := listGenerations(profile, &listing); err != nil || listing.String() != "Nenhuma geração publicada.\n" {
		t.Fatalf("lista vazia: %q %v", listing.String(), err)
	}
	if err := restoreGeneration(profile, "g-1", nil); err == nil {
		t.Fatal("retorno para um perfil inexistente foi aceito")
	}
}
