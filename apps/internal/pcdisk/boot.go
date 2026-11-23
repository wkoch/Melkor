package pcdisk

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode/utf16"
)

const (
	efiLoaderGUID = "-4a67b082-0a4c-41cf-b6c7-440b29bb8c4f"
	maxKernelSize = 28 << 20
)

func BootEntry(slot string) string  { return "Melkor Linux - slot " + slot }
func KernelName(slot string) string { return "kernel-" + slot }

func ReadKernel(ctx context.Context, squashfs string) ([]byte, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	command := exec.CommandContext(ctx, "unsquashfs", "-processors", "1", "-cat", squashfs, "boot/bzImage")
	command.Stderr = os.Stderr
	pipe, err := command.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := command.Start(); err != nil {
		return nil, err
	}
	kernel, err := io.ReadAll(io.LimitReader(pipe, maxKernelSize+1))
	if err != nil || len(kernel) > maxKernelSize {
		cancel()
		_ = command.Wait()
		return nil, errors.New("falha ao extrair o kernel ou tamanho acima do limite")
	}
	if err := command.Wait(); err != nil {
		return nil, fmt.Errorf("extrair kernel do SquashFS: %w", err)
	}
	if len(kernel) < 0x206 || string(kernel[0x202:0x206]) != "HdrS" {
		return nil, errors.New("o SquashFS não contém um bzImage Linux válido")
	}
	return kernel, nil
}

type BootBackend struct {
	ESP, EFI, Runtime, Booted string
}

func validSlot(slot string) bool { return slot == "A" || slot == "B" }

func BootedSlot(cmdline string) string {
	var slot string
	for _, arg := range strings.Fields(cmdline) {
		value, ok := strings.CutPrefix(arg, "rauc.slot=")
		if !ok {
			continue
		}
		if slot != "" || !validSlot(value) {
			return ""
		}
		slot = value
	}
	return slot
}

func (b BootBackend) variable(name string) (string, error) {
	path := filepath.Join(b.EFI, name+efiLoaderGUID)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	for _, slot := range []string{"A", "B"} {
		if string(data) == string(encodeEntry(slot)) {
			return slot, nil
		}
	}
	return "", fmt.Errorf("variável EFI %s tem uma entrada desconhecida", name)
}

func encodeEntry(slot string) []byte {
	data := []byte{7, 0, 0, 0}
	for _, ch := range utf16.Encode([]rune(BootEntry(slot) + "\x00")) {
		data = binary.LittleEndian.AppendUint16(data, ch)
	}
	return data
}

func (b BootBackend) setVariable(ctx context.Context, name, slot string) error {
	path := filepath.Join(b.EFI, name+efiLoaderGUID)
	data := encodeEntry(slot)
	err := os.WriteFile(path, data, 0o600)
	if !errors.Is(err, os.ErrPermission) {
		return err
	}

	// O efivarfs monta as variáveis do Boot Loader Interface existentes como
	// imutáveis. Só libere o arquivo escolhido pelo backend, durante esta escrita.
	if output, unlockErr := exec.CommandContext(ctx, "/bin/busybox", "chattr", "-i", path).CombinedOutput(); unlockErr != nil {
		return fmt.Errorf("liberar %s: %w (%s)", name, unlockErr, strings.TrimSpace(string(output)))
	}
	err = os.WriteFile(path, data, 0o600)
	if output, lockErr := exec.Command("/bin/busybox", "chattr", "+i", path).CombinedOutput(); lockErr != nil {
		return errors.Join(err, fmt.Errorf("proteger %s: %w (%s)", name, lockErr, strings.TrimSpace(string(output))))
	}
	return err
}

func (b BootBackend) confirmed() (string, error) {
	slot, err := b.variable("LoaderEntryDefault")
	if slot == "" {
		slot = "A"
	}
	return slot, err
}

func (b BootBackend) kernel(slot string) string { return filepath.Join(b.ESP, KernelName(slot)) }

func (b BootBackend) ready(slot string) (bool, error) {
	info, err := os.Stat(b.kernel(slot))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return err == nil && info.Mode().IsRegular(), err
}

func (b BootBackend) mayWrite(slot string) error {
	current, err := b.confirmed()
	if err != nil {
		return err
	}
	pending, err := b.variable("LoaderEntryOneShot")
	if err != nil {
		return err
	}
	if current != b.Booted || slot == current || pending != "" {
		return errors.New("inicie pela geração confirmada e conclua a atualização pendente")
	}
	return nil
}

func (b BootBackend) stage(ctx context.Context, slot string) error {
	kernel, err := ReadKernel(ctx, filepath.Join(b.Runtime, "core-"+strings.ToLower(slot)))
	if err != nil {
		return err
	}
	final := b.kernel(slot)
	staged := final + ".new"
	if err := os.Remove(staged); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	stage, err := os.OpenFile(staged, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer os.Remove(staged)
	_, err = stage.Write(kernel)
	if err == nil {
		err = stage.Sync()
	}
	if closeErr := stage.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err := os.Rename(staged, final); err != nil {
		return err
	}
	return syncBootDir(b.ESP)
}

func syncBootDir(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func (b BootBackend) Run(ctx context.Context, args []string, output io.Writer) error {
	if len(args) == 0 || !validSlot(b.Booted) {
		return errors.New("slot iniciado ou comando RAUC inválido")
	}
	if len(args) == 1 && args[0] == "get-primary" {
		slot, err := b.variable("LoaderEntryOneShot")
		if slot == "" && err == nil {
			slot, err = b.confirmed()
		}
		if err == nil {
			_, err = fmt.Fprintln(output, slot)
		}
		return err
	}
	if len(args) < 2 || !validSlot(args[1]) {
		return errors.New("slot RAUC inválido")
	}
	slot := args[1]
	switch args[0] {
	case "get-state":
		if len(args) == 2 {
			ready, err := b.ready(slot)
			if err != nil {
				return err
			}
			state := "bad"
			if ready {
				state = "good"
			}
			_, err = fmt.Fprintln(output, state)
			return err
		}
	case "set-primary":
		if len(args) == 2 {
			if err := b.mayWrite(slot); err != nil {
				return err
			}
			if err := b.stage(ctx, slot); err != nil {
				return err
			}
			return b.setVariable(ctx, "LoaderEntryOneShot", slot)
		}
	case "set-state":
		if len(args) != 3 {
			break
		}
		switch args[2] {
		case "bad":
			if err := b.mayWrite(slot); err != nil {
				return err
			}
			if err := os.Remove(b.kernel(slot)); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
			return syncBootDir(b.ESP)
		case "good":
			ready, err := b.ready(slot)
			if err != nil {
				return err
			}
			if !ready || slot != b.Booted {
				return errors.New("somente o slot iniciado com kernel disponível pode ser confirmado")
			}
			current, err := b.confirmed()
			if err != nil || current == slot {
				return err
			}
			return b.setVariable(ctx, "LoaderEntryDefault", slot)
		}
	}
	return errors.New("comando RAUC inválido")
}
