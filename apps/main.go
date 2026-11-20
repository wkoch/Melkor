package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
)

func main() {
	if err := run(os.Args[1:], syscall.Exec); err != nil {
		fmt.Fprintln(os.Stderr, "melkor:", err)
		os.Exit(1)
	}
}

func run(args []string, replace func(string, []string, []string) error) error {
	const usage = "Uso: melkor instalar <nome> | melkor atualizar | melkor importar | melkor executar <nome> [argumentos] | melkor remover <nome> | melkor shell [argumentos] | melkor catalogo | melkor receita"
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Println(usage)
		return nil
	}
	profile, err := profileDir()
	if err != nil {
		return err
	}
	switch args[0] {
	case "instalar":
		if len(args) == 2 && args[1] != "" {
			return update(profile, args[1], build)
		}
	case "atualizar":
		if len(args) == 1 {
			return update(profile, "", build)
		}
	case "importar":
		return importPackage(profile, args[1:], packageOutput)
	case "executar":
		if len(args) >= 2 {
			return runPackage(profile, args[1], args[2:], replace)
		}
	case "remover":
		if len(args) == 2 {
			return removeFromProfile(profile, args[1])
		}
	case "catalogo":
		return catalogCommand(profile, args[1:])
	case "receita":
		return recipeCommand(profile, args[1:], build)
	case "shell":
		bin := filepath.Join(profile, "atual", "bin")
		shell := filepath.Join(bin, "nu")
		if _, err := os.Stat(shell); errors.Is(err, os.ErrNotExist) {
			shell = "/bin/sh"
		} else if err != nil {
			return err
		}
		searchPath := bin
		if inherited := os.Getenv("PATH"); inherited != "" {
			searchPath += string(os.PathListSeparator) + inherited
		}
		configDirs := os.Getenv("XDG_CONFIG_DIRS")
		if configDirs == "" {
			configDirs = "/etc/xdg"
		}
		configDirs = filepath.Join(profile, "atual", "config") + ":" + configDirs
		env := slices.DeleteFunc(os.Environ(), func(entry string) bool {
			return strings.HasPrefix(entry, "PATH=") || strings.HasPrefix(entry, "XDG_CONFIG_DIRS=")
		})
		return replace(shell, append([]string{shell}, args[1:]...), append(env, "PATH="+searchPath, "XDG_CONFIG_DIRS="+configDirs))
	}
	return errors.New(usage)
}

func catalogCommand(profile string, args []string) error {
	switch {
	case len(args) == 4 && args[0] == "sincronizar":
		catalog, err := syncCatalog(profile, args[1], args[2], args[3])
		if err == nil {
			fmt.Printf("Catálogo %s@%s ativado (%d receitas).\n", catalog.Tag, catalog.Commit, len(catalog.Recipes))
		}
		return err
	case len(args) == 2 && args[0] == "validar":
		total, err := validateRecipes(args[1])
		if err == nil {
			fmt.Printf("%d receitas válidas.\n", total)
		}
		return err
	}
	return errors.New("Uso: melkor catalogo sincronizar <origem> <tag> <chaves-ssh> | melkor catalogo validar <diretório>")
}

func recipeCommand(profile string, args []string, build func(string, recipe, string) error) error {
	switch {
	case len(args) == 3 && args[0] == "testar":
		dir, err := testRecipe(profile, args[1], args[2], build)
		if err == nil {
			fmt.Printf("Executáveis em %s\nPara testar os defaults, inclua %s em XDG_CONFIG_DIRS.\n"+
				"Depois de conferir, execute melkor receita aceitar %s\n",
				filepath.Join(dir, "bin"), filepath.Join(dir, "config"), filepath.Base(dir))
		}
		return err
	case len(args) == 2 && args[0] == "aceitar":
		name, err := acceptRecipe(profile, args[1])
		if err == nil {
			fmt.Printf("Receita %s salva no catálogo local. Use melkor instalar %s.\n", name, name)
		}
		return err
	case len(args) == 3 && args[0] == "publicar":
		return publishRecipe(profile, args[1], args[2])
	}
	return errors.New("Uso: melkor receita testar <nome> <arquivo> | melkor receita aceitar <teste> | melkor receita publicar <nome> <checkout>")
}

func profileDir() (string, error) {
	if profile := os.Getenv("MELKOR_PERFIL"); profile != "" {
		if !filepath.IsAbs(profile) {
			return "", errors.New("MELKOR_PERFIL precisa ser um caminho absoluto")
		}
		return filepath.Clean(profile), nil
	}
	root := os.Getenv("XDG_DATA_HOME")
	if !filepath.IsAbs(root) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		root = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(root, "melkor"), nil
}
