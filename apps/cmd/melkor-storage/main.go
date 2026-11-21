package main

import (
	"fmt"
	"os"

	"melkor.local/apps/internal/pcdisk"
)

func run(mountInfo, sysDevices, destination string) error {
	mounts, err := os.Open(mountInfo)
	if err != nil {
		return err
	}
	defer mounts.Close()
	storage, err := pcdisk.ResolveMounted(mounts, sysDevices)
	if err != nil {
		return err
	}
	return storage.StageLinks(destination)
}

func main() {
	if err := run("/proc/self/mountinfo", "/sys/dev/block", "/run/melkor"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
