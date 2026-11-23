package main

import (
	"context"
	"fmt"
	"os"
	"syscall"

	"melkor.local/apps/internal/pcdisk"
)

func main() {
	const efivars = "/sys/firmware/efi/efivars"
	if os.Geteuid() != 0 {
		fmt.Fprintln(os.Stderr, "O backend de boot exige root.")
		os.Exit(1)
	}
	var fs syscall.Statfs_t
	if syscall.Statfs(efivars, &fs) != nil || fs.Type != 0xde5e81e4 {
		fmt.Fprintln(os.Stderr, "efivarfs indisponível.")
		os.Exit(1)
	}
	cmdline, err := os.ReadFile("/proc/cmdline")
	if err == nil {
		backend := pcdisk.BootBackend{
			ESP: "/run/melkor/esp/EFI/BOOT", EFI: efivars, Runtime: "/run/melkor",
			Booted: pcdisk.BootedSlot(string(cmdline)),
		}
		err = backend.Run(context.Background(), os.Args[1:], os.Stdout)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
