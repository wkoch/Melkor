package pcdisk

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRaizDeterminaODiscoDaInstalacao(t *testing.T) {
	for _, test := range []struct{ disk, root string }{{"vdb", "vdb2"}, {"nvme0n1", "nvme0n1p3"}} {
		t.Run(test.disk, func(t *testing.T) {
			sys := t.TempDir()
			layout := fakeDisk(t, sys, test.disk)
			if err := os.Symlink(filepath.Join(test.disk, test.root), filepath.Join(sys, "8:2")); err != nil {
				t.Fatal(err)
			}
			fakeDisk(t, sys, "vdc")
			mounts := strings.NewReader("7 1 8:3 / / ro,relatime - tmpfs none rw\n9 1 8:2 / / ro - squashfs /dev/test ro\n")
			actual, err := ResolveMounted(mounts, sys)
			if err != nil || actual != layout {
				t.Fatalf("disco resolvido: %+v, %v", actual, err)
			}
		})
	}
}

func TestPublicaLinksSemTrocarDiretorioOcupado(t *testing.T) {
	work := t.TempDir()
	layout := fakeDisk(t, work, "vda")
	destination := filepath.Join(work, "melkor")
	if err := layout.StageLinks(destination); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"esp-device": layout.ESP, "core-a": layout.CoreA, "core-b": layout.CoreB, "data": layout.Data} {
		if result, err := os.Readlink(filepath.Join(destination, name)); err != nil || result != want {
			t.Fatalf("link de %s: %q, %v", name, result, err)
		}
	}
	if err := fakeDisk(t, work, "vdb").StageLinks(destination); err == nil {
		t.Fatal("uma segunda instalação substituiu os links em uso")
	}
	if result, err := os.Readlink(filepath.Join(destination, "data")); err != nil || result != layout.Data {
		t.Fatalf("volume de dados substituído: %q, %v", result, err)
	}
}

func TestRecusaRaizErradaOuLayoutIncompleto(t *testing.T) {
	const mounts = "9 1 8:2 / / ro - squashfs /dev/vda2 ro\n"
	for _, test := range []struct{ mode, message string }{{"rw", "somente leitura"}, {"ausente", "dispositivo da raiz"}, {"esp", "slot do core"}, {"dados", "persist ausente"}, {"rotulo", "partição inesperada"}} {
		t.Run(test.mode, func(t *testing.T) {
			sys := t.TempDir()
			fakeDisk(t, sys, "vda")
			root := "vda/vda2"
			text := mounts
			switch test.mode {
			case "rw":
				text = strings.ReplaceAll(text, " ro", " rw")
			case "ausente":
				root = "vdb/vdb2"
			case "esp":
				root = "vda/vda1"
			case "dados":
				if err := os.RemoveAll(filepath.Join(sys, "vda/vda4")); err != nil {
					t.Fatal(err)
				}
			case "rotulo":
				if err := os.WriteFile(filepath.Join(sys, "vda/vda4/uevent"), []byte("DEVTYPE=partition\nPARTN=4\nPARTNAME=foreign\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Symlink(root, filepath.Join(sys, "8:2")); err != nil {
				t.Fatal(err)
			}
			if _, err := ResolveMounted(strings.NewReader(text), sys); err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("erro de validação: %v", err)
			}
		})
	}
}

func fakeDisk(t *testing.T, sys, disk string) MountedStorage {
	t.Helper()
	var nodes [4]string
	for i, name := range []string{"esp", "core-a", "core-b", "persist"} {
		prefix := disk
		if strings.HasPrefix(disk, "nvme") {
			prefix += "p"
		}
		device := fmt.Sprintf("%s%d", prefix, i+1)
		path := filepath.Join(sys, disk, device)
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
		event := fmt.Sprintf("DEVTYPE=partition\nDEVNAME=%s\nPARTN=%d\nPARTNAME=%s\n", device, i+1, name)
		if err := os.WriteFile(filepath.Join(path, "uevent"), []byte(event), 0o600); err != nil {
			t.Fatal(err)
		}
		nodes[i] = "/dev/" + device
	}
	return MountedStorage{ESP: nodes[0], CoreA: nodes[1], CoreB: nodes[2], Data: nodes[3]}
}
