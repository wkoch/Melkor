package pcdisk

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var testCredentials = credentials{
	root: "$6$rootlab$" + strings.Repeat("a", 86),
	user: "$6$userlab$" + strings.Repeat("b", 86),
}

func rawCredentials(c credentials) string {
	return "root:" + c.root + "\nmelkor:" + c.user + "\n"
}

func TestCredenciaisRecusamEntradasAmbiguasOuExpostas(t *testing.T) {
	valid := testCredentials
	input := filepath.Join(t.TempDir(), "hashes")
	for _, test := range []struct {
		name string
		body string
		mode os.FileMode
	}{
		{"senha_em_texto", "root:secreta\nmelkor:outra\n", 0o600},
		{"permissao_publica", rawCredentials(valid), 0o644},
		{"mesmo_hash", rawCredentials(credentials{root: valid.root, user: valid.root}), 0o600},
		{"incompleto", "root:" + valid.root, 0o600},
		{"sufixo_extra", rawCredentials(valid) + "daemon:*", 0o600},
		{"grande", strings.Repeat("x", 1024), 0o600},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := os.WriteFile(input, []byte(test.body), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(input, test.mode); err != nil {
				t.Fatal(err)
			}
			_, err := readCredentials(input, nil)
			if err == nil || strings.Contains(err.Error(), valid.root) {
				t.Fatal("credenciais inválidas foram aceitas ou expostas")
			}
		})
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(input, link); err != nil {
		t.Fatal(err)
	}
	if _, err := readCredentials(link, nil); err == nil {
		t.Fatal("link simbólico aceito")
	}
}

func TestEstagioDeCredenciaisMantemPermissoesPrivadas(t *testing.T) {
	c := testCredentials
	work := t.TempDir()
	input := filepath.Join(work, "hashes")
	if err := os.WriteFile(input, []byte(rawCredentials(c)), 0o600); err != nil {
		t.Fatal(err)
	}
	seed, err := stageCredentials(input, work)
	if err != nil {
		t.Fatal(err)
	}
	for name, mode := range map[string]os.FileMode{".melkor": 0o700, ".melkor/credenciais": 0o600} {
		info, err := os.Stat(filepath.Join(seed, name))
		if err != nil || info.Mode().Perm() != mode {
			t.Fatalf("permissão inválida em %s: %+v, %v", name, info, err)
		}
	}
	data, err := os.ReadFile(filepath.Join(seed, ".melkor", "credenciais"))
	if err != nil || string(data) != rawCredentials(c) {
		t.Fatal("dados do arquivo de credenciais divergentes")
	}
}

func TestComposicaoShadowPreservaContasDaGeracao(t *testing.T) {
	base := "root:*:0:0:99999:7:::\ndbus:!:0:0:99999:7:::\nmelkor:!:0:2:12345:7:::\n"
	c := testCredentials
	want := strings.ReplaceAll(strings.ReplaceAll(base, "root:*:", "root:"+c.root+":"), "melkor:!:", "melkor:"+c.user+":")
	result, err := c.mergeShadow(base)
	if err != nil || result != want {
		t.Fatal("shadow diverge da configuração local")
	}
	for _, invalid := range []string{base + "root:!:0:\n", base + "melkor:*:0:\n", "dbus:!:0:\nroot:!\n", "root:*:0:\n"} {
		if _, err := c.mergeShadow(invalid); err == nil {
			t.Fatal("contas ausentes ou duplicadas foram aceitas")
		}
	}
}

func TestBootBloqueiaProvisaoComDiretorioDeCredenciaisInseguro(t *testing.T) {
	work := t.TempDir()
	bind := func(string, string) error { return errors.New("montagem inesperada") }
	if err := ApplyCredentials(work, "/ausente", work, bind); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(work, ".melkor"), 0o755); err != nil {
		t.Fatal(err)
	}
	err := ApplyCredentials(work, "/ausente", work, bind)
	if err == nil || !strings.Contains(err.Error(), "0700") {
		t.Fatalf("erro: %v", err)
	}
}

func TestProvisaoPublicaShadowPrivadoEReverteFalhaDeMount(t *testing.T) {
	work := t.TempDir()
	secrets := filepath.Join(work, ".melkor")
	if err := os.Mkdir(secrets, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(secrets, "credenciais"), []byte(rawCredentials(testCredentials)), 0o600); err != nil {
		t.Fatal(err)
	}
	stock := filepath.Join(work, "shadow")
	if err := os.WriteFile(stock, []byte("root:!:0:\ndbus:!:0:\nmelkor:!:0:\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	validOwner := func(info os.FileInfo) bool { return true }
	errBind := errors.New("montagem recusada")
	var mounted string
	bind := func(source, target string) error {
		if target != stock {
			t.Fatal("destino inesperado")
		}
		mounted = source
		info, err := os.Stat(source)
		if err != nil || info.Mode().Perm() != 0o600 {
			t.Fatal("shadow temporário tem permissões inválidas")
		}
		content, err := os.ReadFile(source)
		if err != nil || !strings.Contains(string(content), "root:"+testCredentials.root+":0:\ndbus:!:0:") {
			t.Fatal("shadow temporário ausente ou incorreto")
		}
		return errBind
	}
	if err := applyCredentials(work, stock, work, validOwner, bind); !errors.Is(err, errBind) {
		t.Fatalf("erro: %v", err)
	}
	if _, err := os.Stat(mounted); !os.IsNotExist(err) {
		t.Fatal("shadow temporário sobreviveu à falha")
	}
	if err := applyCredentials(work, stock, work, validOwner, func(string, string) error { return nil }); err != nil {
		t.Fatal(err)
	}
	rejectFileOwner := func(info os.FileInfo) bool { return info.IsDir() }
	if err := applyCredentials(work, stock, work, rejectFileOwner, bind); err == nil {
		t.Fatal("arquivo com dono inesperado aceito")
	}
}
