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


def check_console(process, log, timeout, slot=None):
    with serial_console(process, log, timeout) as console:
        if slot == "B":
            selection = console.expect(rb"\*Melkor Linux M1 - slot [AB]", "menu de slots do GRUB")
            # A primeira marca aparece durante o desenho. Aguarde a contagem quando existir.
            console.expect(rb"The highlighted entry will be executed automatically in",
                           "contagem opcional do GRUB", wait=1)
            if selection.endswith(b"*Melkor Linux M1 - slot A"):
                console.send(b"\x1b[B", end=b"")
                selection = console.expect(rb"\*Melkor Linux M1 - slot B|Booting `Melkor Linux M1 - slot A'",
                                           "seleção do slot B no GRUB")
                if not selection.endswith(b"*Melkor Linux M1 - slot B"):
                    raise RuntimeError("O GRUB iniciou o slot A antes de confirmar a seleção de B.")
            console.send(b"")

        login(console, b"melkor", b"melkor", rb"^[^\n]*\$ ")
        console.send(PROBE.encode("ascii"))
        result = console.expect(rb"^M0-END\r?$", "resposta das verificações")
        if slot:
            probe = (WRITE_DOCUMENT if slot == "A" else "") + PERSIST_PROBE
            console.send(probe.encode("ascii"))
            persistent = console.expect(rb"^M1-END\r?$", "verificação de persistência do M1")

    lines = result.splitlines()
    for expected in (b"M0-ID=melkor", b"M0-PID1=dinit", b"M0-LOGIN=melkor", b"M0-UEFI=yes"):
        if expected not in lines:
            raise RuntimeError(f"Valor ausente no console: {expected.decode('ascii')}.")

    root = re.search(rb"^M0-ROOT=squashfs,([^\n]+)$", result, re.MULTILINE)
    if root is None or b"ro" not in root[1].split(b","):
        raise RuntimeError("A raiz do convidado precisa estar em SquashFS, montada somente para leitura.")

    if slot:
        expected = [b"M1-VERSION=m1", f"M1-SLOT=rauc.slot={slot}".encode("ascii"),
                    b"M1-DOCUMENT=melkor-m1-persistencia"]
        if slot == "A":
            expected.append(b"M1-SYNC=ok")
        for field in expected:
            if field not in persistent.splitlines():
                raise RuntimeError(f"Verificação do slot {slot} falhou: {field.decode('ascii')}.")
        data = re.search(rb"^M1-DATA=([^\n]+)$", persistent, re.MULTILINE)
        if data is None or b"rw" not in data[1].split(b","):
            raise RuntimeError("A partição ext4 de dados precisa estar montada para escrita.")


def run(args, report_dir, report):
    image = args.image.resolve()
    if not image.is_file():
        raise RuntimeError(f"Imagem de disco virtual ausente: {image}. Execute make image.")
    report.update(image=str(image), image_sha256=file_digest(image))

    qemu = os.environ.get("QEMU_SYSTEM") or shutil.which("qemu-system-x86_64", path=SYSTEM_PATH)
    if qemu is None:
        raise RuntimeError("Instale qemu-system-x86_64 para executar o teste.")
    code, variables = find_firmware(args)
    variables_copy = report_dir / "OVMF_VARS.fd"
    shutil.copyfile(variables, variables_copy)
    disk = image
    if args.ab:
        disk = report_dir / "disk.img"
        shutil.copyfile(image, disk)

    accel = args.accel
    if accel == "auto":
        accel = "kvm" if os.access("/dev/kvm", os.R_OK | os.W_OK) else "tcg"
    timeout = args.timeout or (90 if accel == "kvm" else 240)

    def drive(path, options):
        return f"file={str(path).replace(',', ',,')},format=raw,{options}"

    command = [qemu, "-machine", "q35", "-accel", accel, "-m", "512", "-display", "none",
               "-serial", "stdio", "-monitor", "none", "-nic", "none", "-no-reboot",
               "-drive", drive(code, "if=pflash,unit=0,readonly=on"),
               "-drive", drive(variables_copy, "if=pflash,unit=1"),
               "-drive", drive(disk, "if=none,id=melkor" if args.ab else "if=none,id=melkor,readonly=on"),
               "-device", "virtio-blk-pci,drive=melkor,bootindex=1"]

    report.update(accel=accel, timeout_seconds=timeout, command=shlex.join(command),
                  ovmf_code_sha256=file_digest(code), ovmf_vars_sha256=file_digest(variables))
    for slot in ("A", "B") if args.ab else (None,):
        log_name = f"serial-{slot.lower()}.log" if slot else "serial.log"
        with (report_dir / log_name).open("wb") as log, subprocess.Popen(
            command, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.STDOUT
        ) as process:
            try:
                check_console(process, log, timeout, slot)
            finally:
                if process.poll() is None:
                    process.kill()
                log.write(process.stdout.read())
        if slot:
            report.setdefault("verified_slots", []).append(slot)


def main():
    version = subprocess.run(["git", "rev-parse", "--verify", "HEAD"], capture_output=True, text=True)
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("image", type=Path)
    parser.add_argument("--ab", action="store_true", help="Inicia A e B e verifica o mesmo documento.")
    parser.add_argument("--log-dir", type=Path, default=Path("out/boot-tests"))
    parser.add_argument("--ovmf-code", type=Path, default=os.environ.get("OVMF_CODE"))
    parser.add_argument("--ovmf-vars", type=Path, default=os.environ.get("OVMF_VARS"))
    parser.add_argument("--accel", choices=("auto", "kvm", "tcg"), default=os.environ.get("QEMU_ACCEL", "auto"))
    parser.add_argument("--timeout", type=int)
    args = parser.parse_args()
    if args.timeout is not None and args.timeout <= 0:
        parser.error("--timeout precisa ser maior que zero.")
    if args.accel not in ("auto", "kvm", "tcg"):
        parser.error("QEMU_ACCEL deve ser auto, kvm ou tcg.")

    args.log_dir.mkdir(parents=True, exist_ok=True)
    report_dir = Path(tempfile.mkdtemp(prefix="boot-", dir=args.log_dir.resolve()))
    report = {"commit": version.stdout.strip() or None, "exit_code": 1}
    try:
        run(args, report_dir, report)
        report["exit_code"] = 0
        if args.ab:
            print("Slots A e B iniciados via UEFI. Documento criado em A e reaberto em B.")
        else:
            print("Login serial UEFI, identidade Melkor, dinit como PID 1 e raiz somente para leitura verificados.")
    except (OSError, RuntimeError) as error:
        report["error"] = str(error)
        print(error, file=sys.stderr)
    finally:
        (report_dir / "result.json").write_text(json.dumps(report, indent=2) + "\n", encoding="utf-8")
        print(f"Evidências em {report_dir}")
    return report["exit_code"]


if __name__ == "__main__":
    sys.exit(main())
