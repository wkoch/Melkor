#!/usr/bin/env python3
"""Faz login no console de uma VM UEFI e registra os resultados no host."""

import argparse
from contextlib import contextmanager
import hashlib
import json
import os
from pathlib import Path
import re
import selectors
import shlex
import shutil
import subprocess
import sys
import tempfile
import time
from types import SimpleNamespace


OVMF_DIRS = (Path("/usr/share/edk2/ovmf"), Path("/usr/share/OVMF"))
# Evita binários de montagens temporárias que aparecem no PATH do host.
SYSTEM_PATH = "/usr/bin:/usr/local/bin"
OVMF_NAMES = (("OVMF_CODE_4M.fd", "OVMF_VARS_4M.fd"), ("OVMF_CODE.fd", "OVMF_VARS.fd"))

PROBE = (
    '. /etc/os-release; '
    'printf "\\nM0-ID=%s\\n" "$ID"; '
    'printf "M0-PID1=%s\\n" "$(cat /proc/1/comm)"; '
    'printf "M0-LOGIN=%s\\n" "$(id -un)"; '
    'printf "M0-ROOT="; awk \'$2 == "/" {print $3 "," $4; exit}\' /proc/mounts; '
    '[ -d /sys/firmware/efi ] && printf "M0-UEFI=yes\\n"; '
    'printf "M0-END\\n"'
)
PERSIST_PROBE = (
    '. /etc/os-release; printf "\\nM1-VERSION=%s\\n" "$VERSION_ID"; '
    'printf "M1-SLOT="; awk \'{for (i=1; i<=NF; i++) if ($i ~ /^rauc[.]slot=/) print $i}\' /proc/cmdline; '
    'printf "M1-DATA="; awk \'$2 == "/persist" && $3 == "ext4" {print $4; exit}\' /proc/mounts; '
    'printf "M1-DOCUMENT="; cat "$HOME/.melkor-m1-probe"; '
    'printf "\\nM1-END\\n"'
)
WRITE_DOCUMENT = (
    'printf "melkor-m1-persistencia\\n" > "$HOME/.melkor-m1-probe" && sync '
    '&& printf "\\nM1-SYNC=ok\\n"; '
)
PC_SWAP_PROBE = (
    "for attempt in 1 2 3 4 5; do "
    "awk '$1 == \"/persist/.swap/swapfile\" && $2 == \"file\" && $3 >= 4190000 "
    "{ok=1} END {exit !ok}' /proc/swaps && { printf \"\\nPC-SWAP=ok\\n\"; break; }; sleep 1; done; "
)
LIVE_PROBE = (
    '. /etc/os-release; printf "\\nLIVE-VERSION=%s\\n" "$VERSION_ID"; '
    '[ "$(id -u)" -eq 1000 ] && printf "LIVE-USER=ok\\n"; '
    'grep -q "frame buffer" /sys/class/vtconsole/vtcon*/name && printf "LIVE-DISPLAY=ok\\n"; '
    'for attempt in 1 2 3 4 5; do live_media && [ -b /dev/nvme0n1p1 ] && break; sleep 1; done; '
    'live_media && [ -b /dev/nvme0n1p1 ] && printf "LIVE-DEVICES=ok\\n"; '
    '[ ! -w /dev/sda ] && [ ! -w /dev/nvme0n1 ] && printf "LIVE-PERMISSIONS=ok\\n"; '
    'lspci >/dev/null && lsusb >/dev/null && /usr/sbin/iw dev >/dev/null && printf "LIVE-TOOLS=ok\\n"; '
    'awk \'$2 == "/" && $3 ~ /^(rootfs|ramfs|tmpfs)$/ {ram=1} '
    '$1 ~ /^\\/dev\\// {disk=1} END {if (ram && !disk) print "LIVE-RAM=ok"}\' /proc/mounts; '
    'printf "ram\\n" > "$HOME/.melkor-live-probe" && printf "LIVE-HOME=ok\\n"; '
    'printf "LIVE-END\\n"'
)
LIVE_WIFI_PROBE = (
    'cd /lib/firmware && [ -s iwlwifi-cc-a0-77.ucode ] && [ -s regulatory.db ] '
    '&& [ -s regulatory.db.p7s ] && printf "\\nLIVE-FIRMWARE=ok\\n"; '
    'kernel_log=$(dmesg) && ! printf "%s\\n" "$kernel_log" | '
    'grep -Ei "failed to load regulatory[.]db|regulatory[.]db.*(malformed|invalid|error)" '
    '&& /usr/sbin/iw reg get | grep -q "^country 00:" && printf "LIVE-REGDB=ok\\n"; '
    'for attempt in 1 2 3 4 5; do '
    'dbus-send --system --print-reply --dest=org.freedesktop.DBus /org/freedesktop/DBus '
    'org.freedesktop.DBus.NameHasOwner string:net.connman.iwd 2>/dev/null | grep -q "boolean true" '
    '&& { printf "\\nLIVE-IWD=ok\\n"; break; }; sleep 1; done; '
    'printf "\\nLIVE-WIFI-END\\n"'
)


def file_digest(path):
    with path.open("rb") as stream:
        return hashlib.file_digest(stream, "sha256").hexdigest()


def find_firmware(args):
    if args.ovmf_code or args.ovmf_vars:
        if not (args.ovmf_code and args.ovmf_vars):
            raise RuntimeError("Informe OVMF_CODE e OVMF_VARS do mesmo conjunto.")
        pairs = [(args.ovmf_code, args.ovmf_vars)]
    else:
        pairs = [(directory / code, directory / variables)
                 for directory in OVMF_DIRS for code, variables in OVMF_NAMES]

    for code, variables in pairs:
        if code.is_file() and variables.is_file():
            return code.resolve(), variables.resolve()
    raise RuntimeError("Firmware OVMF ausente. Informe OVMF_CODE e OVMF_VARS em formato raw.")


@contextmanager
def serial_console(process, log, timeout):
    deadline = time.monotonic() + timeout
    pending = b""

    with selectors.DefaultSelector() as serial:
        serial.register(process.stdout, selectors.EVENT_READ)

        def expect(pattern, stage, *, wait=None):
            nonlocal pending
            limit = deadline if wait is None else min(deadline, time.monotonic() + wait)
            match = re.search(pattern, pending, re.MULTILINE)
            while not match:
                remaining = limit - time.monotonic()
                if remaining <= 0 or not serial.select(remaining):
                    if wait is not None:
                        return None
                    raise RuntimeError(f"Prazo de {timeout}s excedido ao aguardar {stage}.")
                chunk = os.read(process.stdout.fileno(), 65536)
                if not chunk:
                    if wait is not None:
                        return None
                    raise RuntimeError(f"Console serial fechado durante {stage}.")
                log.write(chunk)
                log.flush()
                pending += chunk
                match = re.search(pattern, pending, re.MULTILINE)
            received, pending = pending[:match.end()], pending[match.end():]
            return received.replace(b"\r", b"")

        def send(line, end=b"\r"):
            process.stdin.write(line + end)
            process.stdin.flush()

        yield SimpleNamespace(expect=expect, send=send)


def login(console, user, password, prompt):
    # O console envia "\n\r" antes do prompt.
    console.expect(rb"^\r?melkor login: ?", "prompt de login")
    console.send(user)
    console.expect(rb"^\r?Password: ?", "prompt de senha")
    console.send(password)
    console.expect(prompt, "shell do convidado")


def check_console(process, log, timeout, slot=None, pc=False, credentials=None):
    with serial_console(process, log, timeout) as console:
        if slot == "B":
            label = b"Melkor Linux PC - slot " if pc else b"Melkor Linux M1 - slot "
            selection = console.expect(rb"\*" + label + rb"[AB]", "menu de slots do GRUB")
            # A primeira marca aparece durante o desenho. Aguarde a contagem quando existir.
            console.expect(rb"The highlighted entry will be executed automatically in",
                           "contagem opcional do GRUB", wait=1)
            if selection.endswith(b"*" + label + b"A"):
                console.send(b"\x1b[B", end=b"")
                selection = console.expect(rb"\*" + label + rb"B|Booting `" + label + rb"A'",
                                           "seleção do slot B no GRUB")
                if not selection.endswith(b"*" + label + b"B"):
                    raise RuntimeError("O GRUB iniciou o slot A antes de confirmar a seleção de B.")
            console.send(b"")

        if credentials:
            console.expect(rb"^\r?melkor login: ?", "prompt antes de testar a credencial antiga")
            console.send(b"melkor")
            console.expect(rb"^\r?Password: ?", "prompt da senha antiga")
            console.send(b"melkor")
            console.expect(rb"^\r?Login incorrect\r?$", "recusa da senha antiga")
            login(console, b"melkor", credentials[0], rb"^[^\n]*\$ ")
            console.send(b'test ! -r /etc/shadow && test ! -r /persist/.melkor/credenciais '
                         b'&& test ! -w /etc && test ! -w /dev/nvme0n1 && printf "\\nPC-USER=ok\\n"')
            console.expect(rb"^PC-USER=ok\r?$", "limites da conta comum")
            console.send(b"su -")
            console.expect(rb"^\r?Password: ?", "senha separada de root")
            console.send(credentials[1])
            console.expect(rb"^[^\n]*# ", "sessão root")
            console.send(b'test "$(id -u)" = 0 && ls -ln /etc/shadow | '
                         b'awk \'$1 == "-rw-------" && $3 == 0 && $4 == 0 {print "PC-ROOT=ok"}\'')
            console.expect(rb"^PC-ROOT=ok\r?$", "conta root e permissões do shadow")
            console.send(b"exit")
            console.expect(rb"^[^\n]*\$ ", "retorno à conta comum")
        else:
            login(console, b"melkor", b"melkor", rb"^[^\n]*\$ ")
        console.send(PROBE.encode("ascii"))
        result = console.expect(rb"^M0-END\r?$", "resposta das verificações")
        if slot:
            probe = (WRITE_DOCUMENT if slot == "A" else "") + PERSIST_PROBE
            if pc:
                probe = PC_SWAP_PROBE + probe
            console.send(probe.encode("ascii"))
            persistent = console.expect(rb"^M1-END\r?$", "verificação de persistência A/B")

    lines = result.splitlines()
    for expected in (b"M0-ID=melkor", b"M0-PID1=dinit", b"M0-LOGIN=melkor", b"M0-UEFI=yes"):
        if expected not in lines:
            raise RuntimeError(f"Valor ausente no console: {expected.decode('ascii')}.")

    root = re.search(rb"^M0-ROOT=squashfs,([^\n]+)$", result, re.MULTILINE)
    if root is None or b"ro" not in root[1].split(b","):
        raise RuntimeError("A raiz do convidado precisa estar em SquashFS, montada somente para leitura.")

    if slot:
        expected = [b"M1-VERSION=pc" if pc else b"M1-VERSION=m1", f"M1-SLOT=rauc.slot={slot}".encode("ascii"),
                    b"M1-DOCUMENT=melkor-m1-persistencia"]
        if pc:
            expected.append(b"PC-SWAP=ok")
        if slot == "A":
            expected.append(b"M1-SYNC=ok")
        for field in expected:
            if field not in persistent.splitlines():
                raise RuntimeError(f"Verificação do slot {slot} falhou: {field.decode('ascii')}.")
        data = re.search(rb"^M1-DATA=([^\n]+)$", persistent, re.MULTILINE)
        if data is None or b"rw" not in data[1].split(b","):
            raise RuntimeError("A partição ext4 de dados precisa estar montada para escrita.")


def check_live_console(process, log, timeout, optical=False):
    with serial_console(process, log, timeout) as console:
        login(console, b"melkor", b"melkor", rb"^[^\n]*\$ ")
        console.send(PROBE.encode("ascii"))
        system = console.expect(rb"^M0-END\r?$", "verificação do boot UEFI")
        media = "grep -qx 5 /sys/bus/scsi/devices/*/type" if optical else "[ -b /dev/sda ]"
        console.send(f"live_media() {{ {media}; }}; {LIVE_PROBE}".encode("ascii"))
        live = console.expect(rb"^LIVE-END\r?$", "verificação do ambiente em RAM")
        console.send(LIVE_WIFI_PROBE.encode("ascii"))
        wifi = console.expect(rb"^LIVE-WIFI-END\r?$", "verificação de Wi-Fi")
    lines = system.splitlines() + live.splitlines() + wifi.splitlines()
    for field in (b"M0-ID=melkor", b"M0-PID1=dinit", b"M0-UEFI=yes", b"LIVE-VERSION=live", b"LIVE-USER=ok",
                  b"LIVE-DISPLAY=ok", b"LIVE-DEVICES=ok", b"LIVE-PERMISSIONS=ok", b"LIVE-TOOLS=ok",
                  b"LIVE-RAM=ok", b"LIVE-HOME=ok", b"LIVE-FIRMWARE=ok", b"LIVE-REGDB=ok", b"LIVE-IWD=ok"):
        if field not in lines:
            raise RuntimeError(f"Verificação da mídia live falhou: {field.decode('ascii')}.")


def create_live_test_disk(path):
    with path.open("xb") as disk:
        disk.truncate(64 << 20)
    # Um volume ext4 válido exercita o risco de uma montagem automática acidental.
    subprocess.run(["sfdisk", "--no-reread", "--no-tell-kernel", path],
                   input="label: gpt\nstart=2048, size=65536, type=L\n", text=True, check=True,
                   stdout=subprocess.DEVNULL)
    subprocess.run(["mke2fs", "-q", "-F", "-t", "ext4", "-L", "melkor-data", "-b", "4096",
                    "-E", "offset=1048576", path, "8192"], check=True)


def run(args, report_dir, report):
    image = args.image.resolve()
    if not image.is_file():
        raise RuntimeError(f"Imagem de disco virtual ausente: {image}. Execute make image.")
    report.update(image=str(image), image_sha256=file_digest(image))
    credentials = None
    if args.pc_user_password is not None:
        credentials = tuple(path.read_bytes().strip() for path in (args.pc_user_password, args.pc_root_password))
        if any(not re.fullmatch(rb"[0-9a-f]{32}", password) for password in credentials) or credentials[0] == credentials[1]:
            raise RuntimeError("Use senhas distintas e descartáveis geradas por openssl rand -hex 16.")

    qemu = os.environ.get("QEMU_SYSTEM") or shutil.which("qemu-system-x86_64", path=SYSTEM_PATH)
    if qemu is None:
        raise RuntimeError("Instale qemu-system-x86_64 para executar o teste.")
    code, variables = find_firmware(args)
    variables_copy = report_dir / "OVMF_VARS.fd"
    shutil.copyfile(variables, variables_copy)
    disk = image
    if args.pc:
        disk = report_dir / "disk.img"
        subprocess.run(["cp", "--reflink=auto", "--sparse=always", "--", image, disk], check=True)
    elif args.ab:
        disk = report_dir / "disk.img"
        shutil.copyfile(image, disk)

    accel = args.accel
    if accel == "auto":
        accel = "kvm" if not args.live and os.access("/dev/kvm", os.R_OK | os.W_OK) else "tcg"
    timeout = args.timeout or (90 if accel == "kvm" else 240)
    optical = args.live and disk.suffix.lower() == ".iso"

    def drive(path, options):
        return f"file={str(path).replace(',', ',,')},format=raw,{options}"

    command = [qemu, "-machine", "q35", "-accel", accel, "-m", "512", "-display", "none",
               "-serial", "stdio", "-monitor", "none", "-nic", "none", "-no-reboot",
               "-drive", drive(code, "if=pflash,unit=0,readonly=on"),
               "-drive", drive(variables_copy, "if=pflash,unit=1")]
    media_options = "if=none,id=melkor"
    if optical:
        media_options += ",media=cdrom"
    elif not (args.ab or args.pc):
        media_options += ",readonly=on"
    command += ["-drive", drive(disk, media_options)]
    if args.live:
        attached = report_dir / "internal-nvme.img"
        create_live_test_disk(attached)
        report["internal_sha256_before"] = file_digest(attached)
        if optical:
            command += ["-device", "ide-cd,bus=ide.0,drive=melkor,bootindex=1"]
        else:
            command += ["-device", "qemu-xhci,id=xhci", "-device", "usb-storage,bus=xhci.0,drive=melkor,bootindex=1"]
        command += ["-drive", drive(attached, "if=none,id=internal"),
                    "-device", "nvme,drive=internal,serial=melkor-teste"]
    elif args.pc:
        command += ["-device", "nvme,drive=melkor,serial=melkor-teste,bootindex=1"]
    else:
        command += ["-device", "virtio-blk-pci,drive=melkor,bootindex=1"]

    report.update(accel=accel, timeout_seconds=timeout, command=shlex.join(command),
                  ovmf_code_sha256=file_digest(code), ovmf_vars_sha256=file_digest(variables))
    for slot in ("A", "B") if args.ab or args.pc else (None,):
        log_name = f"serial-{slot.lower()}.log" if slot else "serial.log"
        with (report_dir / log_name).open("wb") as log, subprocess.Popen(
            command, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.STDOUT
        ) as process:
            try:
                if args.live:
                    check_live_console(process, log, timeout, optical)
                else:
                    check_console(process, log, timeout, slot, pc=args.pc, credentials=credentials)
            finally:
                if process.poll() is None:
                    process.kill()
                log.write(process.stdout.read())
        if slot:
            report.setdefault("verified_slots", []).append(slot)
    if args.live:
        report["internal_sha256_after"] = file_digest(attached)
        if report["internal_sha256_before"] != report["internal_sha256_after"]:
            raise RuntimeError("O disco NVMe virtual foi alterado durante o boot live.")


def main():
    version = subprocess.run(["git", "rev-parse", "--verify", "HEAD"], capture_output=True, text=True)
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("image", type=Path)
    mode = parser.add_mutually_exclusive_group()
    mode.add_argument("--ab", action="store_true", help="Inicia A e B e verifica o mesmo documento.")
    mode.add_argument("--pc", action="store_true", help="Verifica A/B, dados e arquivo swap no disco PC.")
    mode.add_argument("--live", action="store_true", help="Inicia por USB ou CD UEFI e verifica o NVMe virtual.")
    parser.add_argument("--log-dir", type=Path, default=Path("out/boot-tests"))
    parser.add_argument("--ovmf-code", type=Path, default=os.environ.get("OVMF_CODE"))
    parser.add_argument("--ovmf-vars", type=Path, default=os.environ.get("OVMF_VARS"))
    parser.add_argument("--accel", choices=("auto", "kvm", "tcg"), default=os.environ.get("QEMU_ACCEL", "auto"))
    parser.add_argument("--timeout", type=int)
    parser.add_argument("--pc-user-password", type=Path, help="Arquivo com a senha descartável do usuário.")
    parser.add_argument("--pc-root-password", type=Path, help="Arquivo com a senha descartável de root.")
    args = parser.parse_args()
    if args.timeout is not None and args.timeout <= 0:
        parser.error("--timeout precisa ser maior que zero.")
    if args.accel not in ("auto", "kvm", "tcg"):
        parser.error("QEMU_ACCEL deve ser auto, kvm ou tcg.")
    passwords = (args.pc_user_password, args.pc_root_password)
    if any(passwords) and (not args.pc or not all(passwords)):
        parser.error("Informe ambos os arquivos de senha e use --pc.")

    args.log_dir.mkdir(parents=True, exist_ok=True)
    report_dir = Path(tempfile.mkdtemp(prefix="boot-", dir=args.log_dir.resolve()))
    report = {"commit": version.stdout.strip() or None, "exit_code": 1}
    try:
        run(args, report_dir, report)
        report["exit_code"] = 0
        if args.ab or args.pc:
            print("Slots A e B iniciados via UEFI. Documento criado em A e reaberto em B.")
        elif args.live:
            print("Boot da mídia via UEFI, framebuffer, login em RAM e NVMe virtual intacto verificados.")
        else:
            print("Login serial UEFI, identidade Melkor, dinit como PID 1 e raiz somente para leitura verificados.")
    except (OSError, RuntimeError, subprocess.CalledProcessError) as error:
        report["error"] = str(error)
        print(error, file=sys.stderr)
    finally:
        (report_dir / "result.json").write_text(json.dumps(report, indent=2) + "\n", encoding="utf-8")
        print(f"Evidências em {report_dir}")
    return report["exit_code"]


if __name__ == "__main__":
    sys.exit(main())
