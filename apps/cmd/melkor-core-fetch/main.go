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
	"time"

	"melkor.local/apps/internal/httpsfetch"
)

const maxBundleBytes = (2 << 30) + (64 << 20)

func bundleDigest(value string) (string, error) {
	digest := strings.ToLower(value)
	if decoded, err := hex.DecodeString(digest); err != nil || len(decoded) != 32 {
		return "", errors.New("informe um SHA-256 de 64 dígitos hexadecimais")
	}
	return digest, nil
}

func checkStaging(staging string, uid int) error {
	info, err := os.Lstat(staging)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode().Perm() != 0o700 || int(info.Sys().(*syscall.Stat_t).Uid) != uid {
		return errors.New("a área de atualização deve pertencer a root e ter modo 0700")
	}
	return nil
}

func run(ctx context.Context, args []string, uid int, staging string, transport http.RoundTripper,
	install func(context.Context, string) error) error {
	if len(args) != 2 {
		return errors.New("uso: melkor-core-update --https URL SHA256")
	}
	digest, err := bundleDigest(args[1])
	if err != nil {
		return err
	}
	if err := checkStaging(staging, uid); err != nil {
		return err
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
	if err := dispatch(ctx, os.Args[1:], "/etc/melkor-update", "/persist/rauc", os.Geteuid(), transport, install); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func dispatch(ctx context.Context, args []string, config, staging string, uid int, transport http.RoundTripper,
	install func(context.Context, string) error) error {
	if err := checkStaging(staging, uid); err != nil {
		return err
	}
	switch {
	case len(args) == 0:
		message, err := fromChannel(ctx, config, staging, transport, time.Now(), func(url, digest string) error {
			return run(ctx, []string{url, digest}, uid, staging, transport, install)
		})
		if err == nil {
			fmt.Println(message)
		}
		return err
	case len(args) == 2 && args[0] == "--channel" && args[1] != "":
		return setChannel(staging, args[1])
	case len(args) == 1 && args[0] == "--reset-channel":
		return setChannel(staging, "")
	default:
		return run(ctx, args, uid, staging, transport, install)
	}
}
