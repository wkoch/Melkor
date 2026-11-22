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

func open(ctx context.Context, transport http.RoundTripper, source string, limit int64) (io.ReadCloser, error) {
	addr, err := url.Parse(source)
	if err != nil || !TrustedSource(addr) {
		return nil, fmt.Errorf("fonte inválida: informe HTTPS sem credenciais")
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
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("baixar %s: HTTP %d", source, resp.StatusCode)
	}
	if resp.ContentLength > limit {
		resp.Body.Close()
		return nil, fmt.Errorf("fonte excede o limite de %d bytes", limit)
	}
	return resp.Body, nil
}

func Small(ctx context.Context, transport http.RoundTripper, source string, limit int64) ([]byte, error) {
	body, err := open(ctx, transport, source, limit)
	if err != nil {
		return nil, err
	}
	defer body.Close()
	data, err := io.ReadAll(io.LimitReader(body, limit+1))
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("fonte excede o limite de %d bytes", limit)
	}
	return data, err
}

func File(ctx context.Context, transport http.RoundTripper, source, digest, file string, limit int64) error {
	body, err := open(ctx, transport, source, limit)
	if err != nil {
		return err
	}
	defer body.Close()
	out, err := os.Create(file)
	if err != nil {
		return err
	}
	hash := sha256.New()
	written, err := io.Copy(io.MultiWriter(out, hash), io.LimitReader(body, limit+1))
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
