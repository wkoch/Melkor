package main

import (
	"fmt"
	"os"
	"syscall"

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
	var err error
	switch {
	case len(os.Args) == 1:
		err = run("/proc/self/mountinfo", "/sys/dev/block", "/run/melkor")
	case len(os.Args) == 2 && os.Args[1] == "credenciais":
		err = pcdisk.ApplyCredentials("/persist", "/etc/shadow", "/run/melkor", func(source, target string) error {
			return syscall.Mount(source, target, "", syscall.MS_BIND, "")
		})
	default:
		err = fmt.Errorf("uso: melkor-storage [credenciais]")
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
