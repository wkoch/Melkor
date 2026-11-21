package main

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRedeSoEntregaBundleCompletoEVerificado(t *testing.T) {
	bundle := "bundle completo"
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(bundle)))
	plain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("um redirecionamento HTTPS abriu HTTP")
	}))
	defer plain.Close()
	tls := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ok":
			fmt.Fprint(w, bundle)
		case "/redirect":
			http.Redirect(w, r, "/ok", http.StatusFound)
		case "/http":
			http.Redirect(w, r, plain.URL, http.StatusFound)
		case "/truncado":
			w.Header().Set("Content-Length", "200")
			fmt.Fprint(w, bundle)
		case "/grande":
			w.Header().Set("Content-Length", fmt.Sprint(maxBundleBytes+1))
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer tls.Close()
	staging := t.TempDir()
	if err := os.Chmod(staging, 0o700); err != nil {
		t.Fatal(err)
	}
	installFailure := errors.New("RAUC rejeitou assinatura")
	for _, test := range []struct {
		path, digest string
		tls          bool
		install      bool
		result       error
	}{
		{"/ok", digest, true, true, nil},
		{"/redirect", digest, true, true, nil},
		{"/ok", digest, true, true, installFailure},
		{"/ok", digest, false, false, nil},
		{"/ok", strings.Repeat("0", 64), true, false, nil},
		{"/http", digest, true, false, nil},
		{"/truncado", digest, true, false, nil},
		{"/grande", digest, true, false, nil},
		{"/ausente", digest, true, false, nil},
	} {
		var transport http.RoundTripper
		if test.tls {
			transport = tls.Client().Transport
		}
		called := false
		err := run(context.Background(), []string{tls.URL + test.path, test.digest}, os.Getuid(), staging, transport,
			func(_ context.Context, file string) error {
				called = true
				contents, readErr := os.ReadFile(file)
				dir, statErr := os.Stat(filepath.Dir(file))
				if readErr != nil || string(contents) != bundle || statErr != nil || dir.Mode().Perm() != 0o700 {
					t.Fatalf("bundle ou permissões divergentes: %v, %v", readErr, statErr)
				}
				return test.result
			})
		if called != test.install || (test.install && !errors.Is(err, test.result)) || (!test.install && err == nil) {
			t.Fatalf("%s: RAUC chamado=%v, erro=%v", test.path, called, err)
		}
		if entries, err := os.ReadDir(staging); err != nil || len(entries) != 0 {
			t.Fatalf("bundle temporário permaneceu após %s: %v", test.path, err)
		}
	}
}

func TestRecusaParametrosEAreaDeGravacaoInsegura(t *testing.T) {
	dir := t.TempDir()
	link := filepath.Join(dir, "link")
	if err := os.Symlink(dir, link); err != nil {
		t.Fatal(err)
	}
	install := func(context.Context, string) error { t.Fatal("RAUC chamado"); return nil }
	args := []string{"https://example.invalid/test.raucb", strings.Repeat("a", 64)}
	for _, test := range []struct {
		args []string
		path string
	}{
		{nil, dir}, {[]string{args[0], "sha-incorreto"}, dir},
		{args, link}, {args, filepath.Join(dir, "ausente")},
	} {
		if err := run(context.Background(), test.args, os.Getuid(), test.path, nil, install); err == nil {
			t.Fatal("atualização inválida aceita")
		}
	}
}
