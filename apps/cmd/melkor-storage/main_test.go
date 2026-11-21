package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunPublicaLinksDoDiscoDaRaiz(t *testing.T) {
	work := t.TempDir()
	sys := filepath.Join(work, "sys")
	mounts := filepath.Join(work, "mountinfo")
	destination := filepath.Join(work, "melkor")
	for i, label := range []string{"esp", "core-a", "core-b", "persist"} {
		device := fmt.Sprintf("vda%d", i+1)
		dir := filepath.Join(sys, "vda", device)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		event := fmt.Sprintf("DEVTYPE=partition\nDEVNAME=%s\nPARTN=%d\nPARTNAME=%s\n", device, i+1, label)
		if err := os.WriteFile(filepath.Join(dir, "uevent"), []byte(event), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink("vda/vda2", filepath.Join(sys, "8:2")); err != nil {
		t.Fatal(err)
	}
	if err := run(mounts, sys, destination); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("mountinfo ausente: %v", err)
	}
	for _, mode := range []string{"rw", "ro"} {
		if err := os.WriteFile(mounts, []byte("9 1 8:2 / / "+mode+" - squashfs /dev/vda2 ro\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		err := run(mounts, sys, destination)
		if mode == "rw" && (err == nil || !strings.Contains(err.Error(), "somente leitura")) {
			t.Fatalf("uma raiz gravável deveria ser recusada: %v", err)
		}
		if mode == "ro" && err != nil {
			t.Fatal(err)
		}
	}
	for name, want := range map[string]string{"esp-device": "/dev/vda1", "core-a": "/dev/vda2", "core-b": "/dev/vda3", "data": "/dev/vda4"} {
		if got, err := os.Readlink(filepath.Join(destination, name)); err != nil || got != want {
			t.Fatalf("%s: destino %q, erro %v", name, got, err)
		}
	}
}
