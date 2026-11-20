package main

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
)

const testPackageImage = "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func localPackageFixture(t *testing.T, suffix string) (string, string) {
	t.Helper()
	file := filepath.Join(t.TempDir(), "pacote com espaço"+suffix)
	content := []byte("pacote simulado")
	if err := os.WriteFile(file, content, 0o600); err != nil {
		t.Fatal(err)
	}
	return file, fmt.Sprintf("%x", sha256.Sum256(content))
}

func packageOption(args []string, key string) string {
	for _, arg := range args {
		if value, found := strings.CutPrefix(arg, key+"="); found {
			return value
		}
	}
	return ""
}

type packageSimulation struct {
	t        *testing.T
	format   string
	metadata string
	fail     string
	calls    [][]string
}

func (s *packageSimulation) command(_ context.Context, args ...string) (string, error) {
	s.t.Helper()
	s.calls = append(s.calls, slices.Clone(args))
	switch args[0] {
	case "info":
		if s.fail == "rootful" {
			return "false", nil
		}
		if s.fail == "podman" {
			return "", errors.New("podman indisponível")
		}
		return "true\n", nil
	case "build":
		mode := packageOption(args, "--network")
		data, err := os.ReadFile(packageOption(args, "--file"))
		if err != nil {
			s.t.Fatal(err)
		}
		in, err := os.ReadFile(filepath.Join(args[len(args)-1], "pacote."+s.format))
		if err != nil || string(in) != "pacote simulado" {
			s.t.Fatalf("entrada inesperada: %q (%v)", in, err)
		}
		if mode == "private" {
			if !strings.Contains(string(data), "--download") || packageOption(args, "--pull") != "missing" {
				s.t.Fatalf("obtenção de dependências: %q", data)
			}
		} else if mode != "none" || packageOption(args, "--pull") != "never" ||
			!strings.Contains(string(data), "COPY --from=dependencias") {
			s.t.Fatalf("instalação %q: %q", mode, data)
		}
		if mode == s.fail {
			return "", errors.New("transação recusada")
		}
		if s.fail == "sem-id" {
			return "", nil
		}
		image := testPackageImage
		if s.fail == "id-invalido" {
			image = "tag-inesperada"
		}
		return "", os.WriteFile(packageOption(args, "--iidfile"), []byte(image+"\n"), 0o600)
	case "run":
		for _, expected := range []string{"--pull=never", "--read-only", "--network=none", "--cap-drop=all",
			"--image-volume=ignore", "--security-opt=no-new-privileges", "--env=HOME=/home/melkor"} {
			if !slices.Contains(args, expected) {
				s.t.Fatalf("faltou %s em %q", expected, args)
			}
		}
		if packageOption(args, "--entrypoint") == "/bin/cat" {
			if s.fail == "ler-metadados" {
				return "", errors.New("metadados indisponíveis")
			}
			return s.metadata, nil
		}
		if s.fail == "verificar" {
			return "", errors.New("executável falhou")
		}
		return "versão", nil
	case "image":
		if len(args) != 4 || args[1] != "rm" || args[2] != "--ignore" ||
			!strings.HasPrefix(args[3], "localhost/melkor-pacote:") {
			s.t.Fatalf("remoção inesperada: %q", args)
		}
		return "", nil
	default:
		s.t.Fatalf("comando inesperado: %q", args)
		return "", nil
	}
}

func TestImportacaoMantemPerfilEPassaArgumentosIsolados(t *testing.T) {
	profile := t.TempDir()
	deb, debSHA := localPackageFixture(t, ".deb")
	rpm, rpmSHA := localPackageFixture(t, ".rpm")
	debian := &packageSimulation{t: t, format: "deb", metadata: "fornecedor\n2:1.0-2~test\namd64\n"}
	fedora := &packageSimulation{t: t, format: "rpm", metadata: "outro\n2.0-1\nnoarch\n"}
	if err := importPackage(profile, []string{"primeiro", deb, debSHA, "/usr/bin/primeiro", "--version"}, debian.command); err != nil {
		t.Fatal(err)
	}
	generation, _, err := readProfile(profile)
	if err != nil {
		t.Fatal(err)
	}
	key := filepath.Join(t.TempDir(), "publica.asc")
	if err := os.WriteFile(key, []byte("chave pública de teste"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := importPackage(profile, []string{"--chave-rpm=" + key, "segundo", rpm, rpmSHA, "/opt/app/run"}, fedora.command); err != nil {
		t.Fatal(err)
	}
	current, entries, err := readProfile(profile)
	if err != nil || current == generation || len(entries) != 2 {
		t.Fatalf("perfil: %s %v %v", current, entries, err)
	}
	first, second := entries["primeiro"], entries["segundo"]
	if first.Version != "2:1.0-2~test" || first.Binary.SourceSHA != debSHA || first.Binary.Base != debianPackageBase ||
		second.Binary.Name != "outro" || second.Binary.Arch != "noarch" || second.Binary.KeySHA == "" ||
		second.Binary.Image != testPackageImage || first.Binary.Tag == second.Binary.Tag {
		t.Fatalf("procedência: %+v %+v", first.Binary, second.Binary)
	}
	if err := update(profile, "", func(string, recipe, string) error {
		t.Fatal("uma importação foi reconstruída pelo catálogo")
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := update(profile, "primeiro", nil); err == nil {
		t.Fatal("uma receita substituiu o pacote importado")
	}
	if os.Geteuid() != 0 {
		fake := filepath.Join(t.TempDir(), "podman")
		if err := os.WriteFile(fake, []byte("#!/bin/sh\n"), 0o700); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", filepath.Dir(fake))
		err := runPackage(profile, "primeiro", []string{"--flag", "dois termos"}, func(program string, argv, _ []string) error {
			if program != fake || argv[1] != "--remote=false" || !slices.Contains(argv, testPackageImage) ||
				!slices.Equal(argv[len(argv)-2:], []string{"--flag", "dois termos"}) ||
				!slices.Contains(argv, "--volume="+filepath.Join(profile, "dados/primeiro")+":/home/melkor:z") {
				t.Fatalf("launcher: %q %q", program, argv)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := removeFromProfile(profile, "primeiro"); err != nil {
		t.Fatal(err)
	}
	_, entries, err = readProfile(profile)
	if err != nil || entries["primeiro"].Binary != nil || entries["segundo"].Binary == nil {
		t.Fatalf("remoção alterou outro pacote: %v %v", entries, err)
	}
	if _, err := os.Stat(filepath.Join(generation, "manifest.json")); err != nil {
		t.Fatalf("geração anterior ausente: %v", err)
	}
	if err := removeFromProfile(profile, "desconhecido"); err == nil {
		t.Fatal("remoção de pacote ausente foi aceita")
	}
}

func TestImportacaoFalhaSemTrocarGeracaoOuReterCandidata(t *testing.T) {
	profile := t.TempDir()
	file, digest := localPackageFixture(t, ".deb")
	args := []string{"teste", file, digest, "/usr/bin/ok"}
	sim := &packageSimulation{t: t, format: "deb", metadata: "teste\n1\nall\n"}
	if err := importPackage(profile, args, sim.command); err != nil {
		t.Fatal(err)
	}
	generation, _, err := readProfile(profile)
	if err != nil {
		t.Fatal(err)
	}
	for _, failure := range []string{"rootful", "podman", "private", "sem-id", "id-invalido", "ler-metadados", "none", "verificar"} {
		t.Run(failure, func(t *testing.T) {
			backend := &packageSimulation{t: t, format: "deb", metadata: sim.metadata, fail: failure}
			if err := importPackage(profile, args, backend.command); err == nil {
				t.Fatal("importação inválida ativada")
			}
			check, _, err := readProfile(profile)
			if err != nil || check != generation {
				t.Fatalf("a geração ativa mudou: %q %v", check, err)
			}
			candidates, err := filepath.Glob(filepath.Join(profile, "geracoes", ".candidata-*"))
			if err != nil || len(candidates) != 0 {
				t.Fatalf("candidatas sobraram: %v %v", candidates, err)
			}
			if failure == "verificar" {
				last := backend.calls[len(backend.calls)-1]
				if last[0] != "image" || !strings.Contains(last[3], ":app-") {
					t.Fatalf("imagem reprovada ficou no Podman: %v", backend.calls)
				}
			}
		})
	}
}

func TestEntradaLocalEValidadaAntesDoPodman(t *testing.T) {
	file, digest := localPackageFixture(t, ".deb")
	rpm, rpmDigest := localPackageFixture(t, ".rpm")
	invalid := [][]string{
		nil, {"--chave-rpm="}, {"../escape", file, digest, "/bin/foo"},
		{"ok", file, "sha", "/bin/foo"}, {"ok", file, digest, "/usr/../bin/foo"},
		{"ok", "pacote.tgz", digest, "/bin/foo"}, {"ok", "ausente.deb", digest, "/bin/foo"},
		{"ok", file, strings.Repeat("a", 64), "/bin/foo"},
		{"--chave-rpm=chave.asc", "ok", file, digest, "/bin/foo"},
		{"--chave-rpm=ausente.asc", "ok", rpm, rpmDigest, "/bin/foo"},
	}
	for _, args := range invalid {
		if err := importPackage(t.TempDir(), args, func(context.Context, ...string) (string, error) {
			t.Fatalf("Podman foi chamado com entrada inválida: %q", args)
			return "", nil
		}); err == nil {
			t.Fatalf("entrada aceita: %q", args)
		}
	}

	target := filepath.Join(t.TempDir(), "destino")
	if _, err := stagePackageFile(file, target, 1); err == nil {
		t.Fatal("arquivo grande aceito")
	}
	dir := t.TempDir()
	fifo := filepath.Join(dir, "fifo")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := stagePackageFile(fifo, target, 1024); err == nil {
		t.Fatal("FIFO aceito")
	}
	if _, err := stagePackageFile(file, dir, 1024); err == nil {
		t.Fatal("diretório aberto para escrita")
	}
	for _, info := range []string{"x\n1\narm64", "linha-invalida", "x\n1\naarch64", "../pacote\n1\nall"} {
		format := "deb"
		if strings.HasSuffix(info, "aarch64") {
			format = "rpm"
		}
		if _, _, _, err := packageMetadata(info, format); err == nil {
			t.Fatalf("metadados aceitos: %q", info)
		}
	}
}

func TestDadosIsoladosESaidaDePodman(t *testing.T) {
	base := t.TempDir()
	parent := filepath.Join(base, "perfil")
	if err := os.Mkdir(parent, 0o700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(base, "alias")
	if err := os.Symlink(parent, target); err != nil {
		t.Fatal(err)
	}
	if got, err := packageDataDir(target, "teste"); err != nil || got != filepath.Join(parent, "dados", "teste") {
		t.Fatalf("diretório do perfil: %q %v", got, err)
	}
	if err := os.Symlink(base, filepath.Join(parent, "dados", "escape")); err != nil {
		t.Fatal(err)
	}
	if _, err := packageDataDir(parent, "escape"); err == nil {
		t.Fatal("link para fora do diretório de dados aceito")
	}
	if _, err := packageDataDir(parent, "a:b"); err == nil {
		t.Fatal("separador de volume aceito")
	}

	stub := filepath.Join(base, "podman")
	program := "#!/bin/sh\n[ \"$1\" = --remote=false ] || exit 1\nshift\ncase $1 in\n" +
		"ok) printf '%9000s' a;;\nfalha) printf '\\033[1mtexto' >&2; exit 4;;\nesac\n"
	if err := os.WriteFile(stub, []byte(program), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", base)
	if out, err := packageOutput(context.Background(), "ok"); err != nil || len(out) != 8192 {
		t.Fatalf("saída não limitada: %d %v", len(out), err)
	}
	if _, err := packageOutput(context.Background(), "falha"); err == nil || !strings.Contains(err.Error(), "\\x1b") {
		t.Fatalf("saída sem escape: %v", err)
	}
}
