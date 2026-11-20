package main

import (
	"crypto/sha256"
	"debug/elf"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"
)

const maxSourceBytes = 512 << 20

func build(name string, app recipe, bin string) error {
	if os.Geteuid() == 0 {
		return fmt.Errorf("execute o build como usuário comum com Podman rootless")
	}
	return buildRecipe(name, app, bin, http.DefaultTransport)
}

func buildRecipe(name string, app recipe, bin string, transport http.RoundTripper) error {
	cache, err := os.UserCacheDir()
	if err != nil {
		return err
	}
	cache = filepath.Join(cache, "melkor")
	if err := os.MkdirAll(cache, 0o700); err != nil {
		return err
	}
	work, err := os.MkdirTemp(cache, "build-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)

	fmt.Fprintf(os.Stderr, "Construindo %s %s (%s)\n", name, app.Version, app.Toolchain)
	for _, dir := range []string{"src", "home", "cargo"} {
		if err := os.Mkdir(filepath.Join(work, dir), 0o700); err != nil {
			return err
		}
	}
	if err := writeDefaults(filepath.Join(work, "config"), app.Defaults); err != nil {
		return err
	}
	if err := download(transport, app.Source, app.SHA256, filepath.Join(work, "source.tar.gz"), maxSourceBytes); err != nil {
		return err
	}

	run := func(network string, argv []string) error {
		cmd := inContainer(app.Image, network, work, argv)
		cmd.Stdout = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%s: %w", argv[0], err)
		}
		return nil
	}
	if err := run("none", []string{"tar", "-xzf", "/work/source.tar.gz", "--strip-components=1"}); err != nil {
		return err
	}
	if len(app.Fetch) > 0 {
		if err := run("private", app.Fetch); err != nil {
			return fmt.Errorf("obter dependências: %w", err)
		}
	}
	if err := run("none", app.Build); err != nil {
		return fmt.Errorf("build: %w", err)
	}
	for _, check := range app.Checks {
		executable := path.Join("/work/src", app.Files[check.Run[0]])
		cmd := inContainer(app.Image, "none", work, append([]string{executable}, check.Run[1:]...))
		output, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("verificação %q: %w", check.Run, err)
		}
		if got := strings.TrimSpace(string(output)); got != check.Stdout {
			return fmt.Errorf("verificação %q: saiu %q, esperado %q", check.Run, got, check.Stdout)
		}
	}

	src, err := os.OpenRoot(filepath.Join(work, "src"))
	if err != nil {
		return err
	}
	defer src.Close()
	for filename, file := range app.Files {
		if err := stageExecutable(src, file, filepath.Join(bin, filename)); err != nil {
			return fmt.Errorf("artefato %s: %w", filename, err)
		}
	}
	return nil
}

func inContainer(image, network, work string, argv []string) *exec.Cmd {
	cmd := exec.Command("podman", "run", "--rm", "--pull=missing",
		"--userns=keep-id", "--cap-drop=all", "--security-opt=no-new-privileges",
		"--read-only", "--image-volume=ignore", "--network="+network,
		"--volume="+work+":/work:Z", "--workdir=/work/src",
		"--env=HOME=/work/home", "--env=CARGO_HOME=/work/cargo",
		"--env=XDG_CONFIG_HOME=/work/home/.config", "--env=XDG_CONFIG_DIRS=/work/config",
		"--http-proxy=false", "--entrypoint="+argv[0], image)
	cmd.Args = append(cmd.Args, argv[1:]...)
	cmd.Stderr = os.Stderr
	return cmd
}

func trustedSource(addr *url.URL) bool {
	return addr.Scheme == "https" && addr.Hostname() != "" && addr.User == nil
}

func download(transport http.RoundTripper, source, digest, file string, limit int64) error {
	addr, err := url.Parse(source)
	if err != nil || !trustedSource(addr) {
		return fmt.Errorf("fonte inválida: informe HTTPS sem credenciais")
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   5 * time.Minute,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if !trustedSource(req.URL) || len(via) >= 10 {
				return fmt.Errorf("redirecionamento de fonte inválido")
			}
			return nil
		},
	}
	resp, err := client.Get(addr.String())
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("baixar %s: HTTP %d", source, resp.StatusCode)
	}
	if resp.ContentLength > limit {
		return fmt.Errorf("fonte excede o limite de %d bytes", limit)
	}
	out, err := os.Create(file)
	if err != nil {
		return err
	}
	hash := sha256.New()
	written, err := io.Copy(io.MultiWriter(out, hash), io.LimitReader(resp.Body, limit+1))
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if written > limit {
		return fmt.Errorf("fonte excede o limite de %d bytes", limit)
	}
	if actual := fmt.Sprintf("%x", hash.Sum(nil)); actual != digest {
		return fmt.Errorf("SHA-256 de %s: obtido %s, esperado %s", source, actual, digest)
	}
	return nil
}

func stageExecutable(src *os.Root, name, dest string) error {
	info, err := src.Lstat(name)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Mode()&0o111 == 0 {
		return fmt.Errorf("%s precisa ser um executável regular", name)
	}
	file, err := src.Open(name)
	if err != nil {
		return err
	}
	defer file.Close()
	if err := verifyELF(file); err != nil {
		return err
	}
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, file)
	if err == nil {
		err = out.Sync()
	}
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	return err
}

func verifyELF(file *os.File) error {
	binary, err := elf.NewFile(file)
	if err != nil {
		return err
	}
	if binary.Machine != elf.EM_X86_64 || binary.Class != elf.ELFCLASS64 ||
		(binary.Type != elf.ET_EXEC && binary.Type != elf.ET_DYN) {
		return fmt.Errorf("%s: esperado um ELF executável x86-64", file.Name())
	}
	for _, program := range binary.Progs {
		if program.Type == elf.PT_INTERP {
			return fmt.Errorf("%s: ELF contém um carregador dinâmico", file.Name())
		}
	}
	libs, err := binary.ImportedLibraries()
	if err != nil {
		return err
	}
	if len(libs) > 0 {
		return fmt.Errorf("%s: bibliotecas dinâmicas %v", file.Name(), libs)
	}
	return nil
}
