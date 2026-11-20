package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"
)

func publishRecipe(profile, name, checkout string) error {
	app, err := localRecipe(profile, name)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	origin, err := commandOutput(ctx, checkout, nil, "git", "config", "--get", "remote.origin.url")
	if err != nil {
		return err
	}
	repo, err := githubRepo(origin)
	if err != nil {
		return err
	}
	work, err := os.MkdirTemp("", "melkor-publicar-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	git := func(args ...string) (string, error) {
		return commandOutput(ctx, work, nil, "git", args...)
	}
	if _, err := git("init", "--quiet", "--template="); err != nil {
		return err
	}
	const base = "refs/heads/main"
	if _, err := git("fetch", "--no-tags", "--no-recurse-submodules", "--", origin, base+":"+base); err != nil {
		return err
	}
	if _, err := git("read-tree", base); err != nil {
		return err
	}
	data, err := json.MarshalIndent(app, "", "  ")
	if err != nil {
		return err
	}
	hash, err := app.identity()
	if err != nil {
		return err
	}
	blob, err := commandOutput(ctx, work, append(data, '\n'), "git", "hash-object", "-w", "--stdin")
	if err != nil {
		return err
	}
	path := "recipes/" + name + ".json"
	if _, err := git("update-index", "--add", "--cacheinfo", "100644,"+blob+","+path); err != nil {
		return err
	}
	tree, err := git("write-tree")
	if err != nil {
		return err
	}
	baseTree, err := git("rev-parse", "--verify", base+"^{tree}")
	if err != nil {
		return err
	}
	if tree == baseTree {
		fmt.Printf("Receita %s já está no catálogo.\n", name)
		return nil
	}
	branch := "melkor/receita-" + name + "-" + hash[:12]
	ref := "refs/heads/" + branch
	title := "Receita para " + name + " " + app.Version
	if err := ensureRecipeBranch(ctx, work, origin, ref, tree, base, path, blob, title); err != nil {
		return err
	}
	pr, err := commandOutput(ctx, work, nil, "gh", "pr", "view", branch, "--repo", repo, "--json", "url", "--jq", ".url")
	if err != nil {
		pr, err = commandOutput(ctx, work, nil, "gh", "pr", "create", "--repo", repo,
			"--head", branch, "--base", "main", "--title", title,
			"--body", "Receita validada no Melkor. A CI deve reproduzir o build e os testes da receita.")
	}
	if err != nil {
		return fmt.Errorf("a branch %s está no GitHub, mas o PR não abriu: %w", branch, err)
	}
	fmt.Println(pr)
	return nil
}

func ensureRecipeBranch(ctx context.Context, work, origin, ref, tree, base, path, blob, title string) error {
	git := func(args ...string) (string, error) {
		return commandOutput(ctx, work, nil, "git", args...)
	}
	_, err := git("ls-remote", "--exit-code", "--refs", "--", origin, ref)
	if err == nil {
		if _, err := git("fetch", "--no-tags", "--no-recurse-submodules", "--", origin, ref+":"+ref); err != nil {
			return err
		}
		files, err := git("diff", "--name-only", ref+"^", ref, "--")
		if err != nil {
			return err
		}
		existing, err := git("rev-parse", "--verify", ref+":"+path)
		_, ancestorErr := git("merge-base", "--is-ancestor", ref+"^", base)
		if files != path || existing != blob || err != nil || ancestorErr != nil {
			return fmt.Errorf("a branch %s já existe com conteúdo diferente; revise o PR existente", ref)
		}
		return nil
	}
	var failure *exec.ExitError
	if !errors.As(err, &failure) || failure.ExitCode() != 2 {
		return err
	}
	commit, err := git("commit-tree", tree, "-p", base, "-m", title)
	if err != nil {
		return err
	}
	_, err = git("push", "--", origin, commit+":"+ref)
	return err
}

func githubRepo(origin string) (string, error) {
	addr, err := url.Parse(strings.Replace(origin, "git@github.com:", "ssh://git@github.com/", 1))
	if err == nil && validCatalogOrigin(addr.String()) && addr.Host == "github.com" {
		slug := strings.TrimSuffix(strings.TrimPrefix(addr.Path, "/"), ".git")
		owner, repo, found := strings.Cut(slug, "/")
		if found && namePattern.MatchString(owner) && namePattern.MatchString(repo) {
			return slug, nil
		}
	}
	return "", errors.New("o checkout precisa ter origin em github.com/<usuário>/<repositório>")
}

func commandOutput(ctx context.Context, dir string, input []byte, program string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, program, args...)
	cmd.Dir = dir
	cmd.Stdin = bytes.NewReader(input)
	if program == "git" {
		cmd.Env = slices.DeleteFunc(os.Environ(), func(item string) bool {
			key, _, _ := strings.Cut(item, "=")
			switch key {
			case "GIT_DIR", "GIT_WORK_TREE", "GIT_COMMON_DIR", "GIT_INDEX_FILE", "GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES":
				return true
			}
			return false
		})
	}
	out, err := cmd.Output()
	if failure, ok := err.(*exec.ExitError); ok {
		return "", fmt.Errorf("%s %s: %w: %s", program, args[0], err, strings.TrimSpace(string(failure.Stderr)))
	}
	return strings.TrimSpace(string(out)), err
}
