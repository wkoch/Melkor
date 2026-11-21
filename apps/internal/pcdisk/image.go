package pcdisk

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type ImageInput struct {
	Destination string
	SizeBytes   uint64
	Core        string
	CoreSHA256  string
	EFI         string
	GRUB        string
	Credentials string
}

func CreateImage(ctx context.Context, input ImageInput) (Layout, error) {
	if input.SizeBytes > uint64(1<<63-1) {
		return Layout{}, fmt.Errorf("imagem de disco excede o tamanho suportado")
	}
	work, err := os.MkdirTemp(filepath.Dir(input.Destination), ".melkor-disco-")
	if err != nil {
		return Layout{}, err
	}
	defer os.RemoveAll(work)

	core, err := stageCore(input, work)
	if err != nil {
		return Layout{}, err
	}
	defer core.Close()
	info, err := core.Stat()
	if err != nil {
		return Layout{}, err
	}
	layout, err := Plan(input.SizeBytes, 512, uint64(info.Size()))
	if err != nil {
		return Layout{}, err
	}
	esp, err := createESP(ctx, input, layout, work)
	if err != nil {
		return Layout{}, err
	}
	defer esp.Close()
	disk, err := os.OpenFile(filepath.Join(work, "disk.img"), os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
	if err != nil {
		return Layout{}, err
	}
	defer disk.Close()
	if err := disk.Truncate(int64(input.SizeBytes)); err != nil {
		return Layout{}, err
	}
	if err := run(ctx, layout.Sfdisk(), "sfdisk", "--no-reread", "--no-tell-kernel", disk.Name()); err != nil {
		return Layout{}, err
	}
	for index, source := range []*os.File{esp, core, core} {
		if _, err := source.Seek(0, io.SeekStart); err != nil {
			return Layout{}, err
		}
		offset := int64(layout.Partitions[index].Start * layout.SectorSize)
		if _, err := io.Copy(io.NewOffsetWriter(disk, offset), source); err != nil {
			return Layout{}, err
		}
	}
	data := layout.Partitions[3]
	offset := data.Start * layout.SectorSize
	blocks := data.Size * layout.SectorSize / 4096
	mkfs := []string{"-q", "-F", "-t", "ext4", "-b", "4096", "-L", "melkor-data",
		"-E", fmt.Sprintf("offset=%d,root_owner=0:0", offset), disk.Name(), fmt.Sprint(blocks)}
	program := "mke2fs"
	if input.Credentials != "" {
		seed, err := stageCredentials(input.Credentials, work)
		if err != nil {
			return Layout{}, err
		}
		program = "fakeroot"
		mkfs = append([]string{"--", "/bin/sh", "-ec", `seed=$1; shift; chown -R 0:0 "$seed"; exec mke2fs -d "$seed" "$@"`,
			"melkor-disco", seed}, mkfs...)
	}
	if err := run(ctx, "", program, mkfs...); err != nil {
		return Layout{}, err
	}
	if err := disk.Sync(); err != nil {
		return Layout{}, err
	}
	if err := ctx.Err(); err != nil {
		return Layout{}, err
	}
	if err := os.Link(disk.Name(), input.Destination); err != nil {
		return Layout{}, err
	}
	return layout, nil
}

func stageCore(input ImageInput, work string) (*os.File, error) {
	want, err := hex.DecodeString(input.CoreSHA256)
	if err != nil || len(want) != sha256.Size {
		return nil, fmt.Errorf("SHA-256 do core inválido")
	}
	source, err := os.Open(input.Core)
	if err != nil {
		return nil, err
	}
	defer source.Close()
	stat, err := source.Stat()
	if err != nil {
		return nil, err
	}
	if !stat.Mode().IsRegular() || stat.Size() < 4 || stat.Size() > SlotSize {
		return nil, fmt.Errorf("core deve ser um arquivo regular de até %d GiB", SlotSize/GiB)
	}
	var magic [4]byte
	if _, err := source.ReadAt(magic[:], 0); err != nil || string(magic[:]) != "hsqs" {
		return nil, fmt.Errorf("core deve estar em formato SquashFS")
	}
	staged, err := os.OpenFile(filepath.Join(work, "core.squashfs"), os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	digest := sha256.New()
	written, err := io.Copy(io.MultiWriter(staged, digest), io.LimitReader(source, stat.Size()+1))
	if err != nil || written != stat.Size() || !strings.EqualFold(hex.EncodeToString(digest.Sum(nil)), input.CoreSHA256) {
		staged.Close()
		return nil, fmt.Errorf("conteúdo ou tamanho do core difere do esperado")
	}
	return staged, nil
}

func createESP(ctx context.Context, input ImageInput, layout Layout, work string) (*os.File, error) {
	config, err := os.ReadFile(input.GRUB)
	if err != nil {
		return nil, err
	}
	script := string(config)
	for index, token := range []string{"@CORE_A@", "@CORE_B@"} {
		if strings.Count(script, token) != 1 {
			return nil, fmt.Errorf("modelo GRUB deve conter exatamente uma ocorrência de %s", token)
		}
		script = strings.Replace(script, token, layout.Partitions[index+1].UUID, 1)
	}
	bootDir := filepath.Join(work, "EFI", "BOOT")
	if err := os.MkdirAll(bootDir, 0o700); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(bootDir, "grub.cfg"), []byte(script), 0o600); err != nil {
		return nil, err
	}
	efi, err := os.ReadFile(input.EFI)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(bootDir, "bootx64.efi"), efi, 0o600); err != nil {
		return nil, err
	}
	state := filepath.Join(bootDir, "grubenv")
	if err := run(ctx, "", "grub-editenv", state, "create"); err != nil {
		return nil, err
	}
	if err := run(ctx, "", "grub-editenv", state, "set", "ORDER=A B", "A_OK=1", "A_TRY=0", "B_OK=0", "B_TRY=0"); err != nil {
		return nil, err
	}
	esp, err := os.OpenFile(filepath.Join(work, "esp.vfat"), os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := esp.Truncate(ESPSize); err != nil {
		esp.Close()
		return nil, err
	}
	if err := run(ctx, "", "mkfs.vfat", "-F", "32", esp.Name()); err != nil {
		esp.Close()
		return nil, err
	}
	if err := run(ctx, "", "mcopy", "-i", esp.Name(), "-sp", filepath.Join(work, "EFI"), "::"); err != nil {
		esp.Close()
		return nil, err
	}
	return esp, nil
}

func run(ctx context.Context, input, command string, args ...string) error {
	process := exec.CommandContext(ctx, command, args...)
	process.Stdin = strings.NewReader(input)
	output, err := process.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w\n%s", command, err, output)
	}
	return nil
}
