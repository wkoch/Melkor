package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"
)

const (
	debianPackageBase = "docker.io/library/python@sha256:d98cc29d3266e535dd9da1099688d9cb3ca4753813571e071763d17725460962"
	fedoraPackageBase = "registry.fedoraproject.org/fedora@sha256:130bdf7ec68a9834ae377c574c3f22992ae103197ef951deaac48e1e1eb7cf2d"

	packageUsage = "Uso: melkor importar [--chave-rpm=<arquivo>] <nome> <pacote.deb|pacote.rpm> <sha256> </executavel> [argumentos de teste]"
)

var (
	packageHashPattern  = regexp.MustCompile(`^[a-f0-9]{64}$`)
	packageFieldPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9.+_~:-]{0,127}$`)
	packageEntryPattern = regexp.MustCompile(`^/[a-zA-Z0-9/._+-]{1,511}$`)
)

type binaryPackage struct {
	Format    string `json:"formato"`
	Name      string `json:"nome"`
	Arch      string `json:"arquitetura"`
	SourceSHA string `json:"sha256"`
	KeySHA    string `json:"chave_rpm_sha256,omitempty"`
	Base      string `json:"base"`
	Image     string `json:"imagem"`
	Tag       string `json:"tag_local"`
	Entry     string `json:"executavel"`
}

type packageBackend func(context.Context, ...string) (string, error)

func importPackage(profile string, args []string, podman packageBackend) error {
	var key string
	if len(args) > 0 && strings.HasPrefix(args[0], "--chave-rpm=") {
		key = strings.TrimPrefix(args[0], "--chave-rpm=")
		if key == "" {
			return errors.New(packageUsage)
		}
		args = args[1:]
	}
	if len(args) < 4 || !namePattern.MatchString(args[0]) || !packageHashPattern.MatchString(args[2]) ||
		!validPackageEntry(args[3]) {
		return errors.New(packageUsage)
	}

	name, source := args[0], args[1]
	format := strings.ToLower(filepath.Ext(source))
	var base string
	switch format {
	case ".deb":
		if key != "" {
			return errors.New("--chave-rpm exige um arquivo RPM")
		}
		base = debianPackageBase
	case ".rpm":
		base = fedoraPackageBase
	default:
		return errors.New("use um arquivo local .deb ou .rpm")
	}

	work, err := os.MkdirTemp("", "melkor-pacote-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	buildDir := filepath.Join(work, "contexto")
	if err := os.Mkdir(buildDir, 0o700); err != nil {
		return err
	}
	digest, err := stagePackageFile(source, filepath.Join(buildDir, "pacote"+format), maxSourceBytes)
	if err != nil {
		return fmt.Errorf("ler pacote local: %w", err)
	}
	if digest != args[2] {
		return fmt.Errorf("SHA-256 do pacote divergente: %s", digest)
	}
	app := &binaryPackage{Format: format[1:], SourceSHA: digest, Base: base, Entry: args[3]}
	if key != "" {
		app.KeySHA, err = stagePackageFile(key, filepath.Join(buildDir, "chave.asc"), 1<<20)
		if err != nil {
			return fmt.Errorf("ler chave RPM: %w", err)
		}
	}
	suffix := strings.TrimPrefix(filepath.Base(work), "melkor-pacote-")
	app.Tag = "localhost/melkor-pacote:app-" + suffix
	prepared := "localhost/melkor-pacote:deps-" + suffix

	activated, err := changeProfile(profile, func(packages map[string]installed) (profileActions, error) {
		if current, exists := packages[name]; exists && current.Binary == nil {
			return nil, fmt.Errorf("%s já está instalado por receita", name)
		}

		return profileActions{name: func(_ string) error {
			ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
			defer cancel()
			version, err := buildPackage(ctx, podman, work, buildDir, prepared, app, args[4:])
			if err != nil {
				return err
			}
			packages[name] = installed{Version: version, Files: []string{}, Binary: app}
			return nil
		}}, nil
	})
	if !activated && app.Image != "" {
		removePackageImage(podman, app.Tag)
	}
	return err
}

func stagePackageFile(source, dest string, limit int64) (string, error) {
	in, err := os.OpenFile(source, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return "", err
	}
	defer in.Close()
	fi, err := in.Stat()
	if err != nil {
		return "", err
	}
	if !fi.Mode().IsRegular() || fi.Size() == 0 || fi.Size() > limit {
		return "", errors.New("esperado um arquivo regular não vazio dentro do limite de tamanho")
	}
	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return "", err
	}
	hash := sha256.New()
	written, err := io.Copy(io.MultiWriter(out, hash), io.LimitReader(in, limit+1))
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return "", err
	}
	if written == 0 || written > limit {
		return "", errors.New("tamanho do arquivo mudou durante a cópia")
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func validPackageEntry(entry string) bool {
	return packageEntryPattern.MatchString(entry) && path.Clean(entry) == entry
}

func buildPackage(ctx context.Context, podman packageBackend, work, dir, prepared string, app *binaryPackage, testArgs []string) (string, error) {
	rootless, err := podman(ctx, "info", "--format={{.Host.Security.Rootless}}")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(rootless) != "true" {
		return "", errors.New("a importação exige Podman rootless")
	}
	fetchFile, installFile := packageContainerfiles(app, prepared)
	buildImage := func(label, network, pull, recipe string) (string, error) {
		file := filepath.Join(work, "Containerfile")
		iid := filepath.Join(work, filepath.Base(label)+".id")
		if err := os.WriteFile(file, []byte(recipe), 0o600); err != nil {
			return "", err
		}
		argv := []string{"build", "--network=" + network, "--pull=" + pull, "--tag=" + label, "--file=" + file,
			"--iidfile=" + iid, "--http-proxy=false", "--no-hosts", "--layers=false", "--force-rm", "--quiet",
			"--platform=linux/amd64", dir}
		if _, err := podman(ctx, argv...); err != nil {
			return "", err
		}
		raw, err := os.ReadFile(iid)
		if err != nil {
			return "", err
		}
		image := strings.TrimSpace(string(raw))
		if !validPackageImage(image) {
			return "", errors.New("Podman devolveu um identificador de imagem inválido")
		}
		return image, nil
	}

	defer removePackageImage(podman, prepared)
	cache, err := buildImage(prepared, "private", "missing", fetchFile)
	if err != nil {
		return "", fmt.Errorf("obter dependências do pacote: %w", err)
	}
	metadata, err := podman(ctx, packageRun(cache, "/bin/cat", "", "/melkor-metadados")...)
	if err != nil {
		return "", err
	}
	name, version, arch, err := packageMetadata(metadata, app.Format)
	if err != nil {
		return "", err
	}
	app.Name, app.Arch = name, arch
	app.Image, err = buildImage(app.Tag, "none", "never", installFile)
	if err != nil {
		removePackageImage(podman, app.Tag)
		return "", fmt.Errorf("instalar o pacote sem rede: %w", err)
	}
	check := packageRun(app.Image, "/bin/sh", "", "-ec", `test -f "$1" && test -x "$1"`, "melkor", app.Entry)
	if len(testArgs) > 0 {
		check = packageRun(app.Image, app.Entry, "", testArgs...)
	}
	if _, err := podman(ctx, check...); err != nil {
		return "", fmt.Errorf("verificar executável da candidata: %w", err)
	}
	return version, nil
}

func packageMetadata(metadata, format string) (string, string, string, error) {
	fields := strings.Split(strings.TrimSpace(metadata), "\n")
	if len(fields) != 3 || !packageFieldPattern.MatchString(fields[0]) || !packageFieldPattern.MatchString(fields[1]) {
		return "", "", "", errors.New("metadados do pacote inválidos")
	}
	arch := fields[2]
	if (format == "deb" && arch != "amd64" && arch != "all") || (format == "rpm" && arch != "x86_64" && arch != "noarch") {
		return "", "", "", fmt.Errorf("arquitetura de pacote incompatível: %q", arch)
	}
	return fields[0], fields[1], arch, nil
}

func validPackageImage(image string) bool {
	digest, ok := strings.CutPrefix(image, "sha256:")
	return ok && packageHashPattern.MatchString(digest)
}

func removePackageImage(podman packageBackend, tag string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	podman(ctx, "image", "rm", "--ignore", tag)
}

func packageRun(image, entry, data string, args ...string) []string {
	cmd := []string{"run", "--rm", "--pull=never", "--userns=keep-id:uid=1000,gid=1000",
		"--user=1000:1000", "--cap-drop=all", "--security-opt=no-new-privileges", "--network=none",
		"--image-volume=ignore", "--read-only", "--pids-limit=256", "--http-proxy=false", "--no-hosts",
		"--env=HOME=/home/melkor", "--workdir=/home/melkor"}
	if data == "" {
		cmd = append(cmd, "--tmpfs=/home/melkor:rw,nosuid,nodev,mode=1777")
	} else {
		cmd = append(cmd, "--interactive", "--volume="+data+":/home/melkor:z")
	}
	return append(append(cmd, "--entrypoint="+entry, image), args...)
}

func runPackage(profile, name string, args []string, replace func(string, []string, []string) error) error {
	if !namePattern.MatchString(name) || os.Geteuid() == 0 {
		return errors.New("use o nome do pacote em uma sessão de usuário comum")
	}
	_, packages, err := readProfile(profile)
	if err != nil {
		return err
	}
	app := packages[name].Binary
	if app == nil || !validPackageImage(app.Image) || !validPackageEntry(app.Entry) {
		return fmt.Errorf("pacote importado indisponível no perfil: %s", name)
	}
	data, err := packageDataDir(profile, name)
	if err != nil {
		return err
	}
	program, err := exec.LookPath("podman")
	if err != nil {
		return err
	}
	argv := append([]string{program, "--remote=false"}, packageRun(app.Image, app.Entry, data, args...)...)
	return replace(program, argv, os.Environ())
}

func packageDataDir(profile, name string) (string, error) {
	base, err := filepath.EvalSymlinks(profile)
	if err != nil {
		return "", err
	}
	parent := filepath.Join(base, "dados")
	data := filepath.Join(parent, name)
	if strings.ContainsAny(data, ":\r\n") {
		return "", errors.New("o caminho de dados contém um separador de volume")
	}
	for _, dir := range []string{parent, data} {
		if err := os.Mkdir(dir, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
			return "", err
		}
		info, err := os.Lstat(dir)
		if err != nil || !info.IsDir() {
			return "", errors.New("o diretório de dados precisa ser um caminho local sem links simbólicos")
		}
	}
	return data, nil
}

func packageOutput(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "podman", append([]string{"--remote=false"}, args...)...)
	var stdout, stderr shortOutput
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("podman %s: %w, saída %q %q", args[0], err, stdout.String(), stderr.String())
	}
	return stdout.String(), nil
}

// Limita a saída de instaladores antes de exibi-la entre aspas.
type shortOutput struct{ content bytes.Buffer }

func (b *shortOutput) String() string { return b.content.String() }

func (b *shortOutput) Write(data []byte) (int, error) {
	const maximum = 8192
	b.content.Write(data[:min(len(data), maximum-b.content.Len())])
	return len(data), nil
}

func packageContainerfiles(app *binaryPackage, prepared string) (string, string) {
	base := "FROM " + app.Base + "\nUSER 0\nWORKDIR /\n"
	offline := "FROM " + prepared + " AS dependencias\n" + base
	if app.Format == "deb" {
		const input = "COPY pacote.deb /tmp/pacote.deb\n"
		return base + input + `ENV DEBIAN_FRONTEND=noninteractive
RUN set -eu; \
    case "$(dpkg-deb -f /tmp/pacote.deb Architecture)" in amd64|all) ;; *) exit 1;; esac; \
    for campo in Package Version Architecture; do dpkg-deb -f /tmp/pacote.deb "$campo"; done > /melkor-metadados; \
    sed -i 's@http://deb.debian.org@https://deb.debian.org@g' /etc/apt/sources.list.d/debian.sources; \
    apt-get update; \
    apt-get -y --no-install-recommends --download-only install /tmp/pacote.deb
`, offline + input + `COPY --from=dependencias /var/cache/apt/archives/ /tmp/melkor-deps/
RUN set -eu; \
    printf '#!/bin/sh\nexit 101\n' > /usr/sbin/policy-rc.d; chmod 0755 /usr/sbin/policy-rc.d; \
    set -- /tmp/pacote.deb; \
    for dep in /tmp/melkor-deps/*.deb; do if [ -f "$dep" ]; then set -- "$@" "$dep"; fi; done; \
    DEBIAN_FRONTEND=noninteractive apt-get -y --no-install-recommends \
        -o Dir::Etc::sourcelist=/dev/null -o Dir::Etc::sourceparts=/dev/null install "$@"; \
    rm -rf /tmp/pacote.deb /tmp/melkor-deps /var/cache/apt/archives/* /var/lib/apt/lists/*
`
	}
	input := "COPY pacote.rpm /tmp/pacote.rpm\n"
	if app.KeySHA != "" {
		input += "COPY chave.asc /tmp/chave.asc\nRUN rpmkeys --import /tmp/chave.asc && rm /tmp/chave.asc\n"
	}
	return base + input + `RUN set -eu; \
    rpm -qp --queryformat '%{NAME}\n%{EVR}\n%{ARCH}\n' /tmp/pacote.rpm > /melkor-metadados; \
    case "$(rpm -qp --queryformat '%{ARCH}' /tmp/pacote.rpm)" in x86_64|noarch) ;; *) exit 1;; esac; \
    mkdir /melkor-deps; \
    dnf -y --setopt=localpkg_gpgcheck=1 --setopt=pkg_gpgcheck=1 --setopt=install_weak_deps=False \
        --setopt=system_cachedir=/tmp/melkor-cache --setopt=keepcache=True \
        install --downloadonly /tmp/pacote.rpm; \
    find /tmp/melkor-cache -type f -name '*.rpm' -exec cp -t /melkor-deps -- {} +; \
    rm -f /melkor-deps/pacote.rpm
`, offline + input + `COPY --from=dependencias /melkor-deps/ /tmp/melkor-deps/
RUN set -eu; \
    set -- /tmp/pacote.rpm; \
    for dep in /tmp/melkor-deps/*.rpm; do if [ -f "$dep" ]; then set -- "$@" "$dep"; fi; done; \
    dnf -y --disablerepo='*' --setopt=localpkg_gpgcheck=1 --setopt=install_weak_deps=False install "$@"; \
    rm -rf /tmp/pacote.rpm /tmp/melkor-deps
`
}
