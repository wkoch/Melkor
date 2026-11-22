package main

import (
	"context"
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCanalRecusaChavesPublicasInvalidas(t *testing.T) {
	wrong, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	wrongDER, err := x509.MarshalPKIXPublicKey(wrong.PublicKey())
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		key   []byte
		error string
	}{
		{[]byte("sem PEM"), "chave pública"},
		{pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: []byte{1, 2, 3}}), "asn1"},
		{pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: wrongDER}), "Ed25519"},
	} {
		if _, err := verifyRelease(nil, tc.key, time.Now()); err == nil || !strings.Contains(err.Error(), tc.error) {
			t.Fatalf("erro esperado %q, obtido %v", tc.error, err)
		}
	}
}

func TestChannel(t *testing.T) {
	pub, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	public := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})
	now := time.Unix(1700000000, 0)
	makeDocument := func(r release, tamper bool) []byte {
		payload, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		signature := ed25519.Sign(key, payload)
		if tamper {
			signature[0] ^= 1
		}
		encoded, err := json.Marshal(map[string][]byte{"payload": payload, "signature": signature})
		if err != nil {
			t.Fatal(err)
		}
		return encoded
	}
	r := release{Schema: 1, Target: pcTarget, Revision: 3, Expires: now.Add(time.Hour),
		URL: "https://example.invalid/update.raucb", SHA256: strings.Repeat("a", 64)}
	stable := makeDocument(r, false)
	tampered := makeDocument(r, true)
	r.Revision = 2
	current := makeDocument(r, false)
	r.Revision = 1
	older := makeDocument(r, false)
	r.Expires = now
	expired := makeDocument(r, false)
	r.Expires = now.Add(time.Hour)
	r.Target = "melkor-m1-x86_64"
	wrongTarget := makeDocument(r, false)
	r.Target = pcTarget
	r.URL = "http://example.invalid"
	insecure := makeDocument(r, false)
	r.URL = "https://example.invalid/update.raucb"
	r.SHA256 = "wrong"
	badDigest := makeDocument(r, false)

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		path := req.URL.Path
		switch path {
		case "/large":
			w.(http.Flusher).Flush()
			io.WriteString(w, strings.Repeat("x", channelLimit+10))
		default:
			body := map[string][]byte{"/stable": stable, "/current": current, "/older": older, "/tampered": tampered,
				"/expired": expired, "/target": wrongTarget, "/insecure": insecure, "/digest": badDigest,
				"/malformed": []byte("{")}[path]
			w.Write(body)
		}
	}))
	defer server.Close()
	config, staging := t.TempDir(), t.TempDir()
	if err := os.Chmod(staging, 0o700); err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string][]byte{"channel": []byte(server.URL + "/stable\n"), "revision": []byte("2\n"), "channel.pem": public} {
		if err := os.WriteFile(filepath.Join(config, name), value, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	ctx := context.Background()
	client := server.Client().Transport
	for _, test := range []struct {
		path, want, wantError string
		call                  bool
	}{
		{"", "Revisão 3 instalada.", "", true},
		{"/current", "já está atualizado", "", false},
		{"/older", "", "revisão anterior", false},
		{"/tampered", "", "assinatura", false},
		{"/expired", "", "expirado", false},
		{"/target", "", "incompatível", false},
		{"/insecure", "", "HTTPS", false},
		{"/digest", "", "SHA-256", false},
		{"/large", "", "excede", false},
		{"/malformed", "", "unexpected end", false},
		{"", "Revisão 3 instalada.", "", true},
	} {
		t.Run(test.path, func(t *testing.T) {
			source := ""
			if test.path != "" {
				source = server.URL + test.path
			}
			if err := setChannel(staging, source); err != nil {
				t.Fatal(err)
			}
			called := false
			message, err := fromChannel(ctx, config, staging, "", client, now, func(url, digest string) error {
				called = true
				if !strings.HasPrefix(url, "https://") || strings.Repeat("a", 64) != digest {
					t.Fatal(url, digest)
				}
				return nil
			})
			if called != test.call {
				t.Fatalf("instalação chamada = %v, erro = %v", called, err)
			}
			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("erro = %v, esperado %q", err, test.wantError)
				}
			} else if err != nil || !strings.Contains(message, test.want) {
				t.Fatalf("mensagem = %q, erro = %v", message, err)
			}
		})
	}
	// O canal adicional usa o endereço de fábrica e exige finalidade assinada.
	bundle := []byte("bundle de desktop")
	desktop := httptest.NewUnstartedServer(nil)
	defer desktop.Close()
	r.Profile = "desktop"
	r.Revision = 3
	r.Expires = time.Now().Add(time.Hour)
	r.URL = "https://" + desktop.Listener.Addr().String() + "/bundle"
	r.SHA256 = fmt.Sprintf("%x", sha256.Sum256(bundle))
	desktopRelease := makeDocument(r, false)
	desktop.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/manifesto":
			w.Write(desktopRelease)
		case "/base":
			w.Write(stable)
		case "/bundle":
			w.Write(bundle)
		default:
			http.NotFound(w, req)
		}
	})
	desktop.StartTLS()
	for name, value := range map[string]string{
		filepath.Join(config, "desktop"):  desktop.URL + "/manifesto",
		filepath.Join(staging, "channel"): server.URL + "/stable",
	} {
		if err := os.WriteFile(name, []byte(value+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	signedBundleInstalled := false
	if err := dispatch(ctx, []string{"--desktop"}, config, staging, os.Getuid(), desktop.Client().Transport,
		func(_ context.Context, filename string) error {
			data, err := os.ReadFile(filename)
			if err != nil || string(data) != string(bundle) {
				t.Fatalf("bundle = %q, erro = %v", data, err)
			}
			signedBundleInstalled = true
			return nil
		}); err != nil || !signedBundleInstalled {
		t.Fatalf("instalação = %v, erro = %v", signedBundleInstalled, err)
	}
	if err := os.WriteFile(filepath.Join(config, "desktop"), []byte(desktop.URL+"/base"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := fromChannel(ctx, config, staging, "desktop", desktop.Client().Transport, now, func(string, string) error {
		t.Fatal("bundle do canal da base não pode passar pelo canal desktop")
		return nil
	}); err == nil || !strings.Contains(err.Error(), "perfil solicitado") {
		t.Fatalf("esperava erro de perfil: %v", err)
	}
	if err := setChannel(staging, desktop.URL+"/manifesto"); err != nil {
		t.Fatal(err)
	}
	if _, err := fromChannel(ctx, config, staging, "", desktop.Client().Transport, now, func(string, string) error {
		t.Fatal("bundle do desktop não pode passar pelo canal da base")
		return nil
	}); err == nil || !strings.Contains(err.Error(), "perfil solicitado") {
		t.Fatalf("esperava erro de perfil: %v", err)
	}
	if err := setChannel(staging, ""); err != nil {
		t.Fatal(err)
	}
	fail := func(string, string) error { return fmt.Errorf("recusa do RAUC") }
	if _, err := fromChannel(ctx, config, staging, "", client, now, fail); err == nil || err.Error() != "recusa do RAUC" {
		t.Fatalf("recusa perdida: %v", err)
	}
	if err := os.Remove(filepath.Join(config, "channel.pem")); err != nil {
		t.Fatal(err)
	}
	if _, err := fromChannel(ctx, config, staging, "", client, now, fail); err == nil || !strings.Contains(err.Error(), "chave pública") {
		t.Fatalf("imagem sem chave: %v", err)
	}
	native := func(context.Context, string) error { t.Fatal("instalação inesperada"); return nil }
	for _, args := range [][]string{{"--channel", "https://example.invalid/channel"}, {"--reset-channel"}, {}} {
		err := dispatch(ctx, args, config, staging, os.Getuid(), client, native)
		if len(args) > 0 && err != nil || len(args) == 0 && err == nil {
			t.Fatalf("%v: %v", args, err)
		}
	}
	if err := dispatch(ctx, []string{"--channel", "http://example.invalid"}, config, staging, os.Getuid(), client, native); err == nil {
		t.Fatal("canal HTTP aceito")
	}
}
