package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
)

type installed struct {
	Version string         `json:"version"`
	Recipe  string         `json:"recipe_sha256,omitempty"`
	Catalog string         `json:"catalogo,omitempty"`
	Files   []string       `json:"files"`
	Config  []string       `json:"config,omitempty"`
	Binary  *binaryPackage `json:"pacote,omitempty"`
}

type profileActions map[string]func(string) error

func update(profile, requested string, build func(string, recipe, string) error) error {
	_, err := changeProfile(profile, func(packages map[string]installed) (profileActions, error) {
		names := []string{requested}
		if requested == "" {
			names = make([]string, 0, len(packages))
			for name, app := range packages {
				if app.Binary == nil {
					names = append(names, name)
				}
			}
			slices.Sort(names)
		}
		changes := make(profileActions)
		catalog, err := readCatalog(profile)
		if err != nil {
			return nil, err
		}
		for _, name := range names {
			if packages[name].Binary != nil {
				return nil, fmt.Errorf("%s já está instalado como pacote importado", name)
			}
			app, catalogRef, err := resolveRecipe(profile, name, catalog)
			if err != nil {
				return nil, err
			}
			hash, err := app.identity()
			if err != nil {
				return nil, err
			}
			if packages[name].Recipe == hash {
				continue
			}
			files := make([]string, 0, len(app.Files))
			for file := range app.Files {
				files = append(files, file)
			}
			slices.Sort(files)
			config := make([]string, 0, len(app.Defaults))
			for file := range app.Defaults {
				config = append(config, file)
			}
			slices.Sort(config)
			changes[name] = func(stage string) error {
				if err := build(name, app, filepath.Join(stage, "bin")); err != nil {
					return fmt.Errorf("%s %s: %w", name, app.Version, err)
				}
				return writeDefaults(filepath.Join(stage, "config"), app.Defaults)
			}
			packages[name] = installed{Version: app.Version, Recipe: hash, Catalog: catalogRef, Files: files, Config: config}
		}
		return changes, nil
	})
	return err
}

func changeProfile(profile string, plan func(map[string]installed) (profileActions, error)) (bool, error) {
	generations := filepath.Join(profile, "geracoes")
	if err := os.MkdirAll(generations, 0o700); err != nil {
		return false, err
	}
	lock, err := os.OpenFile(filepath.Join(profile, ".lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return false, err
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return false, err
	}

	current, packages, err := readProfile(profile)
	if err != nil {
		return false, err
	}
	changes, err := plan(packages)
	if err != nil {
		return false, err
	}
	return publishProfile(profile, generations, current, packages, changes)
}

func publishProfile(profile, generations, current string, packages map[string]installed, changes profileActions) (bool, error) {
	if len(changes) == 0 {
		fmt.Println("Perfil já corresponde ao catálogo.")
		return false, nil
	}
	if err := checkOwners(packages); err != nil {
		return false, err
	}

	stage, err := os.MkdirTemp(generations, ".candidata-")
	if err != nil {
		return false, err
	}
	defer func() {
		if stage != "" {
			os.RemoveAll(stage)
		}
	}()
	bin := filepath.Join(stage, "bin")
	if err := os.Mkdir(bin, 0o755); err != nil {
		return false, err
	}
	for name, app := range packages {
		if _, rebuild := changes[name]; rebuild {
			continue
		}
		for _, file := range app.Files {
			if err := os.Link(filepath.Join(current, "bin", file), filepath.Join(bin, file)); err != nil {
				return false, err
			}
		}
		for _, file := range app.Config {
			target := filepath.Join(stage, "config", file)
			if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
				return false, err
			}
			if err := os.Link(filepath.Join(current, "config", file), target); err != nil {
				return false, err
			}
		}
	}
	for _, name := range slices.Sorted(maps.Keys(changes)) {
		if action := changes[name]; action != nil {
			if err := action(stage); err != nil {
				return false, err
			}
		}
	}
	if err := writeProfile(stage, packages); err != nil {
		return false, err
	}
	generation := strings.Replace(filepath.Base(stage), ".candidata-", "g-", 1)
	published := filepath.Join(generations, generation)
	if err := syncDir(bin); err != nil {
		return false, err
	}
	if err := syncConfig(filepath.Join(stage, "config")); err != nil {
		return false, err
	}
	if err := syncDir(stage); err != nil {
		return false, err
	}
	if err := os.Rename(stage, published); err != nil {
		return false, err
	}
	stage = published
	if err := syncDir(generations); err != nil {
		return false, err
	}
	link := filepath.Join(profile, "."+generation)
	if err := os.Symlink(filepath.Join("geracoes", generation), link); err != nil {
		return false, err
	}
	defer os.Remove(link)
	if err := os.Rename(link, filepath.Join(profile, "atual")); err != nil {
		return false, err
	}
	stage = ""
	fmt.Printf("Geração %s ativada.\n", generation)
	return true, syncDir(profile)
}

func readProfile(profile string) (string, map[string]installed, error) {
	target, err := os.Readlink(filepath.Join(profile, "atual"))
	if errors.Is(err, os.ErrNotExist) {
		return "", make(map[string]installed), nil
	}
	if err != nil {
		return "", nil, err
	}
	name := filepath.Base(target)
	if !strings.HasPrefix(name, "g-") || !namePattern.MatchString(name) ||
		target != filepath.Join("geracoes", name) {
		return "", nil, fmt.Errorf("geração ativa inválida: %q", target)
	}
	current := filepath.Join(profile, target)
	data, err := os.ReadFile(filepath.Join(current, "manifest.json"))
	if err != nil {
		return "", nil, err
	}
	var packages map[string]installed
	if err := json.Unmarshal(data, &packages); err != nil {
		return "", nil, err
	}
	if packages == nil {
		return "", nil, errors.New("manifesto do perfil inválido")
	}
	return current, packages, nil
}

func removeFromProfile(profile, name string) error {
	_, err := changeProfile(profile, func(packages map[string]installed) (profileActions, error) {
		if _, exists := packages[name]; !exists {
			return nil, fmt.Errorf("%s não está instalado no perfil", name)
		}
		delete(packages, name)
		return profileActions{name: nil}, nil
	})
	return err
}

func checkOwners(packages map[string]installed) error {
	owners := make(map[string]string)
	configOwners := make(map[string]string)
	for name, app := range packages {
		for _, file := range app.Files {
			if !namePattern.MatchString(file) {
				return fmt.Errorf("arquivo inválido no perfil de %s: %q", name, file)
			}
			if other, exists := owners[file]; exists {
				return fmt.Errorf("%s e %s declaram o mesmo executável: %s", other, name, file)
			}
			owners[file] = name
		}
		for _, file := range app.Config {
			if !validConfigPath(file) {
				return fmt.Errorf("caminho de configuração inválido no perfil de %s: %q", name, file)
			}
			if other, exists := configOwners[file]; exists {
				return fmt.Errorf("%s e %s declaram a mesma configuração: %s", other, name, file)
			}
			configOwners[file] = name
		}
	}
	return nil
}

func writeProfile(dir string, packages map[string]installed) error {
	file, err := os.OpenFile(filepath.Join(dir, "manifest.json"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(packages)
	if err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	return err
}

func syncDir(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
