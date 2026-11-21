#!/usr/bin/env python3
"""Exercita login local, DRM, seatd e a abertura de um terminal via painel Wayland."""

import argparse
from contextlib import suppress
import json
import os
from pathlib import Path
import shlex
import shutil
import socket
import struct
import subprocess
import tempfile
import time
from types import SimpleNamespace
import wave

from m1_dns_vm import BOOT, guest

WAYLAND = (
    'for socket in /tmp/melkor-wayland.*/wayland-*; do test -S "$socket" && break; done; '
    'export XDG_RUNTIME_DIR="${socket%/*}" WAYLAND_DISPLAY="${socket##*/}"; '
)
TONE = (
    '<script>const c=new AudioContext();const o=c.createOscillator();'
    'const g=c.createGain();g.gain.value=0.1;o.connect(g).connect(c.destination);'
    'o.start();c.resume();</script>'
)


def desktop(console, qmp, run, apps, chromium, umbriel, audio):
    BOOT["login"](console, b"melkor", b"melkor", rb"^[^\n]*\$ ")
    session = "" if umbriel else "--weston "
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
        if audio:
            test_audio_policy(console)
        if apps:
            test_firefox(console, qmp, run, audio)
        if chromium:
            test_chromium(console, qmp, run, audio)
    finally:
        with suppress(RuntimeError):
            qmp("screendump", filename=str(run / "tela-final.ppm"))


def test_audio_policy(console):
    guest(console, WAYLAND + 'for attempt in $(seq 1 15); do '
          'melkor-session-exec pactl get-default-sink 2>/dev/null | grep -q "^alsa_output" && break; sleep 1; done; '
          'melkor-session-exec pactl list short sinks | grep alsa_output || '
          '{ cat "$XDG_RUNTIME_DIR"/pipewire*.log "$XDG_RUNTIME_DIR"/wireplumber.log; exit 1; }')
    guest(console, WAYLAND + '''
        melkor-session-exec parec --record > /tmp/pulse-capture.pcm 2>/tmp/pulse-capture.log &
        recorder=$!; sleep 1; kill "$recorder" 2>/dev/null || :;
        ! wait "$recorder" &&
        test ! -s /tmp/pulse-capture.pcm &&
        grep -qi "Access denied" /tmp/pulse-capture.log &&
        ! melkor-session-exec pactl load-module module-null-sink > /tmp/pulse-module.log 2>&1 &&
        grep -qi "Access denied" /tmp/pulse-module.log ||
        { cat /tmp/pulse-module.log /tmp/pulse-capture.log; exit 1; }
    '''.strip().replace("\n", " "))


def require_audio_stream(console, process):
    guest(console, WAYLAND + 'for attempt in $(seq 1 15); do '
          'melkor-session-exec pactl list sink-inputs >/tmp/pulse-streams.log && '
          f'grep -E "application.process.binary = \\"({process})\\"" /tmp/pulse-streams.log && exit 0; '
          'sleep 1; done; cat /tmp/pulse-streams.log /tmp/melkor-*.log; exit 1')


def browser_page(console, browser, title, audio):
    content = f"<!doctype html><title>{title}</title><h1>{title}</h1>"
    if audio:
        content += TONE
    if browser == "blanc":
        # Blanc aceita apenas HTTP(S) no encaminhamento de URLs pela linha de comando.
        headers = ("HTTP/1.1 200 OK\\r\\nContent-Type: text/html\\r\\n"
                   f"Content-Length: {len(content)}\\r\\nConnection: close\\r\\n\\r\\n")
        responder = ('#!/bin/sh', 'cr=$(printf "\\r")',
                     'while IFS= read -r line && [ "$line" != "$cr" ]; do :; done',
                     "cat /tmp/melkor-http-response")
        guest(console, f'printf "%b%s" {shlex.quote(headers)} {shlex.quote(content)} >/tmp/melkor-http-response && '
              f'printf "%s\\n" {" ".join(map(shlex.quote, responder))} >/tmp/melkor-http && '
              'chmod 700 /tmp/melkor-http')
        guest(console, 'rt=/opt/melkor-desktop; "$rt/lib/ld-musl-x86_64.so.1" '
              '--library-path "$rt/lib:$rt/usr/lib" "$rt/bin/busybox" nc '
              '-lk -s 127.0.0.1 -p 8765 -e /tmp/melkor-http >/tmp/melkor-http.log 2>&1 &')
        url = "http://127.0.0.1:8765/"
        guest(console, f'for attempt in 1 2 3 4 5; do wget -qO- {url} | grep -q {shlex.quote(title)} '
              '&& exit 0; sleep 1; done; cat /tmp/melkor-http.log; exit 1')
        return url
    guest(console, f'dir="$HOME/.local/share/melkor/{browser}"; mkdir -p "$dir" && '
          f'printf "%s\\n" {shlex.quote(content)} >"$dir/melkor-test.html"')
    return "file:///home/browser/melkor-test.html"


def test_firefox(console, qmp, run, audio):
    guest(console, """/usr/bin/nu --no-config-file -c '[1 2 3] | math sum' | grep -Fx 6""")
    guest(console, 'for scheme in http https; do '
          'LC_ALL=C gio mime "x-scheme-handler/$scheme" | head -n 1 | grep -E '
          '"^Default application for .*: melkor-firefox[.]desktop$" || exit 1; done')
    page = browser_page(console, "firefox", "Melkor Firefox ESR", audio)
    guest(console, WAYLAND + "melkor-firefox --headless --screenshot /home/browser/site.png " + page +
          ' > /tmp/melkor-firefox.log 2>&1 && test -s "$HOME/.local/share/melkor/firefox/site.png" '
          '|| { cat /tmp/melkor-firefox.log; exit 1; }')
    if audio:
        guest(console, 'profile="$HOME/.local/share/melkor/firefox/test-profile"; mkdir -p "$profile" && '
              'printf \'user_pref("media.autoplay.default", 0);\\n\' >"$profile/user.js"')
        page = "--profile /home/browser/test-profile " + page
    guest(console, WAYLAND + "melkor-firefox " + page + " >/tmp/melkor-firefox.log 2>&1 &")
    guest(console, "for attempt in $(seq 1 15); do pidof firefox-esr >/dev/null && exit 0; "
          "sleep 1; done; cat /tmp/melkor-firefox.log; exit 1")
    time.sleep(2)
    qmp("screendump", filename=str(run / "03-firefox.ppm"))
    if audio:
        require_audio_stream(console, "firefox(-esr)?")


def test_chromium(console, qmp, run, audio):
    for name, process in (("brave-origin", "brave"), ("blanc", "blanc")):
        page = browser_page(console, name, f"Melkor {name}", audio)
        if audio:
            page = "--no-first-run --no-default-browser-check --autoplay-policy=no-user-gesture-required " + page
            if name == "brave-origin":
                guest(console, 'dir="$HOME/.local/share/melkor/brave-origin/test-profile"; mkdir -p "$dir" && '
                      'printf \'%s\\n\' \'{"brave":{"origin":{"free_tier_accepted":true}}}\' >"$dir/Local State"')
                page = "--user-data-dir=/home/browser/test-profile " + page
            else:
                guest(console, 'dir="$HOME/.local/share/melkor/blanc/.config/Blanc"; mkdir -p "$dir" && '
                      'printf \'%s\\n\' \'{"onboardingVersion":1,"firstRunCompletedAt":1,'
                      '"searchSuggestions":false,"usagePing":false}\' >"$dir/settings.json"')
        guest(console, WAYLAND + f"melkor-{name} " + page + f" >/tmp/melkor-{name}.log 2>&1 &")
        guest(console, f"for attempt in $(seq 1 18); do "
              f'for pid in $(pidof {process}); do '
              'if grep -aq -- "--type=renderer" "/proc/$pid/cmdline" && '
              'awk \'$1=="Seccomp:" && $2==2 {ok=1} END{exit !ok}\' "/proc/$pid/status"; '
              'then exit 0; fi; done; '
              f"sleep 1; done; cat /tmp/melkor-{name}.log; exit 1")
        qmp("screendump", filename=str(run / f"04-{name}.ppm"))
        if audio:
            require_audio_stream(console, process)


def run_test(image, run, apps, chromium, umbriel, audio):
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
        if audio:
            command += ["-audiodev", f"wav,id=audio,path={run / 'audio.wav'}",
                        "-device", "ich9-intel-hda", "-device", "hda-duplex,audiodev=audio"]
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
                            desktop(console, qmp, run, apps, chromium, umbriel, audio)
                        qmp("quit")
                        vm.wait(timeout=10)
            finally:
                if vm.poll() is None:
                    vm.kill()
                log.write(vm.stdout.read())
    if audio:
        check_audio_recording(run / "audio.wav")


def check_audio_recording(path):
    with path.open("r+b") as stream:
        header = stream.read(44)
        # O driver WAV do QEMU pode deixar os campos de tamanho provisórios.
        if header.startswith(b"RIFF\0\0\0\0WAVEfmt \x10\0\0\0") and header[36:] == b"data\0\0\0\0":
            size = stream.seek(0, os.SEEK_END)
            stream.seek(4)
            stream.write(struct.pack("<I", size - 8))
            stream.seek(40)
            stream.write(struct.pack("<I", size - 44))
    with wave.open(str(path), "rb") as recording:
        if not any(recording.readframes(recording.getnframes())):
            raise RuntimeError("A placa HDA virtual não entregou amostras de áudio.")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("image", type=Path)
    parser.add_argument("--log-dir", required=True, type=Path)
    parser.add_argument("--apps", action="store_true")
    parser.add_argument("--chromium", action="store_true")
    parser.add_argument("--umbriel", action="store_true")
    parser.add_argument("--audio", action="store_true")
    args = parser.parse_args()
    if args.audio and not args.apps:
        parser.error("--audio requer --apps.")
    args.log_dir.mkdir(parents=True, exist_ok=True)
    run = Path(tempfile.mkdtemp(prefix="desktop-", dir=args.log_dir)).resolve()
    print(f"Evidências em {run}", flush=True)
    run_test(args.image, run, args.apps, args.chromium, args.umbriel, args.audio)
    tested = "Alacritty com Nushell e Firefox ESR" if args.apps else "foot"
    if args.chromium:
        tested += ", Brave Origin e Blanc"
    session = "Umbriel com Noctalia" if args.umbriel else "Weston"
    print(f"Login local, {session} como usuário comum e abertura de {tested} verificados.")
    if args.audio:
        print("Reprodução por HDA virtual e rejeição de captura e de módulos Pulse verificadas.")


if __name__ == "__main__":
    main()
