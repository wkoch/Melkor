import argparse
import hashlib
from contextlib import contextmanager, redirect_stderr, redirect_stdout
import importlib.util
import io
import json
import os
from pathlib import Path
import subprocess
import sys
from tempfile import TemporaryDirectory
from types import SimpleNamespace
import unittest
from unittest.mock import MagicMock, patch


def script(name):
    path = Path(__file__).resolve().parents[1] / "scripts" / f"{name}.py"
    spec = importlib.util.spec_from_file_location(name, path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


boot = script("test-boot")
image = script("check-m1-image")
bundle = script("bundle-m1")
installer = script("installer-iso")

LOGIN = b"\rmelkor login: \n\rPassword: \nmelkor$ \n"
PROBE = b"M0-ID=melkor\nM0-PID1=dinit\nM0-LOGIN=melkor\nM0-UEFI=yes\nM0-ROOT=squashfs,ro\nM0-END\n"
MENU = b"Limine\nE\x1b[35;2HNTER Boot\n"
LIVE = (b"LIVE-VERSION=live\nLIVE-USER=ok\nLIVE-DISPLAY=ok\nLIVE-DEVICES=ok\n"
        b"LIVE-PERMISSIONS=ok\nLIVE-TOOLS=ok\nLIVE-RAM=ok\nLIVE-HOME=ok\nLIVE-END\n"
        b"LIVE-FIRMWARE=ok\nLIVE-REGDB=ok\nLIVE-IWD=ok\nLIVE-WIFI-END\n")


class InstallerMediaTest(unittest.TestCase):
    def test_valida_manifestos_antes_de_montar_iso(self):
        with TemporaryDirectory() as temporary:
            root = Path(temporary)
            kernel = root / "build/images/bzImage"
            kernel.parent.mkdir(parents=True)
            kernel.write_bytes(b"kernel")
            (root / "SHA256SUMS").write_text(f"{hashlib.sha256(b'kernel').hexdigest()}  bzImage\n")
            self.assertEqual(installer.asset(root, "bzImage")[0], kernel)
            kernel.write_bytes(b"adulterado")
            with patch.object(installer.subprocess, "run") as run, self.assertRaisesRegex(ValueError, "SHA-256"):
                installer.prepare_media(root, root, root / "install.iso")
            run.assert_not_called()

    def test_midia_monta_limine_e_payload_do_disco_sem_sobrescrever(self):
        with TemporaryDirectory() as temporary:
            root = Path(temporary)
            for mode, files in {
                "installer": {"bzImage": b"kernel", "rootfs.cpio.gz": b"initramfs"},
                "pc": {"rootfs.squashfs": b"core", "efi-part/EFI/BOOT/bootx64.efi": b"limine-pc"},
            }.items():
                output = root / mode
                hashes = []
                for name, contents in files.items():
                    path = output / "build/images" / name
                    path.parent.mkdir(parents=True, exist_ok=True)
                    path.write_bytes(contents)
                    hashes.append(f"{hashlib.sha256(contents).hexdigest()}  {name}\n")
                (output / "SHA256SUMS").write_text("".join(hashes))

            limine = root / "installer/build/host/share/melkor-limine"
            limine.mkdir(parents=True)
            for name in ("BOOTX64.EFI", "LICENSE", "3RDPARTY.md"):
                (limine / name).write_text(name)

            def tool(args, **options):
                if args[0] == "cpio":
                    self.assertIn(b"melkor/core.sha256", options["input"])
                    options["stdout"].write(b"payload")
                elif Path(args[0]).name == "xorriso":
                    self.assertIn("MELKOR_INSTALL_x86_64", args)
                    self.assertEqual((Path(args[-1]) / "EFI/BOOT/BOOTX64.EFI").read_text(), "BOOTX64.EFI")
                    self.assertIn("protocol: linux", (Path(args[-1]) / "EFI/BOOT/limine.conf").read_text())
                    self.assertTrue((Path(args[-1]) / "rootfs.cpio.gz").read_bytes().startswith(b"initramfs"))
                    Path(args[args.index("-o") + 1]).write_bytes(b"iso")

            output = root / "install.iso"
            with patch.object(installer.subprocess, "run", side_effect=tool):
                installer.prepare_media(root / "installer", root / "pc", output)
                with self.assertRaises(FileExistsError):
                    installer.prepare_media(root / "installer", root / "pc", output)
            self.assertEqual(output.read_bytes(), b"iso")


class BundleTest(unittest.TestCase):
    def setUp(self):
        directory = TemporaryDirectory()
        self.addCleanup(directory.cleanup)
        self.build = Path(directory.name)
        self.rauc = self.build / "host/bin/rauc"
        self.source = self.build / "images/rootfs.squashfs"
        self.cert = self.build / "cert.pem"
        self.key = self.build / "key.pem"
        self.output = self.build / "update.raucb"
        for path in (self.rauc, self.source, self.cert, self.key):
            path.parent.mkdir(parents=True, exist_ok=True)
            path.touch()
        self.source.write_bytes(b"hsqsconteudo")

    def create(self, version="1.0"):
        bundle.create_bundle(self.rauc, self.source, self.cert, self.key, self.cert, version, self.output)

    def fake_rauc(self, command, *, check, env):
        self.assertTrue(check)
        self.assertEqual(env["PATH"].split(os.pathsep)[0], str(self.rauc.parent))
        self.assertEqual(command[:2], [self.rauc, "bundle"])
        self.assertIn(f"--signing-keyring={self.cert}", command)
        content, candidate = command[-2:]
        self.assertEqual((content / self.source.name).read_bytes(), self.source.read_bytes())
        self.assertEqual((content / "manifest.raucm").read_text(),
                         "[update]\ncompatible=melkor-m1-limine-x86_64\nversion=1.0\n\n"
                         "[bundle]\nformat=verity\n\n[image.rootfs]\nfilename=rootfs.squashfs\n")
        candidate.write_bytes(b"bundle de teste")

    def test_cli_publica_bundle_e_recusa_sobrescrita(self):
        argv = ["bundle-m1.py", str(self.build), "--cert", str(self.cert), "--key", str(self.key),
                "--keyring", str(self.cert), "--version", "1.0", "--output", str(self.output)]
        with patch.object(sys, "argv", argv), patch.object(bundle.subprocess, "run", self.fake_rauc):
            with redirect_stdout(io.StringIO()):
                self.assertEqual(bundle.main(), 0)
            with redirect_stderr(io.StringIO()) as error:
                self.assertEqual(bundle.main(), 1)
            self.assertIn("já existe", error.getvalue())
        self.assertEqual(self.output.read_bytes(), b"bundle de teste")
        self.assertEqual(list(self.build.glob(".bundle-*")), [])

    def test_cli_pc_seleciona_capacidade_e_compatibilidade_proprias(self):
        argv = ["bundle-m1.py", str(self.build), "--pc", "--cert", str(self.cert), "--key", str(self.key),
                "--keyring", str(self.cert), "--version", "1.0", "--output", str(self.output)]
        with patch.object(sys, "argv", argv), patch.object(bundle, "create_bundle") as create:
            with redirect_stdout(io.StringIO()):
                self.assertEqual(bundle.main(), 0)
        self.assertEqual(create.call_args.args[-2:], ("melkor-pc-limine-x86_64", 2 * 1024 ** 3))

    def test_recusa_entradas_invalidas_e_limpa_assinatura_incompleta(self):
        with self.assertRaisesRegex(ValueError, "versão"):
            self.create("nova\n[update]")
        with patch.object(bundle, "SLOT_SIZE", 4), self.assertRaisesRegex(ValueError, "SquashFS"):
            self.create()
        self.key.unlink()
        with self.assertRaisesRegex(ValueError, "ausente"):
            self.create()
        self.key.touch()

        def fail(command, *, check, env):
            self.fake_rauc(command, check=check, env=env)
            raise subprocess.CalledProcessError(1, command)

        with patch.object(bundle.subprocess, "run", fail), self.assertRaises(subprocess.CalledProcessError):
            self.create()
        self.assertFalse(self.output.exists())
        self.assertEqual(list(self.build.glob(".bundle-*")), [])


@contextmanager
def console(data, keep_open=False):
    reader, writer = os.pipe()
    with os.fdopen(reader, "rb") as stdout, os.fdopen(writer, "wb") as sender:
        sender.write(data)
        sender.flush()
        if not keep_open:
            sender.close()
        yield SimpleNamespace(stdin=io.BytesIO(), stdout=stdout)


class SerialTest(unittest.TestCase):
    def test_live_exige_ram_sem_disco_montado_e_usuario_comum(self):
        with console(LOGIN + PROBE + LIVE) as process:
            boot.check_live_console(process, io.BytesIO(), timeout=1)
            self.assertIn(boot.LIVE_PROBE.encode(), process.stdin.getvalue())
            self.assertIn(boot.LIVE_WIFI_PROBE.encode(), process.stdin.getvalue())
        for field in (b"LIVE-RAM=ok", b"LIVE-PERMISSIONS=ok", b"LIVE-USER=ok",
                      b"LIVE-FIRMWARE=ok", b"LIVE-REGDB=ok", b"LIVE-IWD=ok"):
            with self.subTest(field=field), console(LOGIN + PROBE + LIVE.replace(field, b"")) as process:
                with self.assertRaisesRegex(RuntimeError, field.decode()):
                    boot.check_live_console(process, io.BytesIO(), timeout=1)

    def test_boot_m0(self):
        with console(LOGIN + PROBE) as process:
            log = io.BytesIO()
            boot.check_console(process, log, timeout=1)
            self.assertEqual(process.stdin.getvalue(), b"melkor\rmelkor\r" + boot.PROBE.encode() + b"\r")
            self.assertIn(b"M0-ROOT=squashfs,ro", log.getvalue())

    def test_slot_b_selecionado_e_documento_reaberto(self):
        persist = (b"\nM1-VERSION=m1\nM1-SLOT=rauc.slot=B\nM1-DATA=rw,nosuid\n"
                   b"M1-DOCUMENT=melkor-m1-persistencia\nM1-END\n")
        with console(MENU + LOGIN + PROBE + persist) as process:
            boot.check_console(process, io.BytesIO(), timeout=1, slot="B")
            self.assertTrue(process.stdin.getvalue().startswith(b"\x1b[B\rmelkor\r"))
            self.assertNotIn(boot.WRITE_DOCUMENT.encode(), process.stdin.getvalue())

    def test_slot_b_aguarda_menu_antes_de_enviar_teclas(self):
        with console(LOGIN) as process:
            with self.assertRaisesRegex(RuntimeError, "menu do Limine"):
                boot.check_console(process, io.BytesIO(), timeout=1, slot="B")
            self.assertEqual(process.stdin.getvalue(), b"")

    def test_disco_pc_exige_arquivo_swap(self):
        persist = (b"\nM1-VERSION=pc\nM1-SLOT=rauc.slot=A\nM1-DATA=rw,nosuid\n"
                   b"M1-DOCUMENT=melkor-m1-persistencia\nM1-SYNC=ok\nM1-END\n")
        with console(LOGIN + PROBE + b"\nPC-SWAP=ok\n" + persist) as process:
            boot.check_console(process, io.BytesIO(), timeout=1, slot="A", pc=True)
            self.assertIn(boot.PC_SWAP_PROBE.encode(), process.stdin.getvalue())
        with console(LOGIN + PROBE + persist) as process:
            with self.assertRaisesRegex(RuntimeError, "PC-SWAP=ok"):
                boot.check_console(process, io.BytesIO(), timeout=1, slot="A", pc=True)

    def test_disco_pc_exige_senha_root_separada_e_protege_shadow(self):
        rejected = b"\rmelkor login: \n\rPassword: \n\rLogin incorrect\r\n"
        privilege = b"PC-USER=ok\nPassword: \nmelkor# \nPC-ROOT=ok\nmelkor$ \n"
        persist = (b"\nPC-SWAP=ok\nM1-VERSION=pc\nM1-SLOT=rauc.slot=A\nM1-DATA=rw,nosuid\n"
                   b"M1-DOCUMENT=melkor-m1-persistencia\nM1-SYNC=ok\nM1-END\n")
        with console(rejected + LOGIN + privilege + PROBE + persist) as process:
            boot.check_console(process, io.BytesIO(), timeout=1, slot="A", pc=True,
                               credentials=(b"usuario", b"administracao"))
            self.assertTrue(process.stdin.getvalue().startswith(b"melkor\rmelkor\rmelkor\rusuario\r"))
            self.assertIn(b"su -\radministracao\r", process.stdin.getvalue())

    def test_boot_rejeita_identidade_ou_raiz_incorreta(self):
        for target, replacement, error in (
            (b"M0-UEFI=yes", b"M0-UEFI=no", "M0-UEFI=yes"),
            (b"squashfs,ro", b"squashfs,rw", "somente para leitura"),
        ):
            with self.subTest(replacement=replacement), console(LOGIN + PROBE.replace(target, replacement)) as process:
                with self.assertRaisesRegex(RuntimeError, error):
                    boot.check_console(process, io.BytesIO(), timeout=1)

    def test_rejeita_slot_dados_ou_sincronizacao_incorretos(self):
        persist = (b"\nM1-VERSION=m1\nM1-SLOT=rauc.slot=A\nM1-DATA=rw\n"
                   b"M1-DOCUMENT=melkor-m1-persistencia\nM1-SYNC=ok\nM1-END\n")
        for target, replacement, error in (
            (b"slot=A", b"slot=B", "slot A falhou"),
            (b"SYNC=ok", b"SYNC=no", "M1-SYNC=ok"),
            (b"DATA=rw", b"DATA=ro", "para escrita"),
            (b"DOCUMENT=melkor-m1-persistencia", b"DOCUMENT=", "M1-DOCUMENT="),
        ):
            with self.subTest(replacement=replacement), console(LOGIN + PROBE + persist.replace(target, replacement)) as process:
                with self.assertRaisesRegex(RuntimeError, error):
                    boot.check_console(process, io.BytesIO(), timeout=1, slot="A")
                self.assertIn(boot.WRITE_DOCUMENT.encode(), process.stdin.getvalue())

    def test_timeout_e_eof_sao_falhas_distintas(self):
        for keep_open, error in ((True, "Prazo"), (False, "Console serial fechado")):
            with self.subTest(keep_open=keep_open), console(b"", keep_open) as process:
                with self.assertRaisesRegex(RuntimeError, error):
                    boot.check_console(process, io.BytesIO(), timeout=0.05)


class BootRunnerTest(unittest.TestCase):
    def setUp(self):
        work = TemporaryDirectory()
        self.addCleanup(work.cleanup)
        self.path = Path(work.name)
        firmware = self.path / "firmware"
        firmware.mkdir()
        self.code = firmware / "OVMF_CODE.fd"
        self.variables = firmware / "OVMF_VARS.fd"
        self.disk = self.path / "disco,virgem.img"
        for file in (self.code, self.variables, self.disk):
            file.write_bytes(file.name.encode())
        self.args = argparse.Namespace(
            image=self.disk, ovmf_code=self.code, ovmf_vars=self.variables, ab=False, live=False, pc=False,
            pc_user_password=None, pc_root_password=None,
            accel="auto", timeout=None,
        )

    def test_procura_firmware_e_exige_par(self):
        with patch.object(boot, "OVMF_DIRS", (self.code.parent,)):
            self.assertEqual(boot.find_firmware(argparse.Namespace(ovmf_code=None, ovmf_vars=None)),
                             (self.code, self.variables))
        self.args.ovmf_vars = None
        with self.assertRaisesRegex(RuntimeError, "mesmo conjunto"):
            boot.find_firmware(self.args)
        self.args.ovmf_vars = self.variables.with_name("ausente")
        with self.assertRaisesRegex(RuntimeError, "Firmware OVMF ausente"):
            boot.find_firmware(self.args)

    def test_m0_abre_imagem_original_somente_para_leitura(self):
        self.check_run(ab=False)

    def test_m1_reutiliza_copia_gravavel_e_variaveis_nos_dois_slots(self):
        self.check_run(ab=True)

    def test_sondas_emitem_campos_inteiros_com_mensagens_assincronas(self):
        tools = self.path / "bin"
        tools.mkdir()
        for name, body in {
            "awk": """case "$2" in
    /proc/cmdline) printf '%s\n' 'rauc.slot=A' ;;
    /proc/mounts) case "$1" in
        *ext4*) printf '%s\n' 'rw,nosuid,nodev' ;;
        *) printf '%s\n' 'squashfs,ro' ;;
    esac ;;
esac
""",
            "cat": """case "$1" in
    /proc/1/comm) printf '%s\n' dinit ;;
    *) printf '%s\n' melkor-m1-persistencia ;;
esac
""",
        }.items():
            executable = tools / name
            executable.write_text("#!/bin/sh\nprintf '%s\n' '[  OK  ] rauc-setup' >&2\n" + body)
            executable.chmod(0o755)

        completed = subprocess.run(["sh", "-c", boot.PROBE + "; " + boot.PERSIST_PROBE],
                                   env=os.environ | {"PATH": f"{tools}:{os.defpath}"},
                                   stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True, check=True)
        lines = completed.stdout.splitlines()
        for field in ("M0-ROOT=squashfs,ro", "M1-SLOT=rauc.slot=A", "M1-DATA=rw,nosuid,nodev",
                      "M1-DOCUMENT=melkor-m1-persistencia"):
            self.assertIn(field, lines)
        self.assertEqual(lines.count("[  OK  ] rauc-setup"), 5)

    def test_live_inicia_por_usb_e_confere_hash_do_nvme(self):
        self.args.live = True
        attached = self.path / "internal-nvme.img"

        def alter_disk(*args):
            with attached.open("r+b") as disk:
                disk.write(b"alterado")

        for check_disk, changed in ((None, False), (alter_disk, True)):
            with self.subTest(altered=changed), patch.dict(os.environ, {"QEMU_SYSTEM": "/test/qemu"}), \
                 patch.object(boot.subprocess, "Popen") as spawn, patch.object(boot.subprocess, "run") as setup_disk, \
                 patch.object(boot, "check_live_console", side_effect=check_disk):
                spawn.return_value.__enter__.return_value.stdout.read.return_value = b"final"
                report = {}
                if changed:
                    with self.assertRaisesRegex(RuntimeError, "disco NVMe virtual foi alterado"):
                        boot.run(self.args, self.path, report)
                else:
                    boot.run(self.args, self.path, report)
                    self.assertEqual(report["internal_sha256_before"], report["internal_sha256_after"])
                command = spawn.call_args.args[0]
                self.assertIn("usb-storage,bus=xhci.0,drive=melkor,bootindex=1", command)
                self.assertIn("nvme,drive=internal,serial=melkor-teste", command)
                self.assertTrue(any("id=melkor,readonly=on" in value for value in command))
                self.assertEqual([call.args[0][0] for call in setup_disk.call_args_list], ["sfdisk", "mke2fs"])
                self.assertTrue(all(attached in call.args[0] for call in setup_disk.call_args_list))
                attached.unlink()

    def test_live_inicia_iso_como_cd_uefi(self):
        self.args.live = True
        self.args.image = self.path / "melkor-live.iso"
        self.args.image.write_bytes(b"iso")
        with patch.dict(os.environ, {"QEMU_SYSTEM": "/test/qemu"}), \
             patch.object(boot.subprocess, "run"), patch.object(boot.subprocess, "Popen") as spawn, \
             patch.object(boot, "check_live_console") as check:
            spawn.return_value.__enter__.return_value.stdout.read.return_value = b"final"
            boot.run(self.args, self.path, {})
        command = spawn.call_args.args[0]
        self.assertIn("ide-cd,bus=ide.0,drive=melkor,bootindex=1", command)
        self.assertTrue(any("id=melkor,media=cdrom" in value for value in command))
        self.assertTrue(check.call_args.args[-1])

    def check_run(self, ab):
        self.args.ab = ab
        report = {}
        process = MagicMock()
        process.poll.return_value = None
        process.stdout.read.return_value = b"final"
        with patch.dict(os.environ, {"QEMU_SYSTEM": "/test/qemu"}), \
             patch.object(boot.os, "access", return_value=ab), \
             patch.object(boot.subprocess, "Popen") as spawn, patch.object(boot, "check_console") as check:
            spawn.return_value.__enter__.return_value = process
            boot.run(self.args, self.path, report)
        self.assertEqual(report["accel"], "kvm" if ab else "tcg")
        self.assertEqual(report["timeout_seconds"], 90 if ab else 240)
        self.assertEqual(process.kill.call_count, 2 if ab else 1)
        self.assertEqual((self.path / "OVMF_VARS.fd").read_bytes(), self.variables.read_bytes())
        command = spawn.call_args.args[0]
        self.assertIn("-nic", command)
        self.assertEqual(command[command.index("-nic") + 1], "none")
        drives = [command[i + 1] for i, val in enumerate(command) if val == "-drive"]
        self.assertIn("readonly=on", drives[0])
        if ab:
            self.assertEqual([call.args[-1] for call in check.call_args_list], ["A", "B"])
            self.assertEqual(spawn.call_args_list[0], spawn.call_args_list[1])
            self.assertEqual(report["verified_slots"], ["A", "B"])
            self.assertEqual((self.path / "disk.img").read_bytes(), self.disk.read_bytes())
            self.assertNotIn("readonly=on", drives[2])
        else:
            self.assertIn("disco,,virgem.img", drives[2])
            self.assertIn("readonly=on", drives[2])
            self.assertEqual((self.path / "serial.log").read_bytes(), b"final")

    def test_cli_grava_relatorio_quando_imagem_falta(self):
        report = self.path / "reports"
        with patch.object(sys, "argv", ["test-boot", str(self.path / "missing"), "--log-dir", str(report)]), \
             redirect_stdout(io.StringIO()), redirect_stderr(io.StringIO()):
            self.assertEqual(boot.main(), 1)
        result, = report.glob("boot-*/result.json")
        fields = json.loads(result.read_text())
        self.assertEqual(fields["exit_code"], 1)
        self.assertIn("Imagem de disco virtual ausente", fields["error"])

    def test_cli_grava_sucesso_e_rejeita_timeout_invalido(self):
        report = self.path / "reports"
        with redirect_stdout(io.StringIO()), redirect_stderr(io.StringIO()):
            with patch.object(sys, "argv", ["test-boot", str(self.disk), "--ab", "--log-dir", str(report)]), \
                 patch.object(boot, "run"):
                self.assertEqual(boot.main(), 0)
            with patch.object(sys, "argv", ["test-boot", str(self.disk), "--timeout", "0"]):
                with self.assertRaises(SystemExit) as error:
                    boot.main()
                self.assertEqual(error.exception.code, 2)


class ImageCheckTest(unittest.TestCase):
    def setUp(self):
        work = TemporaryDirectory()
        self.addCleanup(work.cleanup)
        self.build = Path(work.name)
        (self.build / "images").mkdir()
        (self.build / "images/melkor-m1.img").touch()
        self.uuid = "data-id"
        self.table = {"label": "gpt", "sectorsize": 512, "partitions": []}
        self.types = {}
        start = 2048
        for index, (name, mib, fstype) in enumerate((
            ("esp", 64, "vfat"), ("core-a", 128, "squashfs"),
            ("core-b", 128, "squashfs"), ("persist", 128, "ext4"),
        )):
            ptype = image.LINUX_TYPE if index else image.ESP_TYPE
            size = mib * image.MIB // 512
            self.table["partitions"].append(dict(name=name, start=start, size=size, type=ptype, uuid=f"slot-{index}"))
            self.types[start * 512] = fstype
            start += size
        self.limine = "".join(
            f"/Melkor Linux - slot {slot}\n    path: boot():/EFI/BOOT/kernel-{slot}\n"
            f"    cmdline: root=PARTUUID=slot-{i} ro rauc.slot={slot}\n"
            for i, slot in enumerate(("A", "B"), start=1)
        ).encode()
        loader = self.build / "host/share/melkor-limine/BOOTX64.EFI"
        loader.parent.mkdir(parents=True)
        loader.write_bytes(b"efi")
        (self.build / "images/bzImage").write_bytes(b"kernel")
        self.persist = f"mount -t ext4 UUID={self.uuid} /persist"
        self.setup = "slot-0 slot-1 slot-2"
        self.rauc = "bundle-formats=verity\n"
        self.resolver = "nameserver 127.0.0.1\nnameserver ::1\n"
        self.dns = json.dumps({
            "listen-addrs": ["127.0.0.1", "::1"],
            "listen-ports": [53],
            "upstream": [image.quad9_stamp(address) for address in image.QUAD9_ADDRESSES],
        })

    def command(self, argv, text=False):
        if argv[:2] == ("sfdisk", "--json"):
            return json.dumps({"partitiontable": self.table})
        if argv[0] == "blkid":
            return f"TYPE={self.types[int(argv[5])]}\nUUID={self.uuid}\n"
        if argv[0] == self.build / "host/bin/mtype":
            self.assertFalse(text)
            return {"BOOTX64.EFI": b"efi", "limine.conf": self.limine,
                    "kernel-A": b"kernel", "kernel-B": b"kernel"}[Path(argv[-1]).name]
        self.assertTrue(text)
        self.assertEqual(argv[0], self.build / "host/bin/unsquashfs")
        return {
            "/etc/os-release": 'VERSION_ID="m1"',
            "/usr/libexec/melkor-persist": self.persist,
            "/usr/libexec/melkor-rauc-setup": self.setup,
            "/etc/rauc/system.conf": self.rauc,
            "/etc/resolv.conf": self.resolver,
            "/etc/dnsproxy.json": self.dns,
        }[argv[-1]]

    def test_valida_gpt_slots_e_persistencia(self):
        with patch.object(image.subprocess, "check_output", side_effect=self.command) as capture, \
             patch.object(image.subprocess, "run") as run:
            image.check(self.build)
            run.assert_called_once_with(["sfdisk", "--verify", self.build / "images/melkor-m1.img"], check=True)
        self.assertEqual(sum(call.args[0][-1] in ("::/EFI/BOOT/kernel-A", "::/EFI/BOOT/kernel-B") for call in capture.call_args_list), 2)

    def test_rejeita_particao_ou_slot_invalido(self):
        with patch.object(image.subprocess, "check_output", side_effect=self.command), \
             patch.object(image.subprocess, "run"):
            with patch.dict(self.table, label="dos"):
                with self.assertRaisesRegex(ValueError, "GPT"):
                    image.check(self.build)
            start = self.table["partitions"][2]["start"]
            with patch.dict(self.table["partitions"][2], start=start + 1):
                with self.assertRaisesRegex(ValueError, "Alinhamento"):
                    image.check(self.build)
            with patch.dict(self.types, {start * 512: "ext4"}):
                with self.assertRaisesRegex(ValueError, "Sistema de arquivos"):
                    image.check(self.build)
            with patch.object(self, "limine", self.limine.replace(b"slot-1", b"slot-2")):
                with self.assertRaisesRegex(ValueError, "PARTUUID"):
                    image.check(self.build)
            with patch.object(self, "persist", "mount -t ext4 UUID=outro /persist"):
                with self.assertRaisesRegex(ValueError, "UUID"):
                    image.check(self.build)
            with patch.object(self, "limine", b"/Melkor Linux - slot B\n"):
                with self.assertRaisesRegex(ValueError, "Ordem inicial"):
                    image.check(self.build)
            with patch.object(self, "setup", "slot-1 slot-2"):
                with self.assertRaisesRegex(ValueError, "ESP"):
                    image.check(self.build)
            with patch.object(self, "rauc", "bundle-formats=plain\n"):
                with self.assertRaisesRegex(ValueError, "verity"):
                    image.check(self.build)

    def test_exige_quad9_doh_em_loopback_nos_dois_slots(self):
        with patch.object(image.subprocess, "check_output", side_effect=self.command) as capture, \
             patch.object(image.subprocess, "run"):
            image.check(self.build)
            self.assertEqual(sum(call.args[0][-1] == "/etc/dnsproxy.json" for call in capture.call_args_list), 2)
            with patch.object(self, "resolver", self.resolver + "nameserver 192.0.2.53\n"):
                with self.assertRaisesRegex(ValueError, "loopback"):
                    image.check(self.build)
            for override in ({"bootstrap": ["192.0.2.53"]}, {"fallback": ["192.0.2.53"]},
                             {"listen-addrs": ["0.0.0.0"]}, {"upstream": ["9.9.9.9"]}):
                with patch.object(self, "dns", json.dumps(json.loads(self.dns) | override)):
                    with self.assertRaisesRegex(ValueError, "Quad9"):
                        image.check(self.build)

    def test_cli_relata_falha_de_ferramenta(self):
        with patch.object(sys, "argv", ["check-m1", str(self.build)]), \
             patch.object(image, "check", side_effect=subprocess.CalledProcessError(1, "sfdisk")), \
             redirect_stderr(io.StringIO()) as stderr:
            self.assertEqual(image.main(), 1)
            self.assertIn("sfdisk", stderr.getvalue())
