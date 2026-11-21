package pcdisk

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

var internalDiskName = regexp.MustCompile(`^(nvme[0-9]+n[0-9]+|sd[a-z]+)$`)
var stableDiskID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:+-]{0,254}$`)

const (
	blkLogicalSector = 0x1268
	blkGetSize64     = 0x80081272
	blkGetDiskSeq    = 0x80081280
	seekData         = 3
	seekHole         = 4
)

type Disk struct {
	ID       string
	Model    string
	Bytes    uint64
	Sector   uint64
	device   string
	sequence uint64
}

func InstallDisks() ([]Disk, error) {
	return installDisks("/dev", "/sys/class/block")
}

func installDisks(dev, sys string) ([]Disk, error) {
	entries, err := os.ReadDir(filepath.Join(dev, "disk/by-id"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var disks []Disk
	seen := map[string]bool{}
	for _, entry := range entries {
		disk, err := inspectDisk(dev, sys, entry.Name())
		if err == nil && !seen[disk.device] {
			disks = append(disks, disk)
			seen[disk.device] = true
		}
	}
	return disks, nil
}

func inspectDisk(dev, sys, id string) (Disk, error) {
	if !stableDiskID.MatchString(id) {
		return Disk{}, fmt.Errorf("identificador de disco inválido")
	}
	link := filepath.Join(dev, "disk/by-id", id)
	if _, err := os.Readlink(link); err != nil {
		return Disk{}, err
	}
	node, err := filepath.EvalSymlinks(link)
	if err != nil {
		return Disk{}, err
	}
	name := filepath.Base(node)
	if filepath.Dir(node) != dev || !internalDiskName.MatchString(name) {
		return Disk{}, fmt.Errorf("selecione um disco interno inteiro")
	}
	devicePath, err := filepath.EvalSymlinks(filepath.Join(sys, name))
	if err != nil {
		return Disk{}, err
	}
	for _, part := range strings.Split(devicePath, string(filepath.Separator)) {
		if strings.HasPrefix(part, "usb") || part == "virtual" {
			return Disk{}, fmt.Errorf("dispositivo de mídia externa ou virtual empilhado")
		}
	}
	read := func(field string) string {
		value, _ := os.ReadFile(filepath.Join(devicePath, field))
		return strings.TrimSpace(string(value))
	}
	if read("removable") != "0" || read("ro") != "0" || !strings.Contains("\n"+read("uevent")+"\n", "\nDEVTYPE=disk\n") {
		return Disk{}, fmt.Errorf("disco removível, protegido contra escrita ou sem metadados")
	}
	sectors, err := strconv.ParseUint(read("size"), 10, 54)
	if err != nil || sectors < MinimumDisk/512 {
		return Disk{}, fmt.Errorf("capacidade do disco inválida")
	}
	logical, err := strconv.ParseUint(read("queue/logical_block_size"), 10, 32)
	if err != nil || (logical != 512 && logical != 4096) || sectors*512%logical != 0 {
		return Disk{}, fmt.Errorf("tamanho de setor lógico incompatível")
	}
	sequence, err := strconv.ParseUint(read("diskseq"), 10, 64)
	if err != nil || sequence == 0 {
		return Disk{}, fmt.Errorf("sequência do disco indisponível")
	}
	return Disk{ID: id, Model: read("device/model"), Bytes: sectors * 512, Sector: logical, device: node, sequence: sequence}, nil
}

func diskIoctl(file *os.File, request uintptr, output unsafe.Pointer) error {
	connection, err := file.SyscallConn()
	if err != nil {
		return err
	}
	var result syscall.Errno
	if err := connection.Control(func(fd uintptr) {
		_, _, result = syscall.Syscall(syscall.SYS_IOCTL, fd, request, uintptr(output))
	}); err != nil {
		return err
	}
	if result != 0 {
		return result
	}
	return nil
}

func openInstallDisk(selected Disk) (*os.File, error) {
	current, err := inspectDisk("/dev", "/sys/class/block", selected.ID)
	if err != nil || current != selected {
		return nil, fmt.Errorf("o disco selecionado mudou")
	}
	file, err := os.OpenFile(selected.device, os.O_RDWR|syscall.O_EXCL|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, fmt.Errorf("abrir disco em modo exclusivo: %w", err)
	}
	info, err := file.Stat()
	var sequence, size uint64
	var sector uint32
	if err != nil || info.Mode()&os.ModeType != os.ModeDevice ||
		diskIoctl(file, blkGetDiskSeq, unsafe.Pointer(&sequence)) != nil ||
		diskIoctl(file, blkGetSize64, unsafe.Pointer(&size)) != nil ||
		diskIoctl(file, blkLogicalSector, unsafe.Pointer(&sector)) != nil ||
		sequence != selected.sequence || size != selected.Bytes || uint64(sector) != selected.Sector {
		file.Close()
		return nil, fmt.Errorf("a identidade do disco aberto diverge da seleção")
	}
	return file, nil
}

type region struct {
	start int64
	end   int64
}

func sparseRegions(file *os.File, start, end int64) ([]region, error) {
	var regions []region
	for start < end {
		data, err := file.Seek(start, seekData)
		if errors.Is(err, syscall.ENXIO) || (err == nil && data >= end) {
			break
		}
		if err != nil {
			return nil, err
		}
		hole, err := file.Seek(data, seekHole)
		if err != nil {
			return nil, err
		}
		if data < start || hole <= data {
			return nil, fmt.Errorf("extensão inválida no disco temporário")
		}
		regions = append(regions, region{data, min(end, hole)})
		start = hole
	}
	return regions, nil
}

func copyRegions(ctx context.Context, dst io.WriterAt, src io.ReaderAt, regions []region) error {
	buffer := make([]byte, MiB)
	for _, extent := range regions {
		for offset := extent.start; offset < extent.end; {
			if err := ctx.Err(); err != nil {
				return err
			}
			chunk := buffer[:min(int64(len(buffer)), extent.end-offset)]
			if _, err := src.ReadAt(chunk, offset); err != nil {
				return err
			}
			if n, err := dst.WriteAt(chunk, offset); err != nil || n != len(chunk) {
				if err != nil {
					return err
				}
				return io.ErrShortWrite
			}
			offset += int64(len(chunk))
		}
	}
	return nil
}

func writeDisk(ctx context.Context, dst, src *os.File, size int64) error {
	regions, err := sparseRegions(src, MiB, size-MiB)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	// Invalida as tabelas anteriores. Só publica a nova GPT após sincronizar os dados.
	zero := make([]byte, MiB)
	for _, offset := range []int64{0, size - MiB} {
		if _, err := dst.WriteAt(zero, offset); err != nil {
			return err
		}
	}
	if err := dst.Sync(); err != nil {
		return err
	}
	if err := copyRegions(ctx, dst, src, regions); err != nil {
		return err
	}
	if err := dst.Sync(); err != nil {
		return err
	}
	if err := copyRegions(ctx, dst, src, []region{{size - MiB, size}, {0, MiB}}); err != nil {
		return err
	}
	return dst.Sync()
}

// Install confirma a identidade pela sequência do kernel e usa somente o descritor exclusivo.
func Install(ctx context.Context, selected Disk, input ImageInput, scratch string) (Layout, error) {
	if os.Geteuid() != 0 || input.Credentials == "" {
		return Layout{}, fmt.Errorf("a instalação exige root e credenciais provisionadas")
	}
	staging, err := os.MkdirTemp(scratch, "melkor-install-")
	if err != nil {
		return Layout{}, err
	}
	defer os.RemoveAll(staging)
	input.Destination = filepath.Join(staging, "disk.img")
	input.SizeBytes = selected.Bytes
	layout, err := createImage(ctx, input, selected.Sector, false)
	if err != nil {
		return Layout{}, fmt.Errorf("preparar disco sem alterar o destino: %w", err)
	}
	image, err := os.Open(input.Destination)
	if err != nil {
		return Layout{}, err
	}
	defer image.Close()
	disk, err := openInstallDisk(selected)
	if err != nil {
		return Layout{}, err
	}
	defer disk.Close()
	if err := writeDisk(ctx, disk, image, int64(selected.Bytes)); err != nil {
		return Layout{}, fmt.Errorf("gravar disco: %w", err)
	}
	return layout, nil
}
