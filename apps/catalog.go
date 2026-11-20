package main

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"
)

//go:embed recipes/*.json
var builtinCatalog embed.FS

var namePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.+-]*$`)
var digestPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

type recipe struct {
	Version   string            `json:"version"`
	Origin    string            `json:"origin"`
	Source    string            `json:"source"`
	SHA256    string            `json:"sha256"`
	Image     string            `json:"image"`
	Toolchain string            `json:"toolchain"`
	Fetch     []string          `json:"fetch,omitempty"`
	Build     []string          `json:"build"`
	Files     map[string]string `json:"files"`
	Defaults  map[string]string `json:"defaults,omitempty"`
	Checks    []check           `json:"checks"`
}

type check struct {
	Run    []string `json:"run"`
	Stdout string   `json:"stdout"`
}

func lookup(name string) (recipe, error) {
	if !namePattern.MatchString(name) {
		return recipe{}, fmt.Errorf("nome de aplicativo inválido: %q", name)
	}
	catalog, dir := fs.FS(builtinCatalog), "recipes"
	if override := os.Getenv("MELKOR_CATALOGO"); override != "" {
		catalog, dir = os.DirFS(override), "."
	}
	data, err := fs.ReadFile(catalog, path.Join(dir, name+".json"))
	if errors.Is(err, fs.ErrNotExist) {
		return recipe{}, fmt.Errorf("%s: receita ausente no catálogo. Para testar uma proposta da IA, use melkor receita testar %s <arquivo>", name, name)
	}
	if err != nil {
		return recipe{}, err
	}
	return parseRecipe(name, data)
}

func parseRecipe(name string, data []byte) (recipe, error) {
	var app recipe
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&app); err != nil {
		return app, fmt.Errorf("receita %s: %w", name, err)
	}
	if err := decoder.Decode(new(json.RawMessage)); err != io.EOF {
		return app, fmt.Errorf("receita %s contém dados após o JSON", name)
	}
	if err := app.validate(); err != nil {
		return app, fmt.Errorf("receita %s: %w", name, err)
	}
	return app, nil
}

func resolveRecipe(profile, name string, catalog catalogSnapshot) (recipe, string, error) {
	if !namePattern.MatchString(name) || os.Getenv("MELKOR_CATALOGO") != "" {
		app, err := lookup(name)
		return app, "externo", err
	}
	if app, found := catalog.Recipes[name]; found {
		return app, catalog.Tag + "@" + catalog.Commit, app.validate()
	}
	app, err := localRecipe(profile, name)
	if !errors.Is(err, fs.ErrNotExist) {
		return app, "local", err
	}
	app, err = lookup(name)
	return app, "embutido", err
}

func (app recipe) validate() error {
	image, digest, pinned := strings.Cut(app.Image, "@sha256:")
	if app.Version == "" || app.Toolchain == "" || !validRecipeURL(app.Origin) ||
		!validRecipeURL(app.Source) || !digestPattern.MatchString(app.SHA256) ||
		image == "" || !pinned || !digestPattern.MatchString(digest) {
		return errors.New("informe origem, fonte HTTPS, versão, SHA-256, toolchain e imagem OCI fixada por digest")
	}
	if len(app.Build) == 0 || app.Build[0] == "" || len(app.Files) == 0 || len(app.Checks) == 0 {
		return errors.New("informe o comando de build, os arquivos e as verificações")
	}
	if len(app.Fetch) > 0 && app.Fetch[0] == "" {
		return errors.New("comando de obtenção de dependências inválido")
	}
	for name, file := range app.Files {
		if !namePattern.MatchString(name) || file == "." || !fs.ValidPath(file) {
			return fmt.Errorf("artefato inválido: %q → %q", name, file)
		}
	}
	for file := range app.Defaults {
		if !validConfigPath(file) {
			return fmt.Errorf("caminho de configuração inválido: %q", file)
		}
	}
	for _, check := range app.Checks {
		if len(check.Run) == 0 || app.Files[check.Run[0]] == "" {
			return errors.New("cada verificação deve executar um dos artefatos da receita")
		}
	}
	return nil
}

func validRecipeURL(value string) bool {
	addr, err := url.Parse(value)
	return err == nil && trustedSource(addr)
}

func validConfigPath(file string) bool {
	return file != "." && fs.ValidPath(file) && !strings.ContainsAny(file, "\x00\r\n\\")
}

func (app recipe) identity() (string, error) {
	data, err := json.Marshal(app)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
