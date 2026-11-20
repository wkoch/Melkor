package main

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

func listGenerations(profile string, out io.Writer) error {
	entries, err := os.ReadDir(filepath.Join(profile, "geracoes"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	active, err := os.Readlink(filepath.Join(profile, "atual"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	type generation struct {
		name     string
		modified time.Time
	}
	var generations []generation
	for _, entry := range entries {
		if !entry.IsDir() || !validGeneration(entry.Name()) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		generations = append(generations, generation{entry.Name(), info.ModTime()})
	}
	if len(generations) == 0 {
		_, err := fmt.Fprintln(out, "Nenhuma geração publicada.")
		return err
	}
	slices.SortFunc(generations, func(a, b generation) int {
		if order := b.modified.Compare(a.modified); order != 0 {
			return order
		}
		return cmp.Compare(a.name, b.name)
	})
	for _, item := range generations {
		marker := " "
		if active == filepath.Join("geracoes", item.name) {
			marker = "*"
		}
		summary := "manifesto indisponível"
		_, packages, err := readGeneration(profile, item.name)
		if err == nil {
			apps := make([]string, 0, len(packages))
			for _, name := range slices.Sorted(maps.Keys(packages)) {
				label := strconv.QuoteToASCII(name + " " + packages[name].Version)
				apps = append(apps, label[1:len(label)-1])
			}
			summary = strings.Join(apps, ", ")
			if summary == "" {
				summary = "perfil vazio"
			}
		}
		if _, err := fmt.Fprintf(out, "%s %s  %s\n", marker, item.name, summary); err != nil {
			return err
		}
	}
	return nil
}

func restoreGeneration(profile, name string, podman packageBackend) error {
	if !validGeneration(name) {
		return fmt.Errorf("identificador de geração inválido: %q", name)
	}
	lock, err := lockProfile(profile)
	if err != nil {
		return err
	}
	defer lock.Close()

	dir, packages, err := readGeneration(profile, name)
	if err != nil {
		return fmt.Errorf("ler geração %s: %w", name, err)
	}
	if err := checkOwners(packages); err != nil {
		return err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer root.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for appName, app := range packages {
		if !namePattern.MatchString(appName) {
			return fmt.Errorf("nome de aplicativo inválido na geração %s: %q", name, appName)
		}
		for _, file := range app.Files {
			info, err := root.Lstat(filepath.Join("bin", file))
			if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
				return fmt.Errorf("executável indisponível na geração %s: %s", name, file)
			}
		}
		for _, file := range app.Config {
			info, err := root.Lstat(filepath.Join("config", file))
			if err != nil || !info.Mode().IsRegular() {
				return fmt.Errorf("configuração indisponível na geração %s: %s", name, file)
			}
		}
		if app.Binary != nil {
			if !validPackageImage(app.Binary.Image) || !validPackageEntry(app.Binary.Entry) {
				return fmt.Errorf("manifesto de pacote inválido para %s", appName)
			}
			if _, err := podman(ctx, "image", "exists", app.Binary.Image); err != nil {
				return fmt.Errorf("imagem de %s indisponível no Podman: %w", appName, err)
			}
		}
	}
	_, err = activateGeneration(profile, name)
	return err
}
