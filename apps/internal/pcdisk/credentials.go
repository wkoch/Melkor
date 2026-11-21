package pcdisk

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
)

const (
	credentialsDir  = ".melkor"
	credentialsName = "credenciais"
)

var passwordHash = regexp.MustCompile(`^\$6\$[A-Za-z0-9./]{1,16}\$[A-Za-z0-9./]{86}$`)

type credentials struct {
	root string
	user string
}

func readCredentials(path string, owner func(os.FileInfo) bool) (credentials, error) {
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return credentials{}, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return credentials{}, err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 || (owner != nil && !owner(info)) {
		return credentials{}, fmt.Errorf("propriedade ou permissões do arquivo de credenciais inválidas")
	}
	data, err := io.ReadAll(io.LimitReader(file, 1024))
	if err != nil {
		return credentials{}, err
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if len(lines) != 2 || len(data) == 1024 {
		return credentials{}, fmt.Errorf("credenciais exigem dois hashes locais")
	}
	root, hasRoot := strings.CutPrefix(lines[0], "root:")
	user, hasUser := strings.CutPrefix(lines[1], "melkor:")
	if !hasRoot || !hasUser || !passwordHash.MatchString(root) || !passwordHash.MatchString(user) || root == user {
		return credentials{}, fmt.Errorf("credenciais exigem hashes SHA-512 crypt distintos para root e melkor")
	}
	return credentials{root: root, user: user}, nil
}

func stageCredentials(path, work string) (string, error) {
	c, err := readCredentials(path, nil)
	if err != nil {
		return "", err
	}
	seed := filepath.Join(work, "persist")
	dir := filepath.Join(seed, credentialsDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	text := "root:" + c.root + "\nmelkor:" + c.user + "\n"
	return seed, os.WriteFile(filepath.Join(dir, credentialsName), []byte(text), 0o600)
}

func (c credentials) mergeShadow(stock string) (string, error) {
	lines := strings.Split(stock, "\n")
	foundRoot, foundUser := false, false
	for index, line := range lines {
		name, fields, hasField := strings.Cut(line, ":")
		if !hasField {
			continue
		}
		_, suffix, hasSuffix := strings.Cut(fields, ":")
		switch name {
		case "root":
			if foundRoot || !hasSuffix {
				return "", fmt.Errorf("entrada root inválida no shadow")
			}
			foundRoot = true
			lines[index] = name + ":" + c.root + ":" + suffix
		case "melkor":
			if foundUser || !hasSuffix {
				return "", fmt.Errorf("entrada melkor inválida no shadow")
			}
			foundUser = true
			lines[index] = name + ":" + c.user + ":" + suffix
		}
	}
	if !foundRoot || !foundUser {
		return "", fmt.Errorf("contas locais ausentes no shadow")
	}
	return strings.Join(lines, "\n"), nil
}

func rootOwned(info os.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && stat.Uid == 0 && stat.Gid == 0
}

// ApplyCredentials mantém as contas de serviço da geração e carrega as senhas locais.
func ApplyCredentials(data, shadow, runtime string, bind func(source, target string) error) error {
	return applyCredentials(data, shadow, runtime, rootOwned, bind)
}

func applyCredentials(data, shadow, runtime string, owner func(os.FileInfo) bool, bind func(source, target string) error) error {
	dir := filepath.Join(data, credentialsDir)
	info, err := os.Lstat(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode().Perm() != 0o700 || !owner(info) {
		return fmt.Errorf("diretório de credenciais precisa pertencer a root com permissão 0700")
	}
	c, err := readCredentials(filepath.Join(dir, credentialsName), owner)
	if err != nil {
		return err
	}
	stock, err := os.ReadFile(shadow)
	if err != nil {
		return err
	}
	merged, err := c.mergeShadow(string(stock))
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(runtime, ".shadow-")
	if err != nil {
		return err
	}
	_, err = io.WriteString(file, merged)
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = bind(file.Name(), shadow)
	}
	if err != nil {
		os.Remove(file.Name())
	}
	return err
}
