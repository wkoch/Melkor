package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"unicode"

	"melkor.local/apps/internal/pcdisk"
)

func line(in *bufio.Reader, out io.Writer, prompt string) (string, error) {
	fmt.Fprint(out, prompt)
	value, err := in.ReadSlice('\n')
	return strings.TrimRight(string(value), "\r\n"), err
}

func selectDisk(in *bufio.Reader, out io.Writer, disks []pcdisk.Disk) (pcdisk.Disk, error) {
	if len(disks) == 0 {
		return pcdisk.Disk{}, fmt.Errorf("nenhum disco interno elegível encontrado")
	}
	fmt.Fprintln(out, "A instalação apagará todas as partições do disco escolhido.")
	for _, disk := range disks {
		fmt.Fprintf(out, "%s (%q, %.1f GiB, setor de %d bytes)\n", disk.ID, disk.Model, float64(disk.Bytes)/pcdisk.GiB, disk.Sector)
	}
	id, err := line(in, out, "ID do disco: ")
	if err != nil {
		return pcdisk.Disk{}, err
	}
	for _, disk := range disks {
		if disk.ID == id {
			token := "APAGAR " + disk.ID
			confirmation, err := line(in, out, "Digite "+token+" para confirmar: ")
			if err != nil {
				return pcdisk.Disk{}, err
			}
			if confirmation != token {
				return pcdisk.Disk{}, fmt.Errorf("instalação cancelada")
			}
			return disk, nil
		}
	}
	return pcdisk.Disk{}, fmt.Errorf("ID ausente na lista de discos")
}

func readPassword(in *bufio.Reader, out io.Writer, account string) (string, error) {
	password, err := line(in, out, "Senha de "+account+" (12 a 128 bytes): ")
	fmt.Fprintln(out)
	if err != nil {
		return "", err
	}
	confirmation, err := line(in, out, "Repita a senha de "+account+": ")
	fmt.Fprintln(out)
	if err != nil {
		return "", err
	}
	if len(password) < 12 || len(password) > 128 || strings.IndexFunc(password, unicode.IsControl) >= 0 || password != confirmation {
		return "", fmt.Errorf("senha inválida ou confirmação divergente para %s", account)
	}
	return password, nil
}

func credentials(ctx context.Context, in *bufio.Reader, terminal *os.File, out io.Writer) (string, error) {
	stty := func(args ...string) ([]byte, error) {
		cmd := exec.Command("stty", args...)
		cmd.Stdin = terminal
		return cmd.Output()
	}
	settings, err := stty("-g")
	if err != nil {
		return "", fmt.Errorf("leitura de senha exige terminal: %w", err)
	}
	if _, err := stty("-echo"); err != nil {
		return "", err
	}
	defer stty(strings.TrimSpace(string(settings)))
	return collectCredentials(in, out, func(password string) (string, error) {
		cmd := exec.CommandContext(ctx, "/bin/busybox", "mkpasswd", "-m", "sha512", "-s")
		cmd.Stdin = strings.NewReader(password + "\n")
		hash, err := cmd.Output()
		return strings.TrimSpace(string(hash)), err
	})
}

func collectCredentials(in *bufio.Reader, out io.Writer, hash func(string) (string, error)) (string, error) {
	root, err := readPassword(in, out, "root")
	if err != nil {
		return "", err
	}
	user, err := readPassword(in, out, "melkor")
	if err != nil {
		return "", err
	}
	if root == user {
		return "", fmt.Errorf("use senhas distintas para root e melkor")
	}
	var hashes strings.Builder
	for _, account := range []struct{ name, password string }{{"root", root}, {"melkor", user}} {
		digest, err := hash(account.password)
		if err != nil {
			return "", fmt.Errorf("gerar hash da senha: %w", err)
		}
		fmt.Fprintf(&hashes, "%s:%s\n", account.name, digest)
	}
	return hashes.String(), nil
}

func install(ctx context.Context, in *bufio.Reader, out io.Writer) error {
	disks, err := pcdisk.InstallDisks()
	if err != nil {
		return err
	}
	selected, err := selectDisk(in, out, disks)
	if err != nil {
		return err
	}
	sha, err := os.ReadFile("/melkor/core.sha256")
	if err != nil {
		return err
	}
	hashes, err := credentials(ctx, in, os.Stdin, out)
	if err != nil {
		return err
	}
	file, err := os.CreateTemp("/run", ".credenciais-")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	_, err = io.WriteString(file, hashes)
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	fmt.Fprintln(out, "Preparando o sistema e verificando os arquivos de instalação.")
	_, err = pcdisk.Install(ctx, selected, pcdisk.ImageInput{
		Core: "/melkor/rootfs.squashfs", CoreSHA256: strings.TrimSpace(string(sha)),
		EFI: "/melkor/bootx64.efi", GRUB: "/melkor/grub-pc.cfg", Credentials: file.Name(),
	}, "/run")
	return err
}

func main() {
	if os.Geteuid() != 0 {
		fmt.Fprintln(os.Stderr, "O instalador exige root.")
		os.Exit(1)
	}
	if err := os.Setenv("PATH", "/usr/sbin:/usr/bin:/sbin:/bin"); err != nil {
		os.Exit(1)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	input := bufio.NewReader(os.Stdin)
	if err := install(ctx, input, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "\nInstalação encerrada: %v\n", err)
	} else {
		fmt.Fprintln(os.Stdout, "\nInstalação concluída. Remova a mídia de instalação ao reiniciar.")
	}
	if answer, err := line(input, os.Stdout, "Digite REINICIAR para reiniciar ou pressione Enter para voltar: "); err == nil && answer == "REINICIAR" {
		_ = exec.Command("/sbin/reboot").Run()
	}
}
