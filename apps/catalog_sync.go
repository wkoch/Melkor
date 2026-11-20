package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

const maxRecipeSize = 128 << 10

type catalogSnapshot struct {
	Tag     string            `json:"tag"`
	Commit  string            `json:"commit"`
	Recipes map[string]recipe `json:"recipes"`
}

func readCatalog(profile string) (catalogSnapshot, error) {
	var catalog catalogSnapshot
	if os.Getenv("MELKOR_CATALOGO") != "" {
		return catalog, nil
	}
	data, err := os.ReadFile(filepath.Join(profile, "catalogo", "atual.json"))
	if errors.Is(err, os.ErrNotExist) {
		return catalog, nil
	}
	if err != nil {
		return catalog, err
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		return catalog, fmt.Errorf("catálogo sincronizado: %w", err)
	}
	if !namePattern.MatchString(catalog.Tag) || !validGitObjectID(catalog.Commit) || len(catalog.Recipes) == 0 {
		return catalog, errors.New("catálogo sincronizado sem versão, commit ou receitas válidas")
	}
	return catalog, nil
}

func syncCatalog(profile, origin, tag, trust string) (catalogSnapshot, error) {
	var catalog catalogSnapshot
	if !validCatalogOrigin(origin) {
		return catalog, errors.New("origem inválida: use HTTPS, ssh://git@github.com ou um caminho local absoluto")
	}
	if !namePattern.MatchString(tag) {
		return catalog, fmt.Errorf("tag inválida: %q", tag)
	}
	trust, err := filepath.Abs(trust)
	if err != nil {
		return catalog, err
	}
	info, err := os.Stat(trust)
	if err != nil || !info.Mode().IsRegular() {
		return catalog, fmt.Errorf("informe um arquivo local de chaves SSH autorizadas: %s", trust)
	}
	work, err := os.MkdirTemp("", "melkor-catalogo-")
	if err != nil {
		return catalog, err
	}
	defer os.RemoveAll(work)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	git := func(args ...string) (string, error) {
		options := []string{"-C", work, "-c", "protocol.allow=never", "-c", "core.hooksPath=/dev/null",
			"-c", "protocol.file.allow=always", "-c", "protocol.https.allow=always",
			"-c", "protocol.ssh.allow=always", "-c", "http.followRedirects=false",
			"-c", "fetch.fsckObjects=true", "-c", "gpg.format=ssh", "-c", "gpg.minTrustLevel=fully",
			"-c", "gpg.openpgp.program=/bin/false", "-c", "gpg.x509.program=/bin/false",
			"-c", "gpg.ssh.allowedSignersFile=" + trust}
		cmd := exec.CommandContext(ctx, "git", append(options, args...)...)
		env := slices.DeleteFunc(os.Environ(), func(item string) bool { return strings.HasPrefix(item, "GIT_") })
		cmd.Env = append(env, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null",
			"GIT_TERMINAL_PROMPT=0", "GIT_SSH_COMMAND=ssh -o BatchMode=yes -o StrictHostKeyChecking=yes")
		output, err := cmd.Output()
		if failure, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(string(failure.Stderr)))
		}
		return string(output), err
	}
	ref := "refs/tags/" + tag
	if _, err := git("init", "--bare", "--quiet", "--template="); err != nil {
		return catalog, err
	}
	if _, err := git("fetch", "--no-tags", "--no-recurse-submodules", "--depth=1", "--", origin, ref+":"+ref); err != nil {
		return catalog, err
	}
	if _, err := git("verify-tag", ref); err != nil {
		return catalog, fmt.Errorf("assinatura da tag %s: %w", tag, err)
	}
	object, err := git("cat-file", "tag", ref)
	if err != nil {
		return catalog, err
	}
	header, body, _ := strings.Cut(object, "\n\n")
	fields := strings.Split(header, "\n")
	if len(fields) < 3 || fields[1] != "type commit" || fields[2] != "tag "+tag ||
		!strings.HasSuffix(body, "-----END SSH SIGNATURE-----\n") {
		return catalog, fmt.Errorf("a tag assinada não corresponde à versão solicitada: %s", tag)
	}
	commit, err := git("rev-parse", "--verify", ref+"^{commit}")
	if err != nil {
		return catalog, err
	}
	catalog.Tag, catalog.Commit = tag, strings.TrimSpace(commit)
	tree, err := git("ls-tree", "-l", "-z", catalog.Commit+":recipes")
	if err != nil {
		return catalog, err
	}
	catalog.Recipes = make(map[string]recipe)
	for _, item := range strings.Split(strings.TrimSuffix(tree, "\x00"), "\x00") {
		meta, filename, _ := strings.Cut(item, "\t")
		fields := strings.Fields(meta)
		name, jsonFile := strings.CutSuffix(filename, ".json")
		if len(fields) != 4 || fields[0] != "100644" || fields[1] != "blob" ||
			!jsonFile || !namePattern.MatchString(name) {
			return catalog, fmt.Errorf("entrada inválida em recipes/: %q", filename)
		}
		size, err := strconv.Atoi(fields[3])
		if err != nil || size > maxRecipeSize {
			return catalog, fmt.Errorf("receita %s excede %d bytes", name, maxRecipeSize)
		}
		data, err := git("cat-file", "blob", fields[2])
		if err != nil {
			return catalog, err
		}
		app, err := parseRecipe(name, []byte(data))
		if err != nil {
			return catalog, err
		}
		catalog.Recipes[name] = app
	}
	return catalog, writeJSONAtomic(filepath.Join(profile, "catalogo", "atual.json"), catalog)
}

func validCatalogOrigin(origin string) bool {
	if filepath.IsAbs(origin) {
		return true
	}
	addr, err := url.Parse(origin)
	if err != nil || addr.Hostname() == "" || addr.RawQuery != "" || addr.Fragment != "" {
		return false
	}
	return trustedSource(addr) || (addr.Scheme == "ssh" && addr.Host == "github.com" &&
		addr.User != nil && addr.User.String() == "git")
}

func validGitObjectID(value string) bool {
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func writeJSONAtomic(path string, value any) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	file, err := os.CreateTemp(dir, ".melkor-")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	enc := json.NewEncoder(file)
	enc.SetIndent("", "  ")
	err = enc.Encode(value)
	if err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err := os.Rename(file.Name(), path); err != nil {
		return err
	}
	return syncDir(dir)
}
