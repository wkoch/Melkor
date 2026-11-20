package main

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDownloadValidaTransporteConteudoETamanho(t *testing.T) {
	cleartext := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("a fonte HTTP não pode ser consultada")
	}))
	defer cleartext.Close()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/source":
			fmt.Fprint(w, "melkor")
		case "/stream":
			fmt.Fprint(w, "mel")
			w.(http.Flusher).Flush()
			fmt.Fprint(w, "kor")
		case "/ok":
			http.Redirect(w, r, "/source", http.StatusFound)
		case "/http":
			http.Redirect(w, r, cleartext.URL, http.StatusFound)
		case "/loop":
			http.Redirect(w, r, "/loop", http.StatusFound)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte("melkor")))
	for _, tc := range []struct {
		name, source, hash, wantError string
		limit                         int64
	}{
		{"íntegro", server.URL + "/source", hash, "", 6},
		{"redirecionamento HTTPS", server.URL + "/ok", hash, "", 6},
		{"hash divergente", server.URL + "/source", strings.Repeat("0", 64), "SHA-256", 6},
		{"limite declarado", server.URL + "/source", hash, "limite", 5},
		{"limite em stream", server.URL + "/stream", hash, "limite", 5},
		{"erro HTTP", server.URL + "/missing", hash, "HTTP 404", 6},
		{"downgrade", server.URL + "/http", hash, "redirecionamento", 6},
		{"loop", server.URL + "/loop", hash, "redirecionamento", 6},
		{"HTTP direto", cleartext.URL, hash, "HTTPS", 6},
		{"credenciais", strings.Replace(server.URL, "://", "://token@", 1), hash, "HTTPS sem credenciais", 6},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dest := filepath.Join(t.TempDir(), "source")
			err := download(server.Client().Transport, tc.source, tc.hash, dest, tc.limit)
			if tc.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantError) {
					t.Fatalf("esperava %q, obtive %v", tc.wantError, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if data, err := os.ReadFile(dest); err != nil || string(data) != "melkor" {
				t.Fatalf("fonte inesperada: %q (%v)", data, err)
			}
		})
	}
}

func TestBuildIsolaFasesERejeitaCandidatos(t *testing.T) {
	dir := t.TempDir()
	static := filepath.Join(dir, "fixture")
	source := filepath.Join(dir, "fixture.go")
	if err := os.WriteFile(source, []byte("package main\nfunc main() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "build", "-o", static, source)
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS=linux", "GOARCH=amd64")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build da fixture: %v\n%s", err, out)
	}
	stub := `#!/bin/sh
set -eu
for arg do
    case "$arg" in
        --entrypoint=*) entry=${arg#*=} ;;
        --network=*) network=${arg#*=} ;;
        --volume=*) work=${arg#*=}; work=${work%:/work:Z} ;;
        --read-only) readonly=yes ;;
        --userns=keep-id) uid=yes ;;
        --security-opt=no-new-privileges) privs=no ;;
    esac
done
test "$readonly/$uid/$privs" = yes/yes/no
printf '%s %s\n' "$entry" "$network" >> "$TEST_PODMAN_LOG"
test "$entry" != "$TEST_PODMAN_FAIL"
case "$entry" in
    tar) test "$(cat "$work/source.tar.gz")" = source ;;
    fetch) ;;
    compile) cp "$TEST_BINARY" "$work/src/nu" ;;
    /work/src/nu) printf '%s\n' "$TEST_CHECK_OUTPUT" ;;
    *) exit 1 ;;
esac
`
	if err := os.WriteFile(filepath.Join(dir, "podman"), []byte(stub), 0o700); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "source")
	}))
	defer server.Close()
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	app := recipe{
		Source: server.URL, SHA256: fmt.Sprintf("%x", sha256.Sum256([]byte("source"))),
		Fetch: []string{"fetch"}, Build: []string{"compile"}, Files: map[string]string{"nu": "nu"},
		Checks: []check{{Run: []string{"nu"}, Stdout: "6"}},
	}
	for _, tc := range []struct {
		name, fail, output, binary, errorText, phases string
	}{
		{"candidata válida", "", "6", static, "", "tar none\nfetch private\ncompile none\n/work/src/nu none\n"},
		{"extração falha", "tar", "6", static, "tar:", "tar none\n"},
		{"dependência falha", "fetch", "6", static, "obter dependências", "tar none\nfetch private\n"},
		{"build falha", "compile", "6", static, "build:", "tar none\nfetch private\ncompile none\n"},
		{"teste falha", "/work/src/nu", "6", static, "verificação", "tar none\nfetch private\ncompile none\n/work/src/nu none\n"},
		{"resultado diverge", "", "7", static, "esperado", "tar none\nfetch private\ncompile none\n/work/src/nu none\n"},
		{"binário dinâmico", "", "6", "/bin/sh", "carregador dinâmico", "tar none\nfetch private\ncompile none\n/work/src/nu none\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := t.TempDir()
			log := filepath.Join(t.TempDir(), "phases")
			t.Setenv("TEST_PODMAN_LOG", log)
			t.Setenv("TEST_PODMAN_FAIL", tc.fail)
			t.Setenv("TEST_BINARY", tc.binary)
			t.Setenv("TEST_CHECK_OUTPUT", tc.output)
			err := buildRecipe("nushell", app, out, server.Client().Transport)
			entries, listErr := os.ReadDir(out)
			if listErr != nil {
				t.Fatal(listErr)
			}
			if tc.errorText == "" {
				if err != nil || len(entries) != 1 || entries[0].Name() != "nu" {
					t.Fatalf("candidata não preparada: %v (%v)", entries, err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.errorText) || len(entries) != 0 {
				t.Fatalf("esperava rejeição por %q sem artefatos, obtive %v (%v)", tc.errorText, entries, err)
			}
			if data, err := os.ReadFile(log); err != nil || string(data) != tc.phases {
				t.Fatalf("fases incorretas: %q (%v)", data, err)
			}
			if files, err := os.ReadDir(filepath.Join(os.Getenv("XDG_CACHE_HOME"), "melkor")); err != nil || len(files) != 0 {
				t.Fatalf("diretório temporário vazou: %v (%v)", files, err)
			}
		})
	}
}

func TestStageExecutableRejeitaLinkEArquivoNaoExecutavel(t *testing.T) {
	dir := t.TempDir()
	src, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	if err := os.WriteFile(filepath.Join(dir, "arquivo"), []byte("texto"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/bin/sh", filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "script"), []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "nu")
	for _, name := range []string{"arquivo", "script", "link", "../escape", "inexistente"} {
		if err := stageExecutable(src, name, dest); err == nil {
			t.Errorf("artefato inválido aceito: %q", name)
		}
	}
}
