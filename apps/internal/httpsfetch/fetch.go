package httpsfetch

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"
)

func TrustedSource(addr *url.URL) bool {
	return addr.Scheme == "https" && addr.Hostname() != "" && addr.User == nil
}

func File(ctx context.Context, transport http.RoundTripper, source, digest, file string, limit int64) error {
	addr, err := url.Parse(source)
	if err != nil || !TrustedSource(addr) {
		return fmt.Errorf("fonte inválida: informe HTTPS sem credenciais")
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   5 * time.Minute,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if !TrustedSource(req.URL) || len(via) >= 10 {
				return fmt.Errorf("redirecionamento de fonte inválido")
			}
			return nil
		},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, addr.String(), nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("baixar %s: HTTP %d", source, resp.StatusCode)
	}
	if resp.ContentLength > limit {
		return fmt.Errorf("fonte excede o limite de %d bytes", limit)
	}
	out, err := os.Create(file)
	if err != nil {
		return err
	}
	hash := sha256.New()
	written, err := io.Copy(io.MultiWriter(out, hash), io.LimitReader(resp.Body, limit+1))
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if written > limit {
		return fmt.Errorf("fonte excede o limite de %d bytes", limit)
	}
	if actual := fmt.Sprintf("%x", hash.Sum(nil)); actual != digest {
		return fmt.Errorf("SHA-256 de %s: obtido %s, esperado %s", source, actual, digest)
	}
	return nil
}
