package pcdisk

import (
	"crypto/rand"
	"fmt"
	"strings"
)

const (
	MiB = 1 << 20
	GiB = 1 << 30

	MinimumDisk = 12 * GiB
	ESPSize     = 128 * MiB
	SlotSize    = 2 * GiB
	SwapSize    = 4 * GiB

	espType   = "c12a7328-f81f-11d2-ba4b-00a0c93ec93b"
	linuxType = "0fc63daf-8483-4772-8e79-3d69d8477de4"
)

type Partition struct {
	Name  string
	Type  string
	UUID  string
	Start uint64 // Em setores lógicos.
	Size  uint64 // Em setores lógicos.
}

type Layout struct {
	DiskUUID   string
	SectorSize uint64
	Partitions [4]Partition
}

// Plan reserva a GPT nas extremidades e distribui o restante entre os slots e os dados.
func Plan(diskBytes, sectorSize, coreBytes uint64) (Layout, error) {
	if sectorSize != 512 && sectorSize != 4096 {
		return Layout{}, fmt.Errorf("tamanho de setor lógico não suportado: %d", sectorSize)
	}
	if diskBytes < MinimumDisk || diskBytes%sectorSize != 0 {
		return Layout{}, fmt.Errorf("o disco deve ter pelo menos %d GiB e terminar em um limite de setor", MinimumDisk/GiB)
	}
	if coreBytes == 0 || coreBytes > SlotSize {
		return Layout{}, fmt.Errorf("o tamanho do core deve caber em um slot de %d GiB", SlotSize/GiB)
	}
	layout := Layout{DiskUUID: uuid(), SectorSize: sectorSize}
	start := uint64(MiB) / sectorSize
	end := (diskBytes/MiB - 1) * MiB / sectorSize
	for index, entry := range [4]struct {
		name string
		kind string
		size uint64
	}{
		{"esp", espType, ESPSize / sectorSize},
		{"core-a", linuxType, SlotSize / sectorSize},
		{"core-b", linuxType, SlotSize / sectorSize},
		{"persist", linuxType, end - start - (ESPSize+2*SlotSize)/sectorSize},
	} {
		layout.Partitions[index] = Partition{Name: entry.name, Type: entry.kind, UUID: uuid(), Start: start, Size: entry.size}
		start += entry.size
	}
	return layout, nil
}

func (layout Layout) Sfdisk() string {
	var script strings.Builder
	fmt.Fprintf(&script, "label: gpt\nlabel-id: %s\nunit: sectors\nsector-size: %d\n\n", layout.DiskUUID, layout.SectorSize)
	for _, part := range layout.Partitions {
		fmt.Fprintf(&script, "start=%d, size=%d, type=%s, uuid=%s, name=%s\n", part.Start, part.Size, part.Type, part.UUID, part.Name)
	}
	return script.String()
}

func uuid() string {
	var value [16]byte
	rand.Read(value[:])
	value[6] = value[6]&0x0f | 0x40
	value[8] = value[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", value[:4], value[4:6], value[6:8], value[8:10], value[10:])
}
