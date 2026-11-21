package httpsfetch

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFile(t *testing.T) {
	const payload = "bundle"
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ok":
			io.WriteString(w, payload)
		case "/redirect":
			http.Redirect(w, r, "/ok", http.StatusFound)
		case "/loop":
			http.Redirect(w, r, "/loop", http.StatusFound)
		case "/down":
			http.Redirect(w, r, "http://example.invalid", http.StatusFound)
		case "/oversize":
			w.(http.Flusher).Flush()
			io.WriteString(w, strings.Repeat("x", 20))
		case "/incomplete":
			w.Header().Set("Content-Length", "12")
			io.WriteString(w, payload)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(payload)))
	for _, test := range []struct {
		name, source, digest, wantError string
		limit                           int64
	}{
		{"success", server.URL + "/redirect", digest, "", 16},
		{"wrong hash", server.URL + "/ok", "bad", "SHA-256", 16},
		{"content length", server.URL + "/ok", digest, "excede", 1},
		{"stream limit", server.URL + "/oversize", digest, "excede", 16},
		{"truncated", server.URL + "/incomplete", digest, "unexpected EOF", 16},
		{"status", server.URL + "/missing", digest, "HTTP 404", 16},
		{"redirect loop", server.URL + "/loop", digest, "redirecionamento", 16},
		{"redirect downgrade", server.URL + "/down", digest, "redirecionamento", 16},
		{"credentials", strings.Replace(server.URL, "https://", "https://user@", 1), digest, "fonte inválida", 16},
		{"malformed", "://", digest, "fonte inválida", 16},
	} {
		t.Run(test.name, func(t *testing.T) {
			dest := filepath.Join(t.TempDir(), "update")
			err := File(context.Background(), server.Client().Transport, test.source, test.digest, dest, test.limit)
			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("erro = %v, esperado %q", err, test.wantError)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(dest)
			if err != nil || string(data) != payload {
				t.Fatalf("arquivo = %q, erro = %v", data, err)
			}
		})
	}

	t.Run("bad destination", func(t *testing.T) {
		dest := filepath.Join(t.TempDir(), "missing", "bundle")
		if err := File(context.Background(), server.Client().Transport, server.URL+"/ok", digest, dest, 16); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("erro = %v", err)
		}
	})

	t.Run("canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		dest := filepath.Join(t.TempDir(), "update")
		if err := File(ctx, server.Client().Transport, server.URL+"/ok", digest, dest, 16); !errors.Is(err, context.Canceled) {
			t.Fatalf("erro = %v", err)
		}
	})
}
