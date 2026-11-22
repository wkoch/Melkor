package pcdisk

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestPlanoUsaODiscoInteiroComIDsUnicos(t *testing.T) {
	pattern := regexp.MustCompile(`^[a-f0-9]{8}-[a-f0-9]{4}-4[a-f0-9]{3}-[89ab][a-f0-9]{3}-[a-f0-9]{12}$`)
	seen := make(map[string]bool)
	for _, sector := range []uint64{512, 4096} {
		disk := uint64(16*GiB) + sector
		layout, err := Plan(disk, sector, SlotSize)
		if err != nil {
			t.Fatal(err)
		}
		checkID := func(id string) {
			t.Helper()
			if !pattern.MatchString(id) || seen[id] {
				t.Fatalf("UUID inválido ou repetido: %s", id)
			}
			seen[id] = true
		}
		checkID(layout.DiskUUID)
		expectedNames := [4]string{"esp", "core-a", "core-b", "persist"}
		expectedStart := uint64(MiB) / sector
		for index, part := range layout.Partitions {
			checkID(part.UUID)
			if part.Name != expectedNames[index] || part.Start != expectedStart || part.Size*sector%MiB != 0 {
				t.Fatalf("posição ou alinhamento inesperado: %+v", part)
			}
			expectedStart += part.Size
		}
		if layout.Partitions[3].Size*sector < SwapSize || disk-expectedStart*sector != MiB+sector {
			t.Fatalf("espaço persistente incorreto: %+v", layout.Partitions)
		}
	}
}

func TestRejeitaDiscoESlotInvalidos(t *testing.T) {
	for _, input := range [][3]uint64{{MinimumDisk, 1024, 1}, {MinimumDisk - 512, 512, 1}, {MinimumDisk + 1, 512, 1}, {MinimumDisk, 512, 0}, {MinimumDisk, 512, SlotSize + 1}} {
		if _, err := Plan(input[0], input[1], input[2]); err == nil {
			t.Fatalf("plano aceitou %v", input)
		}
	}
}

func TestPlanoEfetivamenteGeraUmaGPTEmArquivo(t *testing.T) {
	sfdisk, err := exec.LookPath("sfdisk")
	if err != nil {
		t.Skip("sfdisk indisponível")
	}
	path := filepath.Join(t.TempDir(), "virtual.img")
	disk, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := disk.Truncate(MinimumDisk); err != nil {
		disk.Close()
		t.Fatal(err)
	}
	if err := disk.Close(); err != nil {
		t.Fatal(err)
	}
	layout, err := Plan(MinimumDisk, 512, 1)
	if err != nil {
		t.Fatal(err)
	}
	write := exec.Command(sfdisk, "--no-tell-kernel", "--no-reread", path)
	write.Stdin = strings.NewReader(layout.Sfdisk())
	if output, err := write.CombinedOutput(); err != nil {
		t.Fatalf("sfdisk: %v\n%s", err, output)
	}
	output, err := exec.Command(sfdisk, "--json", path).Output()
	if err != nil {
		t.Fatal(err)
	}
	var parsed struct {
		Table struct {
			Label      string
			ID         string
			Partitions []Partition
		} `json:"partitiontable"`
	}
	if err := json.Unmarshal(output, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.Table.Label != "gpt" || !strings.EqualFold(parsed.Table.ID, layout.DiskUUID) || len(parsed.Table.Partitions) != 4 {
		t.Fatalf("tabela criada com valores incorretos: %s", output)
	}
	for index, part := range parsed.Table.Partitions {
		part.UUID = strings.ToLower(part.UUID)
		part.Type = strings.ToLower(part.Type)
		if part != layout.Partitions[index] {
			t.Fatalf("partição %d criada com valores incorretos: %+v", index, part)
		}
	}
}
