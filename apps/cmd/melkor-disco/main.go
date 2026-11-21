package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"

	"melkor.local/apps/internal/pcdisk"
)

func run(ctx context.Context, args []string, output io.Writer) error {
	flags := flag.NewFlagSet("melkor-disco", flag.ContinueOnError)
	flags.SetOutput(output)
	var input pcdisk.ImageInput
	flags.StringVar(&input.Destination, "saida", "", "novo arquivo de disco virtual")
	flags.StringVar(&input.Core, "core", "", "arquivo SquashFS dos slots")
	flags.StringVar(&input.CoreSHA256, "sha256", "", "SHA-256 esperado do core")
	flags.StringVar(&input.EFI, "efi", "", "executável GRUB bootx64.efi")
	flags.StringVar(&input.GRUB, "grub", "", "modelo de grub.cfg da geração")
	size := flags.Uint64("gib", 16, "capacidade do disco virtual em GiB")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || input.Destination == "" || input.Core == "" || input.EFI == "" || input.GRUB == "" || input.CoreSHA256 == "" {
		return fmt.Errorf("informe -saida, -core, -sha256, -efi e -grub")
	}
	if *size > (1<<63-1)/pcdisk.GiB {
		return fmt.Errorf("capacidade do disco fora do limite")
	}
	input.SizeBytes = *size * pcdisk.GiB
	layout, err := pcdisk.CreateImage(ctx, input)
	if err != nil {
		return err
	}
	return json.NewEncoder(output).Encode(layout)
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	if err := run(ctx, os.Args[1:], os.Stdout); err != nil && !errors.Is(err, flag.ErrHelp) {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
