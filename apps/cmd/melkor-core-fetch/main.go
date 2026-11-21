package main

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"melkor.local/apps/internal/httpsfetch"
)

const maxBundleBytes = (2 << 30) + (64 << 20)

func run(ctx context.Context, args []string, uid int, staging string, transport http.RoundTripper,
	install func(context.Context, string) error) error {
	if len(args) != 2 {
		return errors.New("uso: melkor-core-update --https URL SHA256")
	}
	digest := strings.ToLower(args[1])
	if decoded, err := hex.DecodeString(digest); err != nil || len(decoded) != 32 {
		return errors.New("informe um SHA-256 de 64 dígitos hexadecimais")
	}
	info, err := os.Lstat(staging)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode().Perm() != 0o700 || int(info.Sys().(*syscall.Stat_t).Uid) != uid {
		return errors.New("a área de atualização deve pertencer a root e ter modo 0700")
	}
	work, err := os.MkdirTemp(staging, ".download-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	bundle := filepath.Join(work, "update.raucb")
	if err := httpsfetch.File(ctx, transport, args[0], digest, bundle, maxBundleBytes); err != nil {
		return err
	}
	return install(ctx, bundle)
}

func main() {
	if os.Geteuid() != 0 {
		fmt.Fprintln(os.Stderr, "Entre em uma sessão root para atualizar o core.")
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DisableCompression = true
	defer transport.CloseIdleConnections()
	install := func(ctx context.Context, bundle string) error {
		command := exec.CommandContext(ctx, "/usr/bin/rauc", "install", bundle)
		command.Stdout, command.Stderr = os.Stdout, os.Stderr
		return command.Run()
	}
	if err := run(ctx, os.Args[1:], os.Geteuid(), "/persist/rauc", transport, install); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
