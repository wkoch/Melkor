package pcdisk

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBackendLimine(t *testing.T) {
	image := imageFixture(t)
	dir := filepath.Dir(image.Destination)
	efi := filepath.Join(dir, "efi")
	if err := os.Mkdir(efi, 0o700); err != nil {
		t.Fatal(err)
	}
	b := BootBackend{ESP: dir, EFI: efi, Runtime: dir, Booted: "A"}
	kernel, err := ReadKernel(context.Background(), image.Core)
	if err != nil {
		t.Fatal(err)
	}
	write := func(name string, data []byte) {
		t.Helper()
		if err := os.WriteFile(name, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(b.kernel("A"), kernel)
	write(b.kernel("B"), kernel)
	check := func(want string, fail bool, args ...string) {
		t.Helper()
		var output bytes.Buffer
		err := b.Run(context.Background(), args, &output)
		if (err != nil) != fail || strings.TrimSpace(output.String()) != want {
			t.Fatalf("%v: saída %q, erro %v", args, output.String(), err)
		}
	}
	check("A", false, "get-primary")
	check("good", false, "get-state", "A")
	check("", false, "set-state", "A", "good")
	check("", true, "set-primary", "A")
	check("", true, "set-state", "A", "bad")
	check("", false, "set-state", "B", "bad")
	check("bad", false, "get-state", "B")
	check("", true, "set-state", "B", "good")
	check("", true, "set-state", "X", "good")
	check("", true, "set-state", "A")
	check("", true, "get-state", "A", "B")
	check("", true)
	t.Setenv("TEST_DISK_FAIL", "unsquashfs")
	check("", true, "set-primary", "B")
	check("A", false, "get-primary")
	check("bad", false, "get-state", "B")
	t.Setenv("TEST_DISK_FAIL", "")
	write(b.kernel("B")+".new", []byte("kernel de gravação interrompida"))
	check("", false, "set-primary", "B")
	if got, err := os.ReadFile(b.kernel("B")); err != nil || !bytes.Equal(got, kernel) {
		t.Fatalf("kernel após tentativa repetida: %d bytes, erro %v", len(got), err)
	}
	check("good", false, "get-state", "B")
	check("B", false, "get-primary")
	check("", true, "set-state", "B", "bad")
	if err := os.Remove(filepath.Join(efi, "LoaderEntryOneShot"+efiLoaderGUID)); err != nil {
		t.Fatal(err)
	}
	check("A", false, "get-primary")
	b.Booted = "B"
	check("", true, "set-state", "A", "bad")
	check("", false, "set-state", "B", "good")
	check("B", false, "get-primary")
	check("", false, "set-state", "A", "bad")
	check("", false, "set-state", "A", "bad")
	write(filepath.Join(efi, "LoaderEntryDefault"+efiLoaderGUID), []byte("inválido"))
	check("", true, "get-primary")
	check("", true, "set-primary", "A")
}

func TestSlotDoKernel(t *testing.T) {
	for input, want := range map[string]string{
		"ro rauc.slot=A": "A", "rauc.slot=B init=/sbin/dinit": "B",
		"rauc.slot=A rauc.slot=B": "", "rauc.slot=C": "", "ro": "",
	} {
		if got := BootedSlot(input); got != want {
			t.Fatalf("%q: %q != %q", input, got, want)
		}
	}
}
