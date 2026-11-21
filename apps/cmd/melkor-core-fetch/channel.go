package main

import (
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"melkor.local/apps/internal/httpsfetch"
)

const (
	channelLimit = 16 << 10
	pcTarget     = "melkor-pc-x86_64"
)

type release struct {
	Schema   int       `json:"schema"`
	Target   string    `json:"target"`
	Revision uint32    `json:"revision"`
	Profile  string    `json:"profile,omitempty"`
	Expires  time.Time `json:"expires"`
	URL      string    `json:"url"`
	SHA256   string    `json:"sha256"`
}

func channelURL(source string) error {
	addr, err := url.Parse(source)
	if err != nil || !httpsfetch.TrustedSource(addr) {
		return errors.New("informe uma URL HTTPS sem credenciais")
	}
	return nil
}

func setChannel(staging, source string) error {
	dest := filepath.Join(staging, "channel")
	if source == "" {
		if err := os.Remove(dest); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	if err := channelURL(source); err != nil {
		return err
	}
	out, err := os.CreateTemp(staging, ".channel-")
	if err != nil {
		return err
	}
	defer os.Remove(out.Name())
	_, err = fmt.Fprintln(out, source)
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(out.Name(), dest)
}

func verifyRelease(document, publicPEM []byte, now time.Time) (release, error) {
	var entry release
	block, _ := pem.Decode(publicPEM)
	if block == nil || block.Type != "PUBLIC KEY" {
		return entry, errors.New("chave pública do canal inválida")
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return entry, err
	}
	public, ok := parsed.(ed25519.PublicKey)
	if !ok {
		return entry, errors.New("o canal exige chave Ed25519")
	}
	var envelope struct {
		Payload   []byte `json:"payload"`
		Signature []byte `json:"signature"`
	}
	if err := json.Unmarshal(document, &envelope); err != nil {
		return entry, err
	}
	if !ed25519.Verify(public, envelope.Payload, envelope.Signature) {
		return entry, errors.New("assinatura do canal inválida")
	}
	if err := json.Unmarshal(envelope.Payload, &entry); err != nil {
		return entry, err
	}
	if entry.Schema != 1 || entry.Target != pcTarget || entry.Revision == 0 {
		return entry, errors.New("manifesto incompatível com o PC")
	}
	if !entry.Expires.After(now) {
		return entry, errors.New("manifesto do canal expirado")
	}
	if err := channelURL(entry.URL); err != nil {
		return entry, err
	}
	if _, err := bundleDigest(entry.SHA256); err != nil {
		return entry, err
	}
	return entry, nil
}

func fromChannel(ctx context.Context, config, staging, profile string, transport http.RoundTripper,
	now time.Time, install func(string, string) error) (string, error) {
	public, err := os.ReadFile(filepath.Join(config, "channel.pem"))
	if errors.Is(err, os.ErrNotExist) {
		return "", errors.New("esta imagem ainda não contém a chave pública de atualização do canal")
	}
	if err != nil {
		return "", err
	}
	rawRevision, err := os.ReadFile(filepath.Join(config, "revision"))
	if err != nil {
		return "", err
	}
	current, err := strconv.ParseUint(strings.TrimSpace(string(rawRevision)), 10, 32)
	if err != nil {
		return "", fmt.Errorf("revisão instalada inválida: %w", err)
	}
	var source []byte
	if profile == "desktop" {
		source, err = os.ReadFile(filepath.Join(config, "desktop"))
	} else {
		source, err = os.ReadFile(filepath.Join(staging, "channel"))
		if errors.Is(err, os.ErrNotExist) {
			source, err = os.ReadFile(filepath.Join(config, "channel"))
		}
	}
	if err != nil {
		return "", err
	}
	document, err := httpsfetch.Small(ctx, transport, strings.TrimSpace(string(source)), channelLimit)
	if err != nil {
		return "", err
	}
	entry, err := verifyRelease(document, public, now)
	if err != nil {
		return "", err
	}
	if entry.Profile != profile {
		return "", errors.New("manifesto incompatível com o perfil solicitado")
	}
	if uint64(entry.Revision) < current {
		return "", fmt.Errorf("o canal anunciou a revisão anterior %d (instalada: %d)", entry.Revision, current)
	}
	if uint64(entry.Revision) == current {
		return "O Melkor já está atualizado.", nil
	}
	if err := install(entry.URL, entry.SHA256); err != nil {
		return "", err
	}
	return fmt.Sprintf("Revisão %d instalada. Reinicie para ativá-la.", entry.Revision), nil
}
