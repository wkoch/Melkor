package main

import (
	"context"
	"flag"
	"io"
	"strings"
	"testing"
)

func TestAjudaEValidacao(t *testing.T) {
	ctx := context.Background()
	if err := run(ctx, []string{"-help"}, io.Discard); err != flag.ErrHelp {
		t.Fatalf("ajuda: %v", err)
	}
	if err := run(ctx, nil, io.Discard); err == nil || !strings.Contains(err.Error(), "-saida") {
		t.Fatalf("opções ausentes: %v", err)
	}
	args := []string{"-saida", "x.img", "-core", "x", "-sha256", "x", "-efi", "x", "-grub", "x"}
	if err := run(ctx, append(args, "-gib", "18446744073709551615"), io.Discard); err == nil || !strings.Contains(err.Error(), "limite") {
		t.Fatalf("capacidade fora do limite: %v", err)
	}
	if err := run(ctx, args, io.Discard); err == nil || !strings.Contains(err.Error(), "SHA-256") {
		t.Fatalf("hash inválido: %v", err)
	}
}
