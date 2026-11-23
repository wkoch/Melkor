#!/usr/bin/env python3
"""Instala por UEFI em um NVMe descartável e verifica o disco vizinho e os dois slots."""

import argparse
from contextlib import contextmanager
import hashlib
import json
import os
from pathlib import Path
import re
import secrets
import shutil
import socket
import subprocess
import tempfile
import time
from types import SimpleNamespace

from m1_dns_vm import BOOT, guest
from pc_desktop_vm import run_test as check_desktop


def boot_ventoy(monitor, mode):
    until = time.monotonic() + 15
    while not monitor.exists() and time.monotonic() < until:
        time.sleep(0.1)
    with socket.socket(socket.AF_UNIX, socket.SOCK_STREAM) as control:
        control.connect(str(monitor))
        # O menu do Ventoy aparece no framebuffer, antes do console serial do Melkor.
        time.sleep(13)
        control.sendall(b"sendkey ret\n")
        time.sleep(2)
        if mode == "grub2":
            control.sendall(b"sendkey down\n")
            time.sleep(0.3)
        control.sendall(b"sendkey ret\n")


@contextmanager
def machine(qemu, accel, code, variables, disk, neighbor, log_path, sector,
            iso=None, ventoy=None, mode="normal", memory=None, profile_media=None):
    def drive(path, options):
        return f"file={str(path).replace(',', ',,')},format=raw,{options}"
    command = [qemu, "-machine", "q35", "-accel", accel, "-m", str(memory or (3072 if sector == 4096 else 2048)),
               "-display", "none", "-serial", "stdio",
               "-nic", "none", "-no-reboot", "-vga", "std",
               "-drive", drive(code, "if=pflash,unit=0,readonly=on"),
               "-drive", drive(variables, "if=pflash,unit=1"),
               "-drive", drive(disk, "if=none,id=target"),
               "-device", f"nvme,drive=target,serial=melkor-target,bootindex=2,logical_block_size={sector},physical_block_size={sector}",
               "-drive", drive(neighbor, "if=none,id=neighbor"),
               "-device", "nvme,drive=neighbor,serial=melkor-preserve"]
    if ventoy is not None:
        command += ["-drive", drive(ventoy, "if=none,id=usb,readonly=on"), "-device", "qemu-xhci,id=usbhost",
                    "-device", "usb-storage,bus=usbhost.0,drive=usb,bootindex=1"]
    elif iso is not None:
        command += ["-drive", drive(iso, "if=none,id=media,media=cdrom"),
                    "-device", "ide-cd,bus=ide.0,drive=media,bootindex=1"]
    if profile_media is not None:
        command += ["-drive", drive(profile_media, "if=none,id=profile,readonly=on"),
                    "-device", "qemu-xhci,id=profile-usb",
                    "-device", "usb-storage,bus=profile-usb.0,drive=profile"]
    with tempfile.TemporaryDirectory(prefix="melkor-qemu-") as tmp:
        monitor = Path(tmp) / "monitor"
        command += ["-monitor", f"unix:{monitor},server=on,wait=off" if ventoy else "none"]
        with log_path.open("wb") as log, subprocess.Popen(
                command, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.STDOUT) as vm:
            try:
                if ventoy:
                    boot_ventoy(monitor, mode)
                yield vm, log
            finally:
                if vm.poll() is None:
                    vm.kill()
                log.write(vm.stdout.read())


def check_ventoy_iso(iso, drive):
    table = json.loads(subprocess.check_output(["sfdisk", "--json", drive]))["partitiontable"]
    offset = table["partitions"][0]["start"] * table["sectorsize"]
    with subprocess.Popen(["mtype", "-i", f"{drive}@@{offset}", "::/melkor-installer.iso"], stdout=subprocess.PIPE) as media:
        digest = hashlib.file_digest(media.stdout, "sha256").hexdigest()
        if media.wait() != 0 or digest != BOOT["file_digest"](iso):
            raise RuntimeError("O arquivo na partição de dados do Ventoy difere da ISO em teste.")


def install(vm, log, deadline, disk, passwords, sector):
    fingerprints = BOOT["file_digest"]
    with BOOT["serial_console"](vm, log, deadline) as console:
        BOOT["limine_entry"](console, b"\x1b[B")
        before = fingerprints(disk)
        for confirmation in (b"cancelar", b"APAGAR"):
            console.expect(b"Digite instalar core para iniciar: ", "comando inicial do instalador")
            console.send(b"instalar core")
            initial = console.expect("Número do disco: ".encode(), "lista de discos internos")
            intended = re.search(rb'^([1-9][0-9]*)[.] "QEMU NVMe Ctrl", 16[.]0 GiB, setor de '
                                 + str(sector).encode() + rb" bytes", initial, re.M)
            if intended is None or b", 13.0 GiB, setor de 512 bytes" not in initial:
                raise RuntimeError("A lista precisa apresentar os dois NVMe descartáveis.")
            console.send(intended[1])
            selected = console.expect(rb"Digite APAGAR para confirmar: ", "confirmação de apagamento")
            if b"16.0 GiB" not in selected or b"melkor-target" not in selected:
                raise RuntimeError("A confirmação precisa identificar o NVMe selecionado.")
            console.send(confirmation)
            if confirmation == b"cancelar":
                console.expect("instalação cancelada".encode(), "cancelamento da instalação")
                console.expect(rb"pressione Enter para voltar: ", "retorno ao instalador")
                if before != fingerprints(disk):
                    raise RuntimeError("A confirmação inválida alterou o destino.")
                console.send(b"")
        for account, password in ((b"root", passwords[1]), (b"melkor", passwords[0])):
            console.expect(rb"Senha de " + account + rb" \(12 a 128 bytes\): ", "senha local")
            console.send(password)
            console.expect(rb"Repita a senha de " + account + rb": ", "confirmação da senha")
            console.send(password)
        result = console.expect("Instalação concluída.|Instalação encerrada:[^\r\n]*".encode(), "gravação e sincronização do disco")
        if not result.endswith("Instalação concluída.".encode()):
            raise RuntimeError(result.decode(errors="replace").splitlines()[-1])


def stage_desktop(vm, log, passwords):
    with BOOT["serial_console"](vm, log, 200) as console:
        BOOT["login"](console, b"melkor", passwords[0], rb"^[^\n]*\$ ")
        guest(console, "test ! -e /etc/dinit.d/graphical-login")
        console.send(b"su -")
        console.expect(rb"^\r?Password: ?", "senha administrativa")
        console.send(passwords[1])
        console.expect(rb"^[^\n]*# ", "shell administrativo")
        guest(console, "for attempt in $(seq 1 15); do blkid -t LABEL=MELKOR_APPS -o device && exit 0; sleep 1; done; "
              "blkid; dmesg | tail -30; exit 1")
        guest(console, "melkor-finalizar MELKOR_APPS && sync")


def check_operational_commands(vm, log, passwords):
    offline = rb"(?i)dial (?:tcp|udp)|lookup |network is unreachable|no route to host|" + \
        "ainda não contém a chave".encode()
    with BOOT["serial_console"](vm, log, 200) as console:
        BOOT["login"](console, b"melkor", passwords[0], rb"^[^\n]*\$ ")
        guest(console, "instalar core", expected_exit=2)
        guest(console, "instalar pacote inexistente", expected_exit=2)
        guest(console, "atualizar argumento", expected_exit=2)
        console.send(b"instalar pacote noctalia")
        console.expect(rb"^\r?Password: ?", "elevação para instalar Noctalia")
        console.send(passwords[1])
        console.expect(offline, "tentativa de consultar o canal do desktop sem rede")
        console.expect(rb"^[^\n]*\$ ", "retorno à conta comum")
        console.send(b"atualizar")
        console.expect(rb"^\r?Password: ?", "elevação para atualizar o sistema")
        console.send(passwords[1])
        console.expect(offline, "tentativa de consultar o canal do sistema sem rede")
        console.expect(rb"^[^\n]*\$ ", "retorno à conta comum após atualizar")
        guest(console, 'test -O "$HOME/.local/share/melkor/geracoes" && test "$(id -u)" -ne 0')


def check_installation(iso, run, sector, ventoy=None, mode="normal", memory=None, graphical=False, desktop_bundle=None):
    if ventoy:
        check_ventoy_iso(iso, ventoy)
    qemu = os.environ.get("QEMU_SYSTEM") or shutil.which("qemu-system-x86_64", path=BOOT["SYSTEM_PATH"])
    if qemu is None:
        raise RuntimeError("QEMU ausente.")
    firmware = SimpleNamespace(
        ovmf_code=Path(code) if (code := os.environ.get("OVMF_CODE")) else None,
        ovmf_vars=Path(variables) if (variables := os.environ.get("OVMF_VARS")) else None)
    code, vars_source = BOOT["find_firmware"](firmware)
    variables = run / "OVMF_VARS.fd"
    shutil.copyfile(vars_source, variables)
    disk = run / "target.img"
    neighbor = run / "preserve.img"
    with disk.open("xb") as stream:
        stream.truncate(16 << 30)
    with neighbor.open("xb") as stream:
        stream.truncate(13 << 30)
        stream.seek(1 << 20)
        stream.write(b"melkor-preservar\n")
    before = BOOT["file_digest"](neighbor)
    accel = os.environ.get("QEMU_ACCEL", "tcg")
    passwords = (secrets.token_hex(64 if graphical else 16).encode("ascii"),
                 secrets.token_hex(16).encode("ascii"))
    with machine(qemu, accel, code, variables, disk, neighbor, run / "serial-install.log",
                 sector, iso, ventoy, mode, memory) as session:
        install(*session, 300, disk, passwords, sector)
    if any(password in (run / "serial-install.log").read_bytes() for password in passwords):
        raise RuntimeError("Uma senha foi registrada na saída do instalador.")
    sfdisk = ["sfdisk"]
    if sector != 512:
        sfdisk += ["--sector-size", str(sector)]
    table = json.loads(subprocess.check_output([*sfdisk, "--json", disk]))["partitiontable"]
    parts = table["partitions"]
    if table["label"] != "gpt" or [p["name"] for p in parts] != ["esp", "core-a", "core-b", "persist"]:
        raise RuntimeError("Particionamento GPT inesperado.")
    if (16 << 30) - (parts[-1]["start"] + parts[-1]["size"]) * table["sectorsize"] != 1 << 20:
        raise RuntimeError("A partição persistente precisa usar o espaço até a reserva final.")
    subprocess.run([*sfdisk, "--verify", disk], check=True, stdout=subprocess.DEVNULL)
    for slot in ("A", "B"):
        with machine(qemu, accel, code, variables, disk, neighbor, run / f"serial-{slot.lower()}.log", sector) as session:
            BOOT["check_console"](*session, 240, slot, pc=True, credentials=passwords)
    with machine(qemu, accel, code, variables, disk, neighbor, run / "serial-comandos.log", sector) as session:
        check_operational_commands(*session, passwords)
    if desktop_bundle is not None:
        profile_media = run / "profile-media.img"
        capacity_kib = (desktop_bundle.stat().st_size + 1023) // 1024 + 65536
        subprocess.run(["mkfs.vfat", "-C", "-n", "MELKOR_APPS", profile_media, str(capacity_kib)], check=True)
        subprocess.run(["mcopy", "-i", profile_media, desktop_bundle, "::/melkor-desktop.raucb"], check=True)
        with machine(qemu, accel, code, variables, disk, neighbor, run / "serial-profile.log",
                     sector, profile_media=profile_media) as session:
            stage_desktop(*session, passwords)
    if graphical:
        desktop_logs = run / "desktop"
        desktop_logs.mkdir()
        check_desktop(disk, desktop_logs, apps=True, chromium=True, umbriel=True, audio=True,
                      graphical=True, passwords=passwords)
    after = BOOT["file_digest"](neighbor)
    if before != after:
        raise RuntimeError("O NVMe vizinho foi alterado.")
    (run / "result.json").write_text(json.dumps({
        "slots": ["A", "B"], "iso_sha256": BOOT["file_digest"](iso), "setor_logico": sector,
        "midia": f"ventoy-{mode}" if ventoy else "cd-uefi",
        "login_grafico": graphical,
        "bundle_desktop_sha256": BOOT["file_digest"](desktop_bundle) if desktop_bundle else None,
        "vizinho_sha256_antes": before, "vizinho_sha256_depois": after,
    }, indent=2) + "\n")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("iso", type=Path)
    parser.add_argument("--log-dir", required=True, type=Path)
    parser.add_argument("--memory-mib", type=int)
    parser.add_argument("--sector-size", choices=(512, 4096), type=int, default=512)
    parser.add_argument("--ventoy-drive", type=Path, help="Arquivo regular do pendrive virtual contendo apenas esta ISO.")
    parser.add_argument("--ventoy-mode", choices=("normal", "grub2"), default="normal")
    parser.add_argument("--graphical-login", action="store_true", help="Exercita o desktop com a senha criada no instalador.")
    parser.add_argument("--desktop-bundle", type=Path, help="Aplica um bundle desktop assinado após instalar a base.")
    args = parser.parse_args()
    if args.ventoy_drive is not None and not args.ventoy_drive.is_file():
        parser.error("--ventoy-drive precisa ser um arquivo regular")
    args.log_dir.mkdir(parents=True, exist_ok=True)
    run = Path(tempfile.mkdtemp(prefix="install-", dir=args.log_dir)).resolve()
    print(f"Evidências em {run}", flush=True)
    graphical = args.graphical_login or args.desktop_bundle is not None
    if graphical and args.sector_size != 512:
        parser.error("--graphical-login requer um disco virtual com setores de 512 bytes")
    if args.desktop_bundle is not None and not args.desktop_bundle.is_file():
        parser.error("--desktop-bundle precisa ser um arquivo regular")
    check_installation(args.iso.resolve(strict=True), run, args.sector_size, args.ventoy_drive,
                       args.ventoy_mode, args.memory_mib, graphical, args.desktop_bundle)
    print("Instalação, cancelamento, vizinho intacto e primeiro boot nos dois slots verificados.")


if __name__ == "__main__":
    main()
