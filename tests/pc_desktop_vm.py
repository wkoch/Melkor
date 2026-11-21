#!/usr/bin/env python3
"""Exercita login local, DRM, seatd e a abertura de um terminal via painel Wayland."""

import argparse
from contextlib import suppress
import json
import os
from pathlib import Path
import shutil
import socket
import subprocess
import tempfile
import time
from types import SimpleNamespace

from m1_dns_vm import BOOT, guest


def desktop(console, qmp, run, apps, chromium, umbriel):
    BOOT["login"](console, b"melkor", b"melkor", rb"^[^\n]*\$ ")
    session = "--umbriel " if umbriel else ""
    for char in f"melkor\nmelkor\nmelkor-desktop {session}>/tmp/melkor-gui.log 2>&1\n":
        keys = {
            " ": ["spc"], "\n": ["ret"], "-": ["minus"], "/": ["slash"], ".": ["dot"],
            ">": ["shift", "dot"], "&": ["shift", "7"],
        }.get(char, [char])
        qmp("send-key", keys=[{"type": "qcode", "data": key} for key in keys], **{"hold-time": 45})
        time.sleep(0.08)
    try:
        guest(console, "for attempt in $(seq 1 20); do "
              'for device in /tmp/melkor-wayland.*/wayland-*; do [ -S "$device" ] && exit 0; done; '
              "sleep 1; done; cat /tmp/melkor-gui.log; exit 1")
        if umbriel:
            guest(console, 'for attempt in $(seq 1 16); do '
                  'for socket in /tmp/melkor-wayland.*/noctalia-wayland-*.sock; do '
                  'test -S "$socket" && exit 0; done; '
                  'sleep 1; done; cat /tmp/melkor-gui.log; exit 1')
        else:
            guest(console, 'pid=$(pidof -s weston) && awk \'$1=="Uid:" && $2==1000 {ok=1} END{exit !ok}\' '
                  '"/proc/$pid/status" && grep -iq pixman /tmp/melkor-gui.log')
        time.sleep(1)
        panel = run / "01-desktop.ppm"
        qmp("screendump", filename=str(panel))
        with panel.open("rb") as image:
            assert image.readline().strip() == b"P6", "Formato inesperado no screendump do QEMU."
            width, height = map(int, image.readline().split())
        if umbriel:
            qmp("send-key", keys=[{"type": "qcode", "data": key} for key in ("meta_l", "ret")])
        else:
            qmp("input-send-event", events=[
                {"type": "abs", "data": {"axis": axis, "value": 32767 * 16 // (extent - 1)}}
                for axis, extent in (("x", width), ("y", height))
            ])
            for down in (True, False):
                qmp("input-send-event", events=[{"type": "btn", "data": {"down": down, "button": "left"}}])
        terminal = "nu" if apps else "foot"
        guest(console, f"for attempt in $(seq 1 12); do pidof {terminal} >/dev/null && exit 0; "
              "sleep 1; done; cat /tmp/melkor-gui.log; exit 1")
        qmp("screendump", filename=str(run / "02-terminal.ppm"))
        if apps:
            test_firefox(console, qmp, run)
        if chromium:
            test_chromium(console, qmp, run)
    finally:
        with suppress(RuntimeError):
            qmp("screendump", filename=str(run / "tela-final.ppm"))


def test_firefox(console, qmp, run):
    guest(console, """/usr/bin/nu --no-config-file -c '[1 2 3] | math sum' | grep -Fx 6""")
    launch = (
        'for socket in /tmp/melkor-wayland.*/wayland-*; do test -S "$socket" && break; done; '
        'export XDG_RUNTIME_DIR="${socket%/*}" WAYLAND_DISPLAY="${socket##*/}"; '
    )
    page = "'data:text/html,<title>Melkor Firefox</title><h1>Melkor Firefox ESR</h1>'"
    guest(console, launch + "melkor-firefox --headless --screenshot /home/browser/site.png " + page +
          ' > /tmp/melkor-firefox.log 2>&1 && test -s "$HOME/.local/share/melkor/firefox/site.png" '
          '|| { cat /tmp/melkor-firefox.log; exit 1; }')
    guest(console, launch + "melkor-firefox " + page + " >/tmp/melkor-firefox.log 2>&1 &")
    guest(console, "for attempt in $(seq 1 15); do pidof firefox-esr >/dev/null && exit 0; "
          "sleep 1; done; cat /tmp/melkor-firefox.log; exit 1")
    time.sleep(2)
    qmp("screendump", filename=str(run / "03-firefox.ppm"))


def test_chromium(console, qmp, run):
    for name, process in (("brave-origin", "brave"), ("blanc", "blanc")):
        launch = ('for socket in /tmp/melkor-wayland.*/wayland-*; do test -S "$socket" && break; done; '
                  'export XDG_RUNTIME_DIR="${socket%/*}" WAYLAND_DISPLAY="${socket##*/}"; ')
        page = f"'data:text/html,<title>Melkor {name}</title><h1>Melkor {name}</h1>'"
        guest(console, launch + f"melkor-{name} " + page + f" >/tmp/melkor-{name}.log 2>&1 &")
        guest(console, f"for attempt in $(seq 1 18); do "
              f'for pid in $(pidof {process}); do '
              'if grep -aq -- "--type=renderer" "/proc/$pid/cmdline" && '
              'awk \'$1=="Seccomp:" && $2==2 {ok=1} END{exit !ok}\' "/proc/$pid/status"; '
              'then exit 0; fi; done; '
              f"sleep 1; done; cat /tmp/melkor-{name}.log; exit 1")
        qmp("screendump", filename=str(run / f"04-{name}.ppm"))


def run_test(image, run, apps, chromium, umbriel):
    code, variables = BOOT["find_firmware"](SimpleNamespace(
        ovmf_code=Path(code) if (code := os.environ.get("OVMF_CODE")) else None,
        ovmf_vars=Path(variables) if (variables := os.environ.get("OVMF_VARS")) else None))
    disk = run / "disk.img"
    variables_copy = run / "OVMF_VARS.fd"
    subprocess.run(["cp", "--reflink=auto", "--sparse=always", "--", image, disk], check=True)
    shutil.copyfile(variables, variables_copy)
    qemu = os.environ.get("QEMU_SYSTEM") or shutil.which("qemu-system-x86_64", path=BOOT["SYSTEM_PATH"])
    if not qemu:
        raise RuntimeError("QEMU não está no PATH.")
    accel = "kvm" if os.access("/dev/kvm", os.R_OK | os.W_OK) else "tcg"
    with tempfile.TemporaryDirectory(prefix="melkor-qmp-") as runtime:
        monitor = Path(runtime) / "qmp.sock"
        def drive(path, options):
            return f"file={str(path).replace(',', ',,')},format=raw,{options}"
        memory = "4096" if chromium else ("2048" if apps else "1024")
        command = [qemu, "-machine", "q35", "-accel", accel, "-m", memory, "-display", "none",
                   "-serial", "stdio", "-monitor", "none", "-nic", "none", "-no-reboot",
                   "-qmp", f"unix:{monitor},server=on,wait=off",
                   "-drive", drive(code, "if=pflash,unit=0,readonly=on"),
                   "-drive", drive(variables_copy, "if=pflash,unit=1"),
                   "-drive", drive(disk, "if=none,id=melkor"),
                   "-device", "nvme,drive=melkor,serial=melkor-desktop,bootindex=1",
                   "-vga", "std", "-device", "virtio-tablet-pci"]
        with (run / "serial.log").open("wb") as log, subprocess.Popen(
            command, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.STDOUT
        ) as vm:
            try:
                for _ in range(100):
                    if monitor.exists() or vm.poll() is not None:
                        break
                    time.sleep(0.05)
                with socket.socket(socket.AF_UNIX, socket.SOCK_STREAM) as control:
                    control.settimeout(10)
                    control.connect(str(monitor))
                    with control.makefile("rwb", buffering=0) as pipe:
                        json.loads(pipe.readline())

                        def qmp(operation, **arguments):
                            pipe.write(json.dumps({"execute": operation, "arguments": arguments}).encode() + b"\n")
                            while True:
                                response = json.loads(pipe.readline())
                                if "error" in response:
                                    raise RuntimeError(response["error"])
                                if "return" in response:
                                    return response["return"]

                        qmp("qmp_capabilities")
                        time.sleep(5)
                        qmp("screendump", filename=str(run / "00-boot.ppm"))
                        with BOOT["serial_console"](vm, log, 120) as console:
                            desktop(console, qmp, run, apps, chromium, umbriel)
            finally:
                vm.kill()
                log.write(vm.stdout.read())


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("image", type=Path)
    parser.add_argument("--log-dir", required=True, type=Path)
    parser.add_argument("--apps", action="store_true")
    parser.add_argument("--chromium", action="store_true")
    parser.add_argument("--umbriel", action="store_true")
    args = parser.parse_args()
    args.log_dir.mkdir(parents=True, exist_ok=True)
    run = Path(tempfile.mkdtemp(prefix="desktop-", dir=args.log_dir)).resolve()
    print(f"Evidências em {run}", flush=True)
    run_test(args.image, run, args.apps, args.chromium, args.umbriel)
    tested = "Alacritty com Nushell e Firefox ESR" if args.apps else "foot"
    if args.chromium:
        tested += ", Brave Origin e Blanc"
    session = "Umbriel com Noctalia" if args.umbriel else "Weston"
    print(f"Login local, {session} como usuário comum e abertura de {tested} verificados.")


if __name__ == "__main__":
    main()
