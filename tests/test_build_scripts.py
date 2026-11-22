import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
from tempfile import TemporaryDirectory
import unittest


ROOT = Path(__file__).resolve().parents[1]


class BuildScriptsTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        work = TemporaryDirectory()
        cls.addClassCleanup(work.cleanup)
        cls.work = Path(work.name)
        cls.project = cls.work / "melkor"
        cls.scripts = cls.project / "scripts"
        cls.scripts.mkdir(parents=True)
        for name in ("build-image.sh", "prepare-env.sh", "build-packages.fedora"):
            shutil.copy2(ROOT / "scripts" / name, cls.scripts)
        shutil.copytree(ROOT / "core" / "board", cls.project / "core" / "board", symlinks=True)
        shutil.copytree(ROOT / "core" / "configs", cls.project / "core" / "configs")
        cls.stub = cls.work / "tool_stub.py"
        shutil.copyfile(ROOT / "tests/tool_stub.py", cls.stub)
        cls.stub.chmod(0o755)
        cls.bin = cls.work / "bin"
        cls.bin.mkdir()
        for name in ("git", "make", "rpm", "sudo", "distrobox", "mke2fs", "genimage", "grub-script-check",
                     "grub-editenv", "openssl", "xorriso"):
            (cls.bin / name).symlink_to(cls.stub)
        cls.marker = cls.work / "container"
        cls.marker.touch()
        cls.prepare = cls.wrapper("prepare", f'source "{cls.scripts / "prepare-env.sh"}"\nprepare_env "$@"\n')
        cls.update = cls.wrapper("update", """id() { printf '%s\n' "$TEST_UID"; }
test() {
    if [[ $* = "-x /usr/bin/melkor-core-fetch" ]]; then [[ ${TEST_CORE_FETCH:-} = yes ]]; else builtin test "$@"; fi
}
exec() { printf '%s\n' "$*" >> "$TEST_BOOT_LOG"; exit 0; }
script=$1
shift
source "$script" "$@"
""")
        cls.app = cls.wrapper("app", """test() {
    case "$1" in
        -x|-S) [[ ${TEST_APP:-} = yes && "$*" != "-S /run/session/pulse/native" ]] || [[ ${TEST_PULSE:-} = yes ]] ;;
        *) builtin test "$@" ;;
    esac
}
exec() { printf '%s\n' "$*" > "$TEST_BOOT_LOG"; exit 0; }
script=$1
shift
BASH_ARGV0=$script
source "$script" "$@"
""")
        cls.boot = cls.wrapper("boot", """mount() {
    printf 'mount %s\n' "$*" >> "$TEST_BOOT_LOG"
    return "${TEST_MOUNT_CODE:-0}"
}
hostname() { printf 'hostname %s\n' "$*" >> "$TEST_BOOT_LOG"; }
cat() {
    if [[ $1 = /proc/cmdline ]]; then printf '%s\n' "${TEST_CMDLINE:-}"; else command cat "$@"; fi
}
melkor-storage() {
    printf 'melkor-storage %s\n' "$*" >> "$TEST_BOOT_LOG"
    return "${TEST_CREDENTIALS_CODE:-0}"
}
exec() { printf '%s\n' "$*" >> "$TEST_BOOT_LOG"; }
install() { printf 'install %s\n' "$*" >> "$TEST_BOOT_LOG"; }
mkdir() { printf 'mkdir %s\n' "$*" >> "$TEST_BOOT_LOG"; }
ln() { printf 'ln %s\n' "$*" >> "$TEST_BOOT_LOG"; }
dbus-uuidgen() {
    printf 'dbus-uuidgen %s\n' "$*" >> "$TEST_BOOT_LOG"
    return "${TEST_DBUS_UUID_CODE:-0}"
}
mktemp() { printf '/tmp/melkor-simulado\n'; }
fallocate() {
    printf 'fallocate %s\n' "$*" >> "$TEST_BOOT_LOG"
    return "${TEST_FALLOCATE_CODE:-0}"
}
mkswap() { printf 'mkswap %s\n' "$*" >> "$TEST_BOOT_LOG"; }
swapon() { printf 'swapon %s\n' "$*" >> "$TEST_BOOT_LOG"; }
mv() { printf 'mv %s\n' "$*" >> "$TEST_BOOT_LOG"; }
rm() { printf 'rm %s\n' "$*" >> "$TEST_BOOT_LOG"; }
dbus-run-session() { printf 'dbus-run-session %s %s %s\n' "$XDG_RUNTIME_DIR" "$LIBSEAT_BACKEND" "$*" >> "$TEST_BOOT_LOG"; }
blkid() {
    case "$4" in
        PARTUUID=9327*) printf '/dev/vda1\n' ;;
        PARTUUID=5ec7*) printf '/dev/vda2\n' ;;
        PARTUUID=0c7a*) printf '/dev/vda3\n'; if [[ ${TEST_AMBIGUOUS:-} ]]; then printf '/dev/vdb3\n'; fi ;;
        *) return 1 ;;
    esac
}
test() {
    case "$1" in
        -b) [[ $2 =~ ^/dev/vda[1-3]$ ]] ;;
        -x) if [[ $2 = /opt/melkor-desktop/usr/bin/umbriel ]]; then [[ ${TEST_UMBRIEL:-} ]]; else builtin test "$@"; fi ;;
        *) builtin test "$@" ;;
    esac
}
script=$1
shift
source "$script" "$@"
""")
        cls.session = cls.wrapper("session", """test() {
    case "$1" in
        -x|-S) [[ ${TEST_AUDIO:-} = yes ]] ;;
        *) builtin test "$@" ;;
    esac
}
function /usr/bin/melkor-session-exec() { printf '%s\n' "$1"; }
kill() { printf 'kill\n' >> "$TEST_BOOT_LOG"; }
exec() { "$@"; exit $?; }
desktop() { printf 'desktop\n' >> "$TEST_BOOT_LOG"; }
script=$1
shift
source "$script" "$@"
""")

    @classmethod
    def wrapper(cls, name, body):
        file = cls.work / name
        file.write_text("#!/usr/bin/env bash\n" + body)
        file.chmod(0o755)
        return file

    def setUp(self):
        self.log = self.work / "commands"
        self.log.write_text("")
        self.boot_log = self.work / "commands-boot"
        self.boot_log.write_text("")
        self.out = self.project / "out"
        shutil.rmtree(self.out, ignore_errors=True)
        self.downloads = self.out / "cache"
        self.downloads.mkdir(parents=True)
        self.bundle = self.downloads / "buildroot-2025.02.20.bundle"
        self.env = os.environ.copy()
        for key in ("MELKOR_BUILDROOT_URL", "MELKOR_BUILD_CONTAINER", "BASH_ENV"):
            self.env.pop(key, None)
        self.env.update(PATH=f"{self.bin}:{os.environ['PATH']}", TEST_TOOL_LOG=str(self.log),
                        TEST_BOOT_LOG=str(self.boot_log), NO_PROXY="*", no_proxy="*")

    def run_script(self, path, *args, code=0, **env):
        command = [str(path), *map(str, args)]
        if kcov := os.environ.get("MELKOR_TEST_KCOV"):
            name = "-".join(path.relative_to(self.project).parts) if path.is_relative_to(self.project) else path.name
            report = Path(os.environ["MELKOR_TEST_SHELL_REPORT"]) / name
            command = [kcov, f"--include-path={self.project}", str(report), *command]
        result = subprocess.run(command, cwd=self.project, env=self.env | env,
                                text=True, capture_output=True, timeout=15)
        self.assertEqual(result.returncode, code, result.stdout + result.stderr)
        return result.stdout + result.stderr

    def commands(self):
        return [json.loads(line) for line in self.log.read_text().splitlines()]

    def test_build_gera_evidencias_para_cada_alvo(self):
        for board, source in (("m0", "https://example.invalid/buildroot.git"),
                              ("m1", str(self.bundle)), ("live", str(self.bundle)), ("installer", str(self.bundle))):
            with self.subTest(board=board):
                override = {"MELKOR_BUILDROOT_URL": source} if board == "m0" else {}
                if not override:
                    self.bundle.touch()
                self.run_script(self.scripts / "build-image.sh", f"out/{board}", board, **override)
                self.assertTrue(any(command[-2] == source for command in self.commands() if "fetch" in command))
                build = self.out / board
                name = "rootfs.cpio.gz" if board == "installer" else f"melkor-{board}.img"
                digest = hashlib.sha256((build / "build/images" / name).read_bytes()).hexdigest()
                self.assertEqual((build / "SHA256SUMS").read_text().split()[0], digest)
                if board == "live":
                    self.assertEqual(len((build / "SHA256SUMS").read_text().splitlines()), 2)
                self.assertIn("musl", (build / "toolchain.txt").read_text())
                self.assertEqual(json.loads((build / "packages.json").read_text()), {"mock": "manifesto"})

    def test_build_rejeita_saida_alvo_commit_e_configuracao_invalidos(self):
        script = self.scripts / "build-image.sh"
        for args in (("out/fresh", "other"), ("out/dl/child",), ("/etc/melkor-fake",), ("out/cache",)):
            self.run_script(script, *args, code=2)
        (self.out / "existing").mkdir()
        self.assertIn("nova pasta", self.run_script(script, "out/existing", code=2))
        self.assertEqual(self.commands(), [])
        text = self.run_script(script, "out/commit", code=1, TEST_COMMIT="outro")
        self.assertIn("Esperado", text)
        self.assertTrue(any(command[-2].startswith("https://") for command in self.commands() if "fetch" in command))
        self.assertFalse(any("checkout" in command or command[0] == "make" for command in self.commands()))
        text = self.run_script(script, "out/config", code=1, TEST_MISSING_OPTION="BR2_DOWNLOAD_FORCE_CHECK_HASHES")
        self.assertIn("não ativou BR2_DOWNLOAD_FORCE_CHECK_HASHES", text)
        self.assertFalse(any("show-info" in command for command in self.commands()))

    def test_build_minimo_mantem_a_rede_e_rejeita_camadas_graficas(self):
        script = self.scripts / "build-image.sh"
        minimal = {"MELKOR_PC_MINIMAL": "1"}
        self.run_script(script, "out/minimo", "live", code=2, **minimal)
        self.run_script(script, "out/minimo", "pc", code=2, MELKOR_PC_DESKTOP_LAYER="desktop.tar.gz", **minimal)
        self.run_script(script, "out/minimo", "pc", **minimal)
        defconfig = self.out / "minimo" / "pc-minimal.defconfig"
        config = defconfig.read_text()
        for package in ("DNSPROXY", "IWD", "RAUC"):
            self.assertIn(f"BR2_PACKAGE_{package}=y", config)
        for package in ("WESTON", "FOOT", "MELKOR_NUSHELL", "SEATD"):
            self.assertNotIn(f"BR2_PACKAGE_{package}=y", config)
        self.assertTrue(any(f"BR2_DEFCONFIG={defconfig}" in command for command in self.commands()))

    def test_pc_minimo_desativa_a_sessao_grafica(self):
        board = self.project / "core/board/pc"
        target = self.out / "minimo"
        shutil.copytree(board / "rootfs-overlay", target, symlinks=True)
        self.run_script(board / "post-build.sh", target, MELKOR_PC_MINIMAL="1")
        services = (target / "etc/dinit.d/boot").read_text()
        self.assertIn("waits-for = wifi", services)
        self.assertNotIn("seatd", services)
        self.assertNotIn("graphical-login", services)
        self.assertIn("tty1", (target / "etc/dinit.d/console-local").read_text())
        self.assertFalse((target / "etc/pam.d/greetd").exists())
        self.assertIn("iwctl", (target / "etc/motd").read_text())
        self.assertFalse((target / "usr/bin/melkor-desktop").exists())

    def test_preparo_host_cria_ou_reusa_container(self):
        not_inside = self.work / "host"
        self.run_script(self.prepare, "--check", not_inside)
        self.assertEqual([command[1] for command in self.commands()], ["list", "enter"])
        self.log.write_text("")
        self.run_script(self.prepare, "prepare", not_inside, TEST_CONTAINER_ABSENT="1")
        self.assertEqual([command[1] for command in self.commands()], ["list", "create", "enter"])
        self.assertIn("ausente", self.run_script(self.prepare, "--check", not_inside, code=1,
                                                 TEST_CONTAINER_ABSENT="1"))
        self.assertIn("listar", self.run_script(self.prepare, "--check", not_inside, code=1,
                                                TEST_DISTROBOX_FAILURE="1"))
        self.run_script(self.prepare, "invalido", not_inside, code=2)

    def test_preparo_rejeita_antes_de_executar_arvore(self):
        text = self.run_script(self.prepare, "--check", self.marker, code=1, TEST_MISSING_PACKAGE="wget")
        # O manifesto usa a capacidade /usr/bin/wget no Fedora.
        self.assertIn("Bundle ausente", text)
        text = self.run_script(self.prepare, "--check", self.marker, code=1, TEST_MISSING_PACKAGE="/usr/bin/wget")
        self.assertIn("Requisito ausente", text)
        text = self.run_script(self.prepare, "prepare", self.marker, code=1, TEST_COMMIT="divergente")
        self.assertIn("Bundle aponta", text)
        self.assertFalse(self.bundle.exists())
        self.assertFalse(any("checkout" in command or command[0] == "make" for command in self.commands()))
        self.assertEqual(list(self.downloads.glob("prepare.*")), [])

    def test_preparo_valido_e_cache_incompleto(self):
        self.run_script(self.prepare, "prepare", self.marker, TEST_MISSING_PACKAGE="gcc")
        self.assertTrue(self.bundle.is_file())
        self.assertTrue(any(command[0] == "sudo" for command in self.commands()))
        self.log.write_text("")
        text = self.run_script(self.prepare, "--check", self.marker, code=1, TEST_OFFLINE_FAILURE="1")
        self.assertIn("Faltam fontes", text)
        self.assertIn("fonte fora do cache", (self.downloads / "offline-check.log").read_text())
        self.assertEqual(sum("source" in command for command in self.commands()), 1)
        self.assertFalse(any("clone" in command for command in self.commands()))
        self.assertEqual(list(self.downloads.glob("prepare.*")), [])
        self.assertIn("Ambiente pronto", self.run_script(self.prepare, "--check", self.marker))

    def test_hooks_de_imagem_validam_entradas_e_limpam_temporarios(self):
        build = self.out / "hooks"
        binaries = build / "images"
        efi = binaries / "efi-part/EFI/BOOT"
        efi.mkdir(parents=True)
        host = build / "host"
        host.mkdir()
        (host / "bin").symlink_to(self.bin)
        env = dict(BUILD_DIR=str(build), BINARIES_DIR=str(binaries), HOST_DIR=str(host))
        board = self.project / "core/board"
        self.run_script(board / "m0/post-image.sh", code=1, **env)
        (efi / "bootx64.efi").write_text("EFI")
        (binaries / "bzImage").write_text("kernel")
        (binaries / "rootfs.squashfs").write_text("rootfs")
        self.run_script(board / "m0/post-image.sh", **env)
        self.run_script(board / "m1/post-image.sh", code=5, TEST_GENIMAGE_CODE="5", **env)
        self.run_script(board / "m1/post-image.sh", **env)
        self.run_script(board / "live/post-image.sh", code=1, **env)
        (binaries / "rootfs.cpio.gz").write_text("initramfs")
        self.run_script(board / "live/post-image.sh", code=5, TEST_GENIMAGE_CODE="5", **env)
        self.run_script(board / "live/post-image.sh", **env)
        target = build / "target"
        self.run_script(board / "m1/post-build.sh", target, **env)
        self.assertEqual((target / "boot/bzImage").read_text(), "kernel")
        self.assertTrue((target / "persist").is_dir())
        self.assertEqual(sorted(path.name for path in build.iterdir()), ["host", "images", "target"])

        (target / "etc/rauc").mkdir(parents=True)
        shadow = target / "etc/shadow"
        shadow.write_text("root:*:1::::::\nmelkor:*:1::::::\n")
        hash_file, keyring = build / "hash", build / "keyring"
        password_hash = "$6$lab$" + "a" * 86
        hash_file.write_text(password_hash + "\n")
        cert = "-----BEGIN CERTIFICATE-----\ncertificado\n-----END CERTIFICATE-----\n"
        keyring.write_text("dados fora do certificado\n" + cert)
        self.run_script(board / "m1/post-build.sh", target, MELKOR_RAUC_KEYRING=keyring,
                        MELKOR_LAB_ROOT_HASH_FILE=hash_file, **env)
        self.assertEqual((target / "etc/rauc/keyring.pem").read_text(), cert)
        self.assertEqual(shadow.read_text(), f"root:{password_hash}:1::::::\nmelkor:*:1::::::\n")
        for invalid in ("invalido", password_hash + "\ninvalido"):
            hash_file.write_text(invalid + "\n")
            self.run_script(board / "m1/post-build.sh", target, code=1, MELKOR_LAB_ROOT_HASH_FILE=hash_file, **env)
        self.assertIn(password_hash, shadow.read_text())

    def test_pc_embute_revisao_e_chave_publica_do_canal(self):
        script = self.project / "core/board/pc/post-build.sh"
        target = self.out / "target"
        config = target / "etc/melkor-update"
        config.mkdir(parents=True)
        host = self.out / "host"
        (host / "bin").mkdir(parents=True)
        openssl = host / "bin/openssl"
        openssl.write_text("#!/bin/sh\nif [ \"$5\" = -text_pub ]; then "
                           "printf '%s\\n' 'ED25519 Public-Key:'; else cp \"$4\" \"$7\"; fi\n")
        openssl.chmod(0o755)
        key = self.out / "channel.pem"
        key.write_text("chave de teste")
        self.run_script(script, target, HOST_DIR=host)
        self.assertEqual((config / "revision").read_text(), "0\n")
        for revision in ("-1", "01", "4294967296", "1"):
            self.run_script(script, target, code=1, MELKOR_CORE_REVISION=revision, HOST_DIR=host)
        self.run_script(script, target, MELKOR_CORE_REVISION="1", HOST_DIR=host,
                        MELKOR_RAUC_KEYRING=key, MELKOR_CHANNEL_PUBLIC_KEY=key)
        self.assertEqual((config / "revision").read_text(), "1\n")
        self.assertEqual((config / "channel.pem").read_text(), "chave de teste")
        layer = self.out / "layer.tar.gz"
        payload = self.out / "payload"
        icon = payload / "usr/lib/firefox-esr/browser/chrome/icons/default/default32.png"
        icon.parent.mkdir(parents=True)
        icon.write_text("ícone")
        weston = target / "etc/xdg/weston"
        weston.mkdir(parents=True)
        (weston / "weston.ini").write_text("[core]\n")
        (weston / "melkor-firefox.ini").write_text("[launcher]\n")
        (target / "etc/xdg/melkor").mkdir()
        (target / "etc/xdg/melkor/firefox-policies.json").write_text("{}")
        for config in ("mimeapps.list", "audio-pulse.conf"):
            shutil.copy2(script.parent / "rootfs-overlay/etc/xdg/melkor" / config, target / "etc/xdg/melkor")
        for browser in ("firefox", "brave-origin", "blanc"):
            (target / f"etc/xdg/melkor/{browser}.desktop").write_text("[Desktop Entry]\n")
        (weston / "melkor-chromium.ini").write_text("[launcher]\n")
        for icon_path in ("opt/brave.com/brave-origin/product_logo_32.png",
                          "opt/blanc/usr/share/icons/hicolor/1024x1024/apps/blanc.png"):
            browser_icon = payload / icon_path
            browser_icon.parent.mkdir(parents=True)
            browser_icon.write_text("ícone")
        subprocess.run(["tar", "-czf", layer, "-C", payload, "."], check=True)
        (target / "etc/dinit.d").mkdir()
        for name in ("boot", "console-local"):
            shutil.copy2(script.parent / "rootfs-overlay/etc/dinit.d" / name, target / "etc/dinit.d")
        self.run_script(script, target, code=1, HOST_DIR=host, MELKOR_PC_DESKTOP_LAYER=layer,
                        MELKOR_PC_SESSION_LAYER=layer)
        self.assertNotIn("graphical-login", (target / "etc/dinit.d/boot").read_text())
        (weston / "weston.ini").write_text("[core]\n")
        for name in ("sbin/greetd", "sbin/unix_chkpwd", "bin/cage", "bin/gtkgreet", "bin/umbriel"):
            binary = payload / "usr" / name
            binary.parent.mkdir(parents=True, exist_ok=True)
            binary.touch(0o755)
        subprocess.run(["tar", "-czf", layer, "-C", payload, "."], check=True)
        patchelf = host / "bin/patchelf"
        patchelf.write_text('#!/bin/sh\nprintf "%s\\n" "$@" > "$TEST_BOOT_LOG"\n')
        patchelf.chmod(0o755)
        self.run_script(script, target, HOST_DIR=host, MELKOR_PC_DESKTOP_LAYER=layer,
                        MELKOR_PC_CHROMIUM_LAYER=layer, MELKOR_PC_SESSION_LAYER=layer)
        self.assertEqual((weston / "weston.ini").read_text(), "[core]\n[launcher]\n[launcher]\n")
        self.assertEqual((target / "usr/share/melkor/firefox.png").read_text(), "ícone")
        self.assertEqual((target / "usr/share/melkor/blanc.png").read_text(), "ícone")
        self.assertIn("x-scheme-handler/https=melkor-firefox.desktop;",
                      (target / "usr/share/applications/mimeapps.list").read_text())
        self.assertIn("waits-for = graphical-login", (target / "etc/dinit.d/boot").read_text())
        self.assertIn("tty2", (target / "etc/dinit.d/console-local").read_text())
        service = (target / "etc/dinit.d/graphical-login").read_text()
        self.assertIn("depends-on = persist", service)
        self.assertIn("depends-on = seatd", service)
        self.assertIn("user = melkor", (target / "etc/pam.d/greetd").read_text())
        self.assertEqual(str((target / "usr/sbin/unix_chkpwd").readlink()), "/opt/melkor-desktop/usr/sbin/unix_chkpwd")
        self.assertEqual((target / "opt/melkor-desktop/usr/sbin/unix_chkpwd").stat().st_mode & 0o7777, 0o700)
        self.assertIn("/opt/melkor-desktop/lib/ld-musl-x86_64.so.1", self.boot_log.read_text())

    def test_greeter_usa_runtime_privado_e_abre_apenas_umbriel(self):
        wrapper = self.wrapper("greeter", '''
mktemp() { command mktemp -d "$TEST_GREETER/XXXXXX"; }
function /usr/bin/melkor-session-exec() {
    test "$(stat -c %a "$XDG_RUNTIME_DIR")" = 700
    test "$XDG_RUNTIME_DIR" = "$XDG_CACHE_HOME"
    test "$LIBSEAT_BACKEND:$WLR_RENDERER:$GDK_BACKEND" = seatd:pixman:wayland
    printf '%s\\n' "$@" > "$TEST_BOOT_LOG"
    return "$TEST_GREETER_STATUS"
}
source "$1"
''')
        script = self.project / "core/board/pc/graphical-login/usr/libexec/melkor-greeter"
        for status in (0, 1):
            self.run_script(wrapper, script, code=status, TEST_GREETER=self.out, TEST_GREETER_STATUS=str(status))
            self.assertEqual(self.boot_log.read_text().splitlines(), [
                "cage", "-s", "--", "/usr/bin/melkor-session-exec", "gtkgreet",
                "-c", "/usr/bin/melkor-desktop --umbriel",
            ])
            self.assertEqual(list(self.out.iterdir()), [self.downloads])

    def test_finaliza_pc_com_midia_somente_leitura_e_limpa_area_temporaria(self):
        wrapper = self.wrapper("finalizar", '''
id() { printf '%s\\n' "$TEST_UID"; }
blkid() {
    printf 'blkid %s\\n' "$*" >> "$TEST_BOOT_LOG"
    case "$TEST_MEDIA" in
        no) return 2 ;;
        multiple) printf '/dev/usb1\\n/dev/usb2\\n' ;;
        *) printf '/dev/usb1\\n' ;;
    esac
}
test() {
    if [[ $1 = -b ]]; then [[ $2 = /dev/usb1 ]]; else builtin test "$@"; fi
}
mktemp() { command mktemp -d "$TEST_FINALIZAR/XXXXXX"; }
mount() {
    printf 'mount %s\\n' "$*" >> "$TEST_BOOT_LOG"
    if [[ $TEST_BUNDLE = yes ]]; then printf 'bundle' > "${@: -1}/melkor-desktop.raucb"; fi
}
umount() {
    printf 'umount\\n' >> "$TEST_BOOT_LOG"
    rm -f "$1/melkor-desktop.raucb"
}
melkor-core-update() {
    builtin test "$(stat -c %a "$1")" = 600
    printf 'update %s\\n' "$(cat "$1")" >> "$TEST_BOOT_LOG"
    return "$TEST_UPDATE_CODE"
}
exec() { printf '%s\\n' "$*" >> "$TEST_BOOT_LOG"; exit 0; }
script=$1
shift
source "$script" "$@"
''')
        script = self.project / "core/board/pc/rootfs-overlay/usr/bin/melkor-finalizar"
        env = dict(TEST_UID="0", TEST_MEDIA="yes", TEST_FINALIZAR=self.out,
                   TEST_BUNDLE="yes", TEST_UPDATE_CODE="0")
        self.run_script(wrapper, script, code=2, **(env | {"TEST_UID": "1000"}))
        self.run_script(wrapper, script, "Ventoy", "extra", code=2, **env)
        self.run_script(wrapper, script, **env)
        self.assertEqual(self.boot_log.read_text(), "melkor-core-update --desktop\n")
        for invalid in ("no", "multiple"):
            self.run_script(wrapper, script, "MELKOR_APPS", code=1, **(env | {"TEST_MEDIA": invalid}))
        for status, options in ((1, {"TEST_BUNDLE": "no"}), (42, {"TEST_UPDATE_CODE": "42"}), (0, {})):
            self.boot_log.write_text("")
            self.run_script(wrapper, script, "Ventoy", code=status, **(env | options))
            self.assertEqual(list(self.out.iterdir()), [self.downloads])
            observed = self.boot_log.read_text()
            self.assertIn("blkid -t LABEL=Ventoy -o device", observed)
            self.assertIn("mount -t vfat -o ro,nosuid,nodev,noexec /dev/usb1", observed)
            self.assertIn("umount\n", observed)
            self.assertEqual("update bundle" in observed, status != 1)

    def test_lancadores_pc_preservam_shell_e_isolam_o_navegador(self):
        binaries = self.project / "core/board/pc/rootfs-overlay/usr/bin"
        self.run_script(self.app, binaries / "melkor-terminal")
        self.assertEqual(self.boot_log.read_text(), "/usr/bin/foot\n")
        self.run_script(self.app, binaries / "melkor-terminal", TEST_APP="yes")
        self.assertIn("alacritty --config-file /etc/xdg/alacritty/melkor.toml -e /usr/bin/nu", self.boot_log.read_text())
        self.run_script(self.app, binaries / "melkor-firefox", code=1)
        self.run_script(self.app, binaries / "melkor-firefox", TEST_APP="yes", HOME=self.out,
                        TEST_PULSE="yes",
                        XDG_RUNTIME_DIR="/run/session", WAYLAND_DISPLAY="wayland-1")
        args = self.boot_log.read_text()
        self.assertIn("--unshare-user --unshare-pid --unshare-ipc --unshare-uts", args)
        self.assertIn("--ro-bind /run/session/wayland-1 /run/wayland", args)
        self.assertIn("--ro-bind /etc/resolv.conf /etc/resolv.conf", args)
        self.assertIn("--ro-bind /run/session/pulse/native /run/pulse/native", args)
        self.assertIn("--setenv PULSE_SERVER unix:/run/pulse/native", args)
        self.assertNotIn("/dev/snd", args)
        self.assertNotIn("--no-sandbox", args)
        self.run_script(self.app, binaries / "melkor-browser", code=2)
        self.run_script(self.app, binaries / "melkor-session-exec", code=2)
        self.run_script(self.app, binaries / "melkor-session-exec", "noctalia", "--version")
        self.assertIn("/usr/bin/noctalia --version", self.boot_log.read_text())
        for name, program in (("brave-origin", "/opt/brave.com/brave-origin/brave-origin"),
                              ("blanc", "/opt/blanc/AppRun")):
            self.run_script(self.app, binaries / f"melkor-{name}", TEST_APP="yes", HOME=self.out,
                            XDG_RUNTIME_DIR="/run/session", WAYLAND_DISPLAY="wayland-1")
            args = self.boot_log.read_text()
            self.assertIn(f"/share/melkor/{name}", args)
            self.assertIn(f"{program} --ozone-platform=wayland", args)
            self.assertNotIn("--no-sandbox", args)
            self.assertNotIn("PULSE_SERVER", args)

    def test_pc_audio_acompanha_o_ciclo_da_sessao(self):
        file = self.project / "core/board/pc/rootfs-overlay/usr/libexec/melkor-desktop-session"
        self.run_script(self.session, file, "desktop", XDG_RUNTIME_DIR=self.out)
        self.assertEqual(self.boot_log.read_text(), "desktop\n")
        self.boot_log.write_text("")
        self.run_script(self.session, file, "desktop", TEST_AUDIO="yes", XDG_RUNTIME_DIR=self.out)
        self.assertEqual(self.boot_log.read_text(), "desktop\nkill\nkill\nkill\n")
        for service in ("pipewire", "wireplumber", "pipewire-pulse"):
            self.assertEqual((self.out / f"{service}.log").read_text(), f"{service}\n")

    def test_init_live_exige_devs_em_memoria_antes_de_iniciar_dinit(self):
        init = self.project / "core/board/live/rootfs-overlay/init"
        self.run_script(self.boot, init, code=1, TEST_MOUNT_CODE="1")
        self.assertEqual(self.boot_log.read_text().splitlines(), ["mount -t devtmpfs -o nosuid,mode=0755 devtmpfs /dev"])
        self.boot_log.write_text("")
        self.run_script(self.boot, init)
        self.assertEqual(self.boot_log.read_text().splitlines()[-1], "/sbin/dinit")

    def test_instalador_seleciona_a_tela_ou_o_console_serial(self):
        script = self.project / "core/board/installer/rootfs-overlay/usr/libexec/melkor-installer-console"
        self.run_script(self.boot, script, TEST_CMDLINE="quiet console=tty0")
        self.run_script(self.boot, script, TEST_CMDLINE="quiet melkor.serial=1")
        self.assertEqual(self.boot_log.read_text().splitlines(), [
            "/sbin/getty -n -l /usr/bin/melkor-instalar 38400 tty1 linux",
            "/sbin/getty -L -n -l /usr/bin/melkor-instalar 115200 ttyS0 vt100",
        ])

    def test_dbus_live_prepara_identidade_antes_de_abrir_o_socket(self):
        script = self.project / "core/board/live/rootfs-overlay/usr/libexec/melkor-system-bus"
        self.run_script(self.boot, script, code=1, TEST_DBUS_UUID_CODE="1")
        self.assertEqual(self.boot_log.read_text().splitlines(), [
            "install -d -m 0755 /run/dbus", "dbus-uuidgen --ensure=/run/machine-id",
        ])
        self.boot_log.write_text("")
        self.run_script(self.boot, script)
        self.assertEqual(self.boot_log.read_text().splitlines()[-1],
                         "dbus-daemon --system --nofork --nopidfile --print-address=4")

    def test_pc_monta_dados_e_esp_do_disco_resolvido(self):
        root = self.project / "core/board/pc/rootfs-overlay/usr/libexec"
        self.run_script(self.boot, root / "melkor-pc-persist", code=1, TEST_MOUNT_CODE="1")
        self.assertNotIn("bind", self.boot_log.read_text())
        self.boot_log.write_text("")
        self.run_script(self.boot, root / "melkor-pc-persist", code=1, TEST_CREDENTIALS_CODE="1")
        self.assertNotIn("bind", self.boot_log.read_text())
        self.boot_log.write_text("")
        self.run_script(self.boot, root / "melkor-pc-persist")
        self.run_script(self.boot, root / "melkor-rauc-setup")
        log = self.boot_log.read_text()
        self.assertIn("/run/melkor/data /persist", log)
        self.assertIn("melkor-storage credenciais", log)
        self.assertIn("mount -t tmpfs -o mode=1777,nosuid,nodev tmpfs /tmp", log)
        self.assertIn("/persist/melkor /home/melkor", log)
        self.assertIn("mount -t tmpfs -o mode=1777,nosuid,nodev,noexec tmpfs /dev/shm", log)
        self.assertIn("install -d -m 0700 -o root -g root /persist/iwd", log)
        self.assertIn("/run/melkor/esp-device /run/melkor/esp", log)

    def test_pc_aloca_e_publica_swap_antes_de_ativar(self):
        file = self.project / "core/board/pc/rootfs-overlay/usr/libexec/melkor-pc-swap"
        self.run_script(self.boot, file, code=1, TEST_FALLOCATE_CODE="1")
        self.assertNotIn("swapon", self.boot_log.read_text())
        self.assertIn("rm -f -- /tmp/melkor-simulado", self.boot_log.read_text())
        self.boot_log.write_text("")
        self.run_script(self.boot, file)
        self.assertEqual(self.boot_log.read_text().splitlines()[1:5], [
            "fallocate -l 4G /tmp/melkor-simulado", "mkswap /tmp/melkor-simulado",
            "mv -T /tmp/melkor-simulado /persist/.swap/swapfile", "swapon /persist/.swap/swapfile",
        ])

    def test_pc_inicia_uma_sessao_wayland_com_runtime_privado(self):
        file = self.project / "core/board/pc/rootfs-overlay/usr/bin/melkor-desktop"
        self.run_script(self.boot, file)
        self.assertEqual(self.boot_log.read_text().splitlines(), [
            "dbus-run-session /tmp/melkor-simulado seatd -- /usr/libexec/melkor-desktop-session weston --renderer=pixman",
            "rm -rf -- /tmp/melkor-simulado",
        ])
        self.boot_log.write_text("")
        self.run_script(self.boot, file, TEST_UMBRIEL="1")
        self.assertIn("melkor-session-exec umbriel -c /etc/xdg/melkor/umbriel.toml", self.boot_log.read_text())
        self.boot_log.write_text("")
        self.run_script(self.boot, file, "--weston", TEST_UMBRIEL="1")
        self.assertIn("weston --renderer=pixman", self.boot_log.read_text())
        self.run_script(self.boot, file, "--invalido", code=2)

    def test_rauc_monta_so_dispositivos_inequivocos(self):
        setup = self.project / "core/board/m1/rootfs-overlay/usr/libexec/melkor-rauc-setup"
        self.run_script(self.boot, setup)
        lines = self.boot_log.read_text().splitlines()
        self.assertIn("ln -sf /dev/vda2 /run/melkor/core-a", lines)
        self.assertIn("ln -sf /dev/vda3 /run/melkor/core-b", lines)
        self.assertTrue(lines[-1].endswith("/dev/vda1 /run/melkor/esp"))
        self.boot_log.write_text("")
        self.run_script(self.boot, setup, code=1, TEST_AMBIGUOUS="1")
        self.assertEqual(self.boot_log.read_text(), "")

    def test_atualizacao_exige_bundle_absoluto_e_sessao_root(self):
        update = self.project / "core/board/m1/rootfs-overlay/usr/sbin/melkor-core-update"
        bundle = self.project / "out/update.raucb"
        bundle.touch()
        self.run_script(self.update, update, code=2)
        self.run_script(self.update, update, "out/update.raucb", code=2)
        self.run_script(self.update, update, bundle, code=1, TEST_UID="1000")
        self.assertEqual(self.boot_log.read_text(), "")
        self.run_script(self.update, update, bundle, TEST_UID="0")
        self.assertEqual(self.boot_log.read_text(), f"/usr/bin/flock --nonblock /run/melkor/rauc.lock "
                                                   f"/usr/bin/rauc install {bundle}\n")

    def test_atualizacao_https_usa_a_mesma_trava_do_rauc(self):
        update = self.project / "core/board/m1/rootfs-overlay/usr/sbin/melkor-core-update"
        url, digest = "https://example.invalid/update.raucb", "a" * 64
        self.run_script(self.update, update, "--https", url, code=2, TEST_CORE_FETCH="yes")
        self.run_script(self.update, update, "--https", url, digest, code=2)
        self.run_script(self.update, update, "--https", url, digest, code=1, TEST_CORE_FETCH="yes", TEST_UID="1000")
        self.assertEqual(self.boot_log.read_text(), "")
        self.run_script(self.update, update, "--https", url, digest, TEST_CORE_FETCH="yes", TEST_UID="0")
        self.assertEqual(self.boot_log.read_text(), f"/usr/bin/flock --nonblock /run/melkor/rauc.lock "
                                                   f"/usr/bin/melkor-core-fetch {url} {digest}\n")

    def test_canal_do_pc_exige_root_e_usa_a_trava(self):
        update = self.project / "core/board/m1/rootfs-overlay/usr/sbin/melkor-core-update"
        source = "https://example.invalid/channel.json"
        for args in ((), ("--channel", source), ("--reset-channel",), ("--desktop",)):
            self.run_script(self.update, update, *args, code=1, TEST_UID="1000", TEST_CORE_FETCH="yes")
            self.run_script(self.update, update, *args, TEST_UID="0", TEST_CORE_FETCH="yes")
        prefix = "/usr/bin/flock --nonblock /run/melkor/rauc.lock /usr/bin/melkor-core-fetch"
        self.assertEqual(self.boot_log.read_text().splitlines(),
                         [prefix, f"{prefix} --channel {source}", f"{prefix} --reset-channel",
                          f"{prefix} --desktop"])

    def test_boot_monta_alvos_e_interrompe_em_falha(self):
        board = self.project / "core/board"
        setup = board / "m0/rootfs-overlay/usr/libexec/melkor-setup"
        persist = board / "m1/rootfs-overlay/usr/libexec/melkor-persist"
        self.run_script(self.boot, setup)
        self.run_script(self.boot, persist)
        lines = self.boot_log.read_text().splitlines()
        self.assertTrue(lines[0].endswith("proc /proc"))
        self.assertIn("hostname melkor", lines)
        self.assertTrue(lines[-3].endswith("/persist"))
        self.assertIn("-o melkor -g melkor", lines[-2])
        self.assertEqual(lines[-1], "mount -o bind /persist/melkor /home/melkor")
        for script in (setup, persist):
            self.boot_log.write_text("")
            self.run_script(self.boot, script, code=7, TEST_MOUNT_CODE="7")
            self.assertEqual(len(self.boot_log.read_text().splitlines()), 1)
