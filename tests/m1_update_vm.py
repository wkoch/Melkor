#!/usr/bin/env python3
"""Exercita a instalação assinada M1 em discos virtuais descartáveis."""

import argparse
from contextlib import contextmanager
import json
import os
from pathlib import Path
import re
import runpy
import shlex
import shutil
import subprocess
import tempfile
import time
from types import SimpleNamespace


SCRIPTS = Path(__file__).resolve().parents[1] / "scripts"
BOOT = runpy.run_path(str(SCRIPTS / "test-boot.py"))
create_bundle = runpy.run_path(str(SCRIPTS / "bundle-m1.py"))["create_bundle"]
GRUBENV = "/run/melkor/esp/EFI/BOOT/grubenv"
DOCUMENT = "/home/melkor/persistencia-atualizacao"
INPUT = "/persist/rauc/test-bundle.raucb"
UPDATE = f"/usr/sbin/melkor-core-update {INPUT}"


def fixture_images(build, work):
    host = build / "host/bin"
    source = build / "images/rootfs.squashfs"
    empty = work / "empty"
    empty.mkdir()
    pseudo = work / "original.pseudo"
    subprocess.run([host / "unsquashfs", "-processors", "2", "-pf", pseudo, source], check=True)
    header, separator, payload = pseudo.read_bytes().partition(b"# START OF DATA - DO NOT MODIFY\n")
    assert separator, "Metadados SquashFS ausentes."
    filename = b"etc/dinit.d/rauc-confirm"
    lines = header.splitlines(keepends=True)
    filtered = [line for line in lines if not line.startswith(filename + b" ")]
    assert len(filtered) == len(lines) - 1, "Serviço de confirmação ausente."
    unhealthy = work / "unhealthy.pseudo"
    unhealthy.write_bytes(b"".join(filtered) + separator + payload)

    for name, metadata in (("healthy", pseudo), ("unhealthy", unhealthy)):
        command = [host / "mksquashfs", empty, work / f"{name}.squashfs", "-noappend", "-processors", "2",
                   "-no-progress", "-comp", "gzip", "-pf", metadata, "-p",
                   f"etc/melkor-test-generation f 0644 0 0 echo {name}"]
        if name == "unhealthy":
            command.extend(("-p", "etc/dinit.d/rauc-confirm f 0644 0 0 "
                                  "printf 'type = scripted\\ncommand = /bin/false\\n"
                                  "depends-on = rauc-setup\\ndepends-on = serial-console\\n'"))
        subprocess.run(command, check=True, stdout=subprocess.DEVNULL)


def seed_disk(args, work):
    fixture_images(args.build, work)
    seed = work / "bundles"
    seed.mkdir()
    rauc = args.build / "host/bin/rauc"
    for name, version in (("healthy", "2.0-test"), ("unhealthy", "3.0-test")):
        create_bundle(rauc, work / f"{name}.squashfs", args.cert, args.key, args.cert, version, seed / f"{name}.raucb")
    create_bundle(rauc, work / "healthy.squashfs", args.cert, args.key, args.cert, "2.0-test", seed / "incompatible.raucb",
                  compatible="other-m1")

    data = bytearray((seed / "healthy.raucb").read_bytes())
    signed = len(data) - int.from_bytes(data[-8:], "big") - 8
    assert 4096 < signed < len(data) - 8
    data[signed + (len(data) - 8 - signed) // 2] ^= 1
    (seed / "bad-signature.raucb").write_bytes(data)
    data = bytearray((seed / "healthy.raucb").read_bytes())
    data[4096] ^= 1
    (seed / "corrupted.raucb").write_bytes(data)

    disk = work / "seed.ext4"
    size = sum(path.stat().st_size for path in seed.iterdir()) + 16 * 1024 * 1024
    with disk.open("wb") as stream:
        stream.truncate(((size + 4095) // 4096) * 4096)
    subprocess.run(["mke2fs", "-q", "-F", "-t", "ext4", "-d", seed, disk], check=True)
    return disk


@contextmanager
def machine(work, stage, disk, seed, code, variables, timeout, accel, write_limit=False):
    binary = os.environ.get("QEMU_SYSTEM") or shutil.which("qemu-system-x86_64", path=BOOT["SYSTEM_PATH"])
    if not binary:
        raise RuntimeError("Instale qemu-system-x86_64.")

    def drive(file, options):
        return f"file={str(file).replace(',', ',,')},format=raw,{options}"

    disk_options = "if=none,id=melkor,cache=none"
    if write_limit:
        disk_options += ",bps_wr=2097152"
    argv = [binary, "-machine", "q35", "-accel", accel, "-m", "512", "-display", "none", "-serial", "stdio",
            "-monitor", "none", "-nic", "none", "-no-reboot",
            "-drive", drive(code, "if=pflash,unit=0,readonly=on"),
            "-drive", drive(variables, "if=pflash,unit=1"),
            "-drive", drive(disk, disk_options),
            "-device", "virtio-blk-pci,drive=melkor,bootindex=1",
            "-drive", drive(seed, "if=none,id=seed,readonly=on"),
            "-device", "virtio-blk-pci,drive=seed"]
    (work / f"{stage}.command").write_text(shlex.join(argv) + "\n")
    with (work / f"{stage}.log").open("wb") as log, subprocess.Popen(
        argv, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.STDOUT
    ) as process:
        try:
            with BOOT["serial_console"](process, log, timeout) as console:
                yield console
        finally:
            if process.poll() is None:
                process.kill()
            log.write(process.stdout.read())


def guest(console, command, allowed=(0,)):
    # O comando ecoado nunca contém o marcador no início de uma linha.
    console.send(f"sh -c {shlex.quote(command)}; printf '\\nMELKOR-EXIT=%s\\n' \"$?\"".encode("ascii"))
    result = console.expect(rb"^MELKOR-EXIT=[0-9]+\r?$", command)
    status = int(result.splitlines()[-1].split(b"=", 1)[1])
    if status not in allowed:
        raise RuntimeError(f"O comando no convidado saiu com {status}: {command}")
    return result


def slot(console, name):
    guest(console, f"grep -qw rauc.slot={name} /proc/cmdline && test \"$(cat /proc/1/comm)\" = dinit")


def grub_state(console):
    output = guest(console, f"grub-editenv {GRUBENV} list")
    return dict(re.findall(rb"^(ORDER|[AB]_(?:OK|TRY))=([^\n]*)$", output, re.MULTILINE))


def confirm(console, name):
    guest(console, "for attempt in 1 2 3 4 5; do "
          f"if grub-editenv {GRUBENV} list | grep -qx {name}_TRY=0; then exit 0; fi; "
          "sleep 1; done; dinitctl status rauc-confirm; exit 1")


def slot_hash(console, name):
    return re.search(rb"^([a-f0-9]{64}) ", guest(console, f"sha256sum /run/melkor/core-{name}"), re.MULTILINE)[1]


def install(console, name, allowed=(0,)):
    copy_bundle(console, name)
    return guest(console, UPDATE, allowed=allowed)


def copy_bundle(console, name):
    guest(console, f"cp /run/seed/{name}.raucb {INPUT} && chmod 0600 {INPUT}")


def wait_for_partial_write(disk, candidate):
    table = json.loads(subprocess.check_output(["sfdisk", "--json", disk]))["partitiontable"]
    inactive = next(partition for partition in table["partitions"] if partition["name"] == "core-b")
    offset = inactive["start"] * table["sectorsize"]
    with candidate.open("rb") as image, disk.open("rb", buffering=0) as virtual:
        header = image.read(4096)
        tail_offset = candidate.stat().st_size - 4096
        image.seek(tail_offset)
        tail = image.read(4096)
        virtual.seek(offset)
        assert virtual.read(4096) != header, "Os slots já contêm a mesma geração."
        deadline = time.monotonic() + 30
        while time.monotonic() < deadline:
            virtual.seek(offset)
            if virtual.read(4096) == header:
                virtual.seek(offset + tail_offset)
                assert virtual.read(4096) != tail, "A escrita terminou antes da interrupção."
                return
            time.sleep(0.02)
    raise RuntimeError("A escrita parcial do slot B não foi observada.")


def scenario(args, work, seed, code, firmware_vars, password, outcome, timeout, accel):
    work.mkdir()
    disk, variables = work / "disk.img", work / "OVMF_VARS.fd"
    shutil.copyfile(args.build / "images/melkor-m1.img", disk)
    shutil.copyfile(firmware_vars, variables)
    @contextmanager
    def root(stage, write_limit=False, select_a=False):
        with machine(work, stage, disk, seed, code, variables, timeout, accel, write_limit) as console:
            if select_a:
                console.expect(rb"The highlighted entry will be executed automatically in", "menu do GRUB")
                console.send(b"\x1b[A", end=b"")
                console.expect(rb"\*Melkor Linux M1 - slot A", "seleção manual de A")
                console.send(b"")
            BOOT["login"](console, b"root", password, rb"^[^\n]*# ")
            yield console
            if not write_limit:
                guest(console, "sync")

    with root("01-install-a", write_limit=(outcome == "interrupted")) as console:
        slot(console, "A")
        confirm(console, "A")
        guest(console, f"printf 'persistiu\\n' > {DOCUMENT} && chown melkor {DOCUMENT}")
        guest(console, "mkdir -p /run/seed && mount -t ext4 -o ro /dev/vdb /run/seed")
        initial_state = grub_state(console)
        a_hash, b_hash = slot_hash(console, "a"), slot_hash(console, "b")
        if outcome == "interrupted":
            copy_bundle(console, "healthy")
            guest(console, "sync")
            console.send(UPDATE.encode("ascii"))
            wait_for_partial_write(disk, work.parent / "healthy.squashfs")
        else:
            if outcome == "healthy":
                rejected = guest(console, "su -s /bin/sh -c "
                                 "'/usr/sbin/melkor-core-update /etc/os-release' melkor", allowed=(1,))
                assert b"Entre em uma sess" in rejected, "A recusa não confirmou a falta de privilégio"
                copy_bundle(console, "healthy")
                guest(console, f"flock /run/melkor/rauc.lock {UPDATE}", allowed=(1,))
                assert grub_state(console) == initial_state
                assert slot_hash(console, "b") == b_hash
                for name, error in (("bad-signature", b"signature verification failed"),
                                    ("corrupted", b"Unexpected dm-verity status 'C'"),
                                    ("incompatible", b"Compatible mismatch")):
                    result = install(console, name, allowed=(1,))
                    assert error.lower() in result.lower(), f"{name} apresentou erro inesperado"
                    assert grub_state(console) == initial_state, f"{name} alterou o estado de boot"
                    assert slot_hash(console, "b") == b_hash, f"{name} alterou o slot inativo"
            install(console, outcome)
            assert slot_hash(console, "a") == a_hash, "slot ativo alterado"
            assert grub_state(console)[b"ORDER"] == b"B A"

    if outcome == "interrupted":
        with root("02-restart-a") as console:
            slot(console, "A")
            confirm(console, "A")
            guest(console, f"grep -Fx persistiu {DOCUMENT}")
            assert slot_hash(console, "a") == a_hash, "slot ativo alterado"
            assert slot_hash(console, "b") != b_hash, "slot inativo sem a escrita parcial esperada"
            assert grub_state(console)[b"ORDER"] == b"A B"
            guest(console, "mkdir -p /run/seed && mount -t ext4 -o ro /dev/vdb /run/seed")
            install(console, "healthy")
        with root("03-retry-b") as console:
            slot(console, "B")
            confirm(console, "B")
            guest(console, f"test \"$(cat /etc/melkor-test-generation)\" = healthy && grep -Fx persistiu {DOCUMENT}")
        return

    with root("02-first-b") as console:
        slot(console, "B")
        guest(console, f"test \"$(cat /etc/melkor-test-generation)\" = {outcome} && grep -Fx persistiu {DOCUMENT}")
        if outcome == "healthy":
            confirm(console, "B")
        else:
            guest(console, f"for attempt in 1 2 3 4 5; do test -f {GRUBENV} && exit 0; "
                  "sleep 1; done; exit 1")
        state = grub_state(console)
        assert state[b"B_TRY"] == (b"0" if outcome == "healthy" else b"1"), state

    with root("03-next-boot") as console:
        slot(console, "B" if outcome == "healthy" else "A")
        guest(console, f"grep -Fx persistiu {DOCUMENT}")
        if outcome == "unhealthy":
            assert grub_state(console)[b"B_TRY"] == b"1"
    if outcome == "healthy":
        with root("04-manual-a", select_a=True) as console:
            slot(console, "A")
            guest(console, f"grep -Fx persistiu {DOCUMENT}")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("build", type=Path)
    parser.add_argument("--cert", type=Path, required=True)
    parser.add_argument("--key", type=Path, required=True)
    parser.add_argument("--root-password-file", type=Path, required=True)
    parser.add_argument("--log-dir", type=Path, required=True)
    args = parser.parse_args()
    args.log_dir.mkdir(parents=True, exist_ok=True)
    work = Path(tempfile.mkdtemp(prefix="rauc-", dir=args.log_dir.resolve()))
    print(f"Evidências em {work}", flush=True)
    report = {"concluido": [], "codigo_saida": 1}
    try:
        password = args.root_password_file.read_bytes().strip()
        if not re.fullmatch(rb"[0-9a-f]{32}", password):
            raise ValueError("Use a senha descartável gerada por openssl rand -hex 16.")
        firmware = SimpleNamespace(
            ovmf_code=Path(code) if (code := os.environ.get("OVMF_CODE")) else None,
            ovmf_vars=Path(variables) if (variables := os.environ.get("OVMF_VARS")) else None)
        code, variables = BOOT["find_firmware"](firmware)
        accel = os.environ.get("QEMU_ACCEL", "auto")
        if accel == "auto":
            accel = "kvm" if os.access("/dev/kvm", os.R_OK | os.W_OK) else "tcg"
        if accel not in ("kvm", "tcg"):
            raise ValueError("QEMU_ACCEL deve ser auto, kvm ou tcg.")
        timeout = 120 if accel == "kvm" else 300
        digest = BOOT["file_digest"]
        report.update(
            commit=subprocess.check_output(["git", "rev-parse", "HEAD"], text=True).strip(),
            imagem_sha256=digest(args.build / "images/melkor-m1.img"),
            firmware_sha256=digest(code), variaveis_uefi_sha256=digest(variables),
            acelerador=accel, prazo_segundos=timeout,
        )
        seed = seed_disk(args, work)
        for outcome in ("healthy", "unhealthy", "interrupted"):
            scenario(args, work / outcome, seed, code, variables, password, outcome, timeout, accel)
            report["concluido"].append(outcome)
        report["codigo_saida"] = 0
        print("Assinatura, rejeição, promoção A → B, fallback B → A e interrupção de escrita verificados.")
    except (AssertionError, KeyError, OSError, RuntimeError, ValueError, subprocess.CalledProcessError) as error:
        report["erro"] = str(error)
        print(error)
        return 1
    finally:
        (work / "result.json").write_text(json.dumps(report, indent=2) + "\n")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
