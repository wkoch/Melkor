package pcdisk

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type MountedStorage struct {
	ESP   string
	CoreA string
	CoreB string
	Data  string
}

func (storage MountedStorage) StageLinks(destination string) error {
	work, err := os.MkdirTemp(filepath.Dir(destination), ".melkor-storage-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	for _, link := range [4]struct{ name, path string }{
		{"esp-device", storage.ESP}, {"core-a", storage.CoreA}, {"core-b", storage.CoreB}, {"data", storage.Data},
	} {
		if err := os.Symlink(link.path, filepath.Join(work, link.name)); err != nil {
			return err
		}
	}
	return os.Rename(work, destination)
}

// ResolveMounted identifica o disco pelo dispositivo que o kernel montou na raiz.
func ResolveMounted(mounts io.Reader, sysDevices string) (MountedStorage, error) {
	root, err := rootDevice(mounts)
	if err != nil {
		return MountedStorage{}, err
	}
	partPath, err := filepath.EvalSymlinks(filepath.Join(sysDevices, root))
	if err != nil {
		return MountedStorage{}, fmt.Errorf("dispositivo da raiz: %w", err)
	}
	parent := filepath.Dir(partPath)
	siblings, err := os.ReadDir(parent)
	if err != nil {
		return MountedStorage{}, err
	}
	names := [...]string{"esp", "core-a", "core-b", "persist"}
	var devices [4]string
	for _, entry := range siblings {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(parent, entry.Name())
		metadata, err := os.ReadFile(filepath.Join(path, "uevent"))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return MountedStorage{}, err
		}
		properties := make(map[string]string)
		for _, line := range strings.Split(string(metadata), "\n") {
			if key, value, ok := strings.Cut(line, "="); ok {
				properties[key] = value
			}
		}
		if properties["DEVTYPE"] != "partition" {
			continue
		}
		n, err := strconv.Atoi(properties["PARTN"])
		if err != nil || n < 1 || n > len(devices) || devices[n-1] != "" || properties["PARTNAME"] != names[n-1] || properties["DEVNAME"] != entry.Name() {
			return MountedStorage{}, fmt.Errorf("partição inesperada no disco do core: %s", entry.Name())
		}
		devices[n-1] = filepath.Join("/dev", entry.Name())
	}
	for index, device := range devices {
		if device == "" {
			return MountedStorage{}, fmt.Errorf("partição %s ausente no disco do core", names[index])
		}
	}
	rootPath := filepath.Join("/dev", filepath.Base(partPath))
	if rootPath != devices[1] && rootPath != devices[2] {
		return MountedStorage{}, fmt.Errorf("a raiz deve estar em um slot do core")
	}
	return MountedStorage{ESP: devices[0], CoreA: devices[1], CoreB: devices[2], Data: devices[3]}, nil
}

func rootDevice(mounts io.Reader) (string, error) {
	scanner := bufio.NewScanner(mounts)
	for scanner.Scan() {
		before, after, ok := strings.Cut(scanner.Text(), " - ")
		if !ok {
			continue
		}
		entry := strings.Fields(before)
		if len(entry) < 6 || entry[4] != "/" || !strings.HasPrefix(after, "squashfs ") {
			continue
		}
		for option := range strings.SplitSeq(entry[5], ",") {
			if option == "ro" {
				return entry[2], nil
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("montagem SquashFS somente leitura ausente na raiz")
}
