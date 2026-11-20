package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type testedRecipe struct {
	Name   string `json:"name"`
	Recipe recipe `json:"recipe"`
}

func testRecipe(profile, name, path string, build func(string, recipe, string) error) (string, error) {
	if !namePattern.MatchString(name) {
		return "", fmt.Errorf("nome de aplicativo inválido: %q", name)
	}
	app, err := readRecipeFile(name, path)
	if err != nil {
		return "", err
	}
	dir := filepath.Join(profile, "catalogo", "testes")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	stage, err := os.MkdirTemp(dir, "teste-")
	if err != nil {
		return "", err
	}
	ok := false
	defer func() {
		if !ok {
			os.RemoveAll(stage)
		}
	}()
	bin := filepath.Join(stage, "bin")
	if err := os.Mkdir(bin, 0o700); err != nil {
		return "", err
	}
	if err := build(name, app, bin); err != nil {
		return "", err
	}
	if err := writeDefaults(filepath.Join(stage, "config"), app.Defaults); err != nil {
		return "", err
	}
	if err := writeJSONAtomic(filepath.Join(stage, "receita.json"), testedRecipe{Name: name, Recipe: app}); err != nil {
		return "", err
	}
	ok = true
	return stage, nil
}

func readRecipeFile(name, path string) (recipe, error) {
	file, err := os.Open(path)
	if err != nil {
		return recipe{}, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxRecipeSize+1))
	if err != nil {
		return recipe{}, err
	}
	if len(data) > maxRecipeSize {
		return recipe{}, fmt.Errorf("receita %s excede %d bytes", name, maxRecipeSize)
	}
	return parseRecipe(name, data)
}

func acceptRecipe(profile, id string) (string, error) {
	if !namePattern.MatchString(id) {
		return "", fmt.Errorf("identificador de teste inválido: %q", id)
	}
	data, err := os.ReadFile(filepath.Join(profile, "catalogo", "testes", id, "receita.json"))
	if err != nil {
		return "", err
	}
	var tested testedRecipe
	if err := json.Unmarshal(data, &tested); err != nil {
		return "", err
	}
	if !namePattern.MatchString(tested.Name) {
		return "", errors.New("teste sem nome de aplicativo válido")
	}
	if err := tested.Recipe.validate(); err != nil {
		return "", err
	}
	dir := filepath.Join(profile, "catalogo", "locais")
	return tested.Name, writeJSONAtomic(filepath.Join(dir, tested.Name+".json"), tested.Recipe)
}

func validateRecipes(dir string) (int, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}
	if len(files) == 0 {
		return 0, errors.New("catálogo vazio")
	}
	for _, file := range files {
		name := file.Name()
		app := name[:len(name)-len(filepath.Ext(name))]
		if !file.Type().IsRegular() || filepath.Ext(name) != ".json" || !namePattern.MatchString(app) {
			return 0, fmt.Errorf("entrada inválida em recipes/: %q", name)
		}
		if _, err := readRecipeFile(app, filepath.Join(dir, name)); err != nil {
			return 0, err
		}
	}
	return len(files), nil
}

func localRecipe(profile, name string) (recipe, error) {
	if !namePattern.MatchString(name) {
		return recipe{}, fmt.Errorf("nome de aplicativo inválido: %q", name)
	}
	return readRecipeFile(name, filepath.Join(profile, "catalogo", "locais", name+".json"))
}
