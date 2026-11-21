package pcdisk

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func imageFixture(t *testing.T) ImageInput {
	t.Helper()
	work := t.TempDir()
	bin := filepath.Join(work, "bin")
	if err := os.Mkdir(bin, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"sfdisk", "mke2fs", "grub-editenv", "mkfs.vfat", "mcopy", "fakeroot"} {
		text := "#!/bin/sh\n[ \"${TEST_DISK_FAIL:-}\" != \"${0##*/}\" ]\n"
		if err := os.WriteFile(filepath.Join(bin, name), []byte(text), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	core := []byte("hsqsconteudo-verificado")
	input := ImageInput{
		Destination: filepath.Join(work, "disk.img"), SizeBytes: MinimumDisk,
		Core: filepath.Join(work, "core.squashfs"), CoreSHA256: fmt.Sprintf("%x", sha256.Sum256(core)),
		EFI: filepath.Join(work, "bootx64.efi"), GRUB: filepath.Join(work, "grub.cfg"),
	}
	for file, data := range map[string][]byte{input.Core: core, input.EFI: []byte("EFI"), input.GRUB: []byte("@CORE_A@ @CORE_B@\n")} {
		if err := os.WriteFile(file, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return input
}

func TestDiscoRejeitaCredenciaisInvalidasAntesDePublicar(t *testing.T) {
	input := imageFixture(t)
	input.Credentials = filepath.Join(filepath.Dir(input.Destination), "credenciais")
	if err := os.WriteFile(input.Credentials, []byte("root:senha\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateImage(context.Background(), input); err == nil || !strings.Contains(err.Error(), "dois hashes") {
		t.Fatalf("erro inesperado: %v", err)
	}
	if _, err := os.Stat(input.Destination); !os.IsNotExist(err) {
		t.Fatal("imagem publicada com credenciais inválidas")
	}

	if err := os.WriteFile(input.Credentials, []byte(rawCredentials(testCredentials)), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TEST_DISK_FAIL", "fakeroot")
	if _, err := CreateImage(context.Background(), input); err == nil || !strings.Contains(err.Error(), "fakeroot") {
		t.Fatalf("o gerador ignorou a falha de provisionamento: %v", err)
	}
	if _, err := os.Stat(input.Destination); !os.IsNotExist(err) {
		t.Fatal("imagem publicada sem a provisão de credenciais")
	}
}

func TestGravaCoreNosDoisSlotsEPublicaDisco(t *testing.T) {
	input := imageFixture(t)
	layout, err := CreateImage(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	disk, err := os.Open(input.Destination)
	if err != nil {
		t.Fatal(err)
	}
	defer disk.Close()
	core, err := os.ReadFile(input.Core)
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range layout.Partitions[1:3] {
		actual := make([]byte, len(core))
		if _, err := disk.ReadAt(actual, int64(part.Start*layout.SectorSize)); err != nil || string(actual) != string(core) {
			t.Fatalf("conteúdo de %s: %q, %v", part.Name, actual, err)
		}
	}
}

func TestFalhaMantemDestinoEApagaTemporarios(t *testing.T) {
	for _, test := range []struct{ edit, expected string }{
		{"hash", "SHA-256"}, {"conteudo", "difere"}, {"grub", "exatamente uma"},
		{"mkfs.vfat", "mkfs.vfat"}, {"sfdisk", "sfdisk"}, {"mke2fs", "mke2fs"},
		{"destino", "file exists"},
	} {
		t.Run(test.edit, func(t *testing.T) {
			input := imageFixture(t)
			switch test.edit {
			case "hash":
				input.CoreSHA256 = "invalido"
			case "conteudo":
				input.CoreSHA256 = strings.Repeat("0", 64)
			case "grub":
				if err := os.WriteFile(input.GRUB, []byte("@CORE_A@"), 0o600); err != nil {
					t.Fatal(err)
				}
			case "destino":
				if err := os.WriteFile(input.Destination, []byte("preservar"), 0o600); err != nil {
					t.Fatal(err)
				}
			default:
				t.Setenv("TEST_DISK_FAIL", test.edit)
			}
			if _, err := CreateImage(context.Background(), input); err == nil || !strings.Contains(err.Error(), test.expected) {
				t.Fatalf("erro: %v", err)
			}
			if folders, err := filepath.Glob(filepath.Join(filepath.Dir(input.Destination), ".melkor-disco-*")); err != nil || len(folders) > 0 {
				t.Fatalf("temporários restantes: %v, %v", folders, err)
			}
			data, err := os.ReadFile(input.Destination)
			if test.edit == "destino" {
				if err != nil || string(data) != "preservar" {
					t.Fatalf("destino alterado: %q, %v", data, err)
				}
			} else if !os.IsNotExist(err) {
				t.Fatalf("destino criado em caso de falha: %v", err)
			}
		})
	}
}
