package main

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestLauncherSelecionaShellEPath(t *testing.T) {
	profile := t.TempDir()
	bin := filepath.Join(profile, "atual", "bin")
	t.Setenv("MELKOR_PERFIL", profile)
	t.Setenv("PATH", "/bin")
	app := filepath.Join(bin, "nu")
	sent := errors.New("exec do teste")
	for _, shell := range []string{"/bin/sh", app} {
		if shell == app {
			if err := os.MkdirAll(bin, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(app, nil, 0o700); err != nil {
				t.Fatal(err)
			}
		}
		replace := func(prog string, args, env []string) error {
			if prog != shell || !slices.Equal(args, []string{shell, "-c", "exit 0"}) {
				t.Fatalf("shell ou argumentos incorretos: %s %v", prog, args)
			}
			if !slices.Contains(env, "PATH="+bin+":/bin") || slices.Contains(env, "PATH=/bin") {
				t.Fatalf("o launcher não priorizou os executáveis ativos")
			}
			return sent
		}
		if err := run([]string{"shell", "-c", "exit 0"}, replace); !errors.Is(err, sent) {
			t.Fatalf("esperava executar %s, obtive %v", shell, err)
		}
	}
}

func TestCLIValidaComandosECatalogo(t *testing.T) {
	t.Setenv("MELKOR_PERFIL", t.TempDir())
	t.Setenv("MELKOR_CATALOGO", t.TempDir())
	for _, test := range []struct {
		args []string
		want string
	}{
		{nil, ""},
		{[]string{"atualizar"}, ""},
		{[]string{"atualizar", "extra"}, "Uso"},
		{[]string{"instalar"}, "Uso"},
		{[]string{"instalar", ""}, "Uso"},
		{[]string{"instalar", "../fora"}, "inválido"},
		{[]string{"instalar", "ausente"}, "receita ausente"},
	} {
		err := run(test.args, nil)
		if test.want == "" {
			if err != nil {
				t.Errorf("%v: %v", test.args, err)
			}
		} else if err == nil || !strings.Contains(err.Error(), test.want) {
			t.Errorf("%v: esperava %q, obtive %v", test.args, test.want, err)
		}
	}
}

func TestPerfilPadraoSegueXDG(t *testing.T) {
	t.Setenv("MELKOR_PERFIL", "")
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", "relativo")
	if dir, err := profileDir(); err != nil || dir != filepath.Join(home, ".local", "share", "melkor") {
		t.Fatalf("perfil padrão: %q %v", dir, err)
	}
	t.Setenv("XDG_DATA_HOME", home)
	if dir, err := profileDir(); err != nil || dir != filepath.Join(home, "melkor") {
		t.Fatalf("perfil XDG: %q %v", dir, err)
	}
	t.Setenv("MELKOR_PERFIL", "relativo")
	if _, err := profileDir(); err == nil {
		t.Fatal("um perfil alternativo relativo foi aceito")
	}
}
