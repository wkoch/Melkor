package pcdisk

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unsafe"
)

func diskFixture(t *testing.T, dev, sys, physical, name, alias string, overrides map[string]string) {
	t.Helper()
	data := map[string]string{
		"removable": "0", "ro": "0", "uevent": "DEVTYPE=disk\n", "size": "33554432",
		"queue/logical_block_size": "512", "diskseq": "7", "device/model": "NVMe de teste",
	}
	for key, value := range overrides {
		data[key] = value
	}
	dir := filepath.Join(physical, name)
	for key, value := range data {
		path := filepath.Join(dir, key)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dev, name), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, link := range [][2]string{
		{dir, filepath.Join(sys, name)},
		{"../../" + name, filepath.Join(dev, "disk/by-id", alias)},
	} {
		if err := os.Symlink(link[0], link[1]); err != nil {
			t.Fatal(err)
		}
	}
}

func TestInstaladorListaUmIDPorDiscoInternoInteiro(t *testing.T) {
	root := t.TempDir()
	dev, sys := filepath.Join(root, "dev"), filepath.Join(root, "sys")
	for _, dir := range []string{filepath.Join(dev, "disk/by-id"), sys} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	pci := filepath.Join(root, "devices/pci")
	diskFixture(t, dev, sys, pci, "nvme0n1", "nvme-A", nil)
	diskFixture(t, dev, sys, filepath.Join(pci, "usb2"), "sda", "usb-pendrive", nil)
	diskFixture(t, dev, sys, pci, "sdb", "ata-bloqueado", map[string]string{"ro": "1"})
	diskFixture(t, dev, sys, pci, "sdc", "ata-removivel", map[string]string{"removable": "1"})
	diskFixture(t, dev, sys, pci, "sdd", "ata-pequeno", map[string]string{"size": "2048"})
	diskFixture(t, dev, sys, pci, "sde", "ata-sem-sequencia", map[string]string{"diskseq": "0"})
	diskFixture(t, dev, sys, pci, "sdf", "ata-setor", map[string]string{"queue/logical_block_size": "1024"})
	diskFixture(t, dev, sys, pci, "sda1", "ata-particao", map[string]string{"uevent": "DEVTYPE=partition"})
	diskFixture(t, dev, sys, pci, "sds", "ata-4kn", map[string]string{"queue/logical_block_size": "4096"})
	if err := os.Symlink("../../nvme0n1", filepath.Join(dev, "disk/by-id", "nvme-B")); err != nil {
		t.Fatal(err)
	}
	disks, err := installDisks(dev, sys)
	if err != nil || len(disks) != 2 || disks[0].ID != "ata-4kn" || disks[0].Sector != 4096 ||
		disks[1].ID != "nvme-A" || disks[1].Bytes != 16*GiB || disks[1].sequence != 7 {
		t.Fatalf("discos=%+v, erro=%v", disks, err)
	}
	for _, name := range []string{"../../nvme0n1", "nvme-A\x1b", "nvme-Inexistente"} {
		if _, err := inspectDisk(dev, sys, name); err == nil {
			t.Fatalf("ID inseguro aceito: %q", name)
		}
	}
}

func createSparseDisk(t *testing.T, path string, contents map[int64]string) *os.File {
	t.Helper()
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_EXCL, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { file.Close() })
	if err := file.Truncate(8 * MiB); err != nil {
		t.Fatal(err)
	}
	for offset, value := range contents {
		if _, err := file.WriteAt([]byte(value), offset); err != nil {
			t.Fatal(err)
		}
	}
	return file
}

func TestGravaSomenteExtensoesEPublicaAsDuasTabelas(t *testing.T) {
	work := t.TempDir()
	source := createSparseDisk(t, filepath.Join(work, "source"), map[int64]string{
		512: "primaria", 3 * MiB: "dados", 8*MiB - 512: "secundaria",
	})
	destination := createSparseDisk(t, filepath.Join(work, "destination"), map[int64]string{
		0: "anterior", 3 * MiB: "velho", 8*MiB - 4096: "anterior",
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := writeDisk(ctx, destination, source, 8*MiB); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelamento: %v", err)
	}
	untouched := make([]byte, 8)
	if _, err := destination.ReadAt(untouched, 0); err != nil || string(untouched) != "anterior" {
		t.Fatal("cancelamento alterou o disco")
	}
	if err := writeDisk(context.Background(), destination, source, 8*MiB); err != nil {
		t.Fatal(err)
	}
	for offset, want := range map[int64]string{0: "\x00\x00", 512: "primaria", 3 * MiB: "dados", 8*MiB - 512: "secundaria", 8*MiB - 4096: "\x00\x00"} {
		buffer := make([]byte, len(want))
		if _, err := destination.ReadAt(buffer, offset); err != nil || string(buffer) != want {
			t.Fatalf("offset %d: %q, %v", offset, buffer, err)
		}
	}
}

type shortDisk struct{}

func (shortDisk) WriteAt([]byte, int64) (int, error) { return 0, nil }

func TestErroDuranteCopiaEEntradaInvalida(t *testing.T) {
	src := bytes.NewReader([]byte("dados"))
	ctx := context.Background()
	if err := copyRegions(ctx, shortDisk{}, src, []region{{0, 5}}); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("escrita parcial: %v", err)
	}
	if err := copyRegions(ctx, shortDisk{}, src, []region{{0, 6}}); !errors.Is(err, io.EOF) {
		t.Fatalf("leitura parcial: %v", err)
	}
	if _, err := openInstallDisk(Disk{ID: "../fora"}); err == nil {
		t.Fatal("permitiu abrir disco sem seleção válida")
	}
	if _, err := Install(ctx, Disk{}, ImageInput{}, t.TempDir()); err == nil || !strings.Contains(err.Error(), "credenciais") {
		t.Fatalf("pré-condição de instalação: %v", err)
	}
	file := createSparseDisk(t, filepath.Join(t.TempDir(), "regular"), nil)
	var size uint64
	if err := diskIoctl(file, blkGetSize64, unsafe.Pointer(&size)); err == nil {
		t.Fatal("ioctl aceitou arquivo regular")
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := sparseRegions(file, MiB, 7*MiB); err == nil {
		t.Fatal("busca aceitou arquivo fechado")
	}
}

func TestImagem4KnUsaLosetupEDesmontaDepoisDoSfdisk(t *testing.T) {
	input := imageFixture(t)
	bin := strings.SplitN(os.Getenv("PATH"), ":", 2)[0]
	log := filepath.Join(t.TempDir(), "loop")
	t.Setenv("TEST_LOOP_LOG", log)
	script := "#!/bin/sh\ncase \"$1\" in --find) printf '/dev/loop3\\n';; --detach) printf '%s\\n' \"$2\" > \"$TEST_LOOP_LOG\";; esac\n"
	if err := os.WriteFile(filepath.Join(bin, "losetup"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	layout, err := Plan(MinimumDisk, 4096, 1024)
	if err != nil || layout.Partitions[0].Size*layout.SectorSize != espSize4Kn {
		t.Fatalf("layout: %+v, %v", layout, err)
	}
	t.Setenv("TEST_DISK_FAIL", "sfdisk")
	_, err = createImage(context.Background(), input, 4096, false)
	if err == nil || !strings.Contains(err.Error(), "sfdisk") {
		t.Fatalf("resultado: %v", err)
	}
	detached, err := os.ReadFile(log)
	if err != nil || string(detached) != "/dev/loop3\n" {
		t.Fatalf("o loop temporário ficou ativo: %q, %v", detached, err)
	}
}
