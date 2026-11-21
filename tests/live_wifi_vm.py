#!/usr/bin/env python3
"""Valida a associação WPA2 do usuário comum com rádios virtuais isolados."""

import argparse
import gzip
import os
from pathlib import Path
import shutil
import subprocess
import tempfile

from m1_dns_vm import BOOT, guest


def test_association(vm, log):
    with BOOT["serial_console"](vm, log, 90) as console:
        BOOT["login"](console, b"melkor", b"melkor", rb"^[^\n]*\$ ")
        guest(console, "for attempt in 1 2 3 4 5; do "
              "[ -S /run/dbus/system_bus_socket ] && exit 0; sleep 1; done; exit 1")
        guest(console, "id -nG | grep -qw netdev && iwctl device list")
        guest(console, "iwctl device wlan0 set-property Mode ap")
        guest(console, "iwctl ap wlan0 start Melkor-Lab teste-isolado")
        guest(console, "iwctl station wlan1 scan")
        guest(console, "for attempt in 1 2 3 4 5; do "
              "iwctl station wlan1 get-networks | grep -q Melkor-Lab && exit 0; sleep 1; done; exit 1")
        guest(console, "iwctl --passphrase=teste-isolado station wlan1 connect Melkor-Lab")
        guest(console, "iw dev wlan1 link | grep -q 'SSID: Melkor-Lab'")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("images", type=Path)
    parser.add_argument("hwsim", type=Path, help="módulo mac80211_hwsim.ko do mesmo build Linux")
    parser.add_argument("--log-dir", required=True, type=Path)
    args = parser.parse_args()
    qemu = os.environ.get("QEMU_SYSTEM") or shutil.which("qemu-system-x86_64", path=BOOT["SYSTEM_PATH"])
    for path in (args.hwsim, args.images / "rootfs.cpio.gz", args.images / "bzImage"):
        if not path.is_file():
            parser.error(f"arquivo ausente: {path}")
    if not qemu:
        parser.error("QEMU não está no PATH")
    args.log_dir.mkdir(parents=True, exist_ok=True)
    run = Path(tempfile.mkdtemp(prefix="wifi-", dir=args.log_dir))
    print(f"Evidências em {run.resolve()}", flush=True)
    with tempfile.TemporaryDirectory(prefix="hwsim-", dir=run) as folder:
        work = Path(folder)
        shutil.copyfile(args.hwsim, work / "qa-mac80211_hwsim.ko")
        init = work / "qa-init"
        init.write_text("#!/bin/sh\nset -eu\nmount -t devtmpfs devtmpfs /dev\n"
                        "insmod /qa-mac80211_hwsim.ko radios=2\nexec /sbin/dinit\n")
        init.chmod(0o755)
        overlay = subprocess.run(["cpio", "-o", "--quiet", "-H", "newc", "-R", "0:0"],
                                 cwd=work, input=b"qa-init\nqa-mac80211_hwsim.ko\n",
                                 capture_output=True, check=True).stdout
        initrd = work / "lab.cpio.gz"
        with initrd.open("wb") as out, (args.images / "rootfs.cpio.gz").open("rb") as base:
            shutil.copyfileobj(base, out)
            out.write(gzip.compress(overlay, mtime=0))
        acceleration = "kvm" if os.access("/dev/kvm", os.R_OK | os.W_OK) else "tcg"
        command = [qemu, "-machine", "q35", "-accel", acceleration, "-m", "768",
                   "-display", "none", "-serial", "stdio", "-monitor", "none",
                   "-nic", "none", "-no-reboot", "-kernel", str(args.images / "bzImage"),
                   "-initrd", str(initrd), "-append", "console=ttyS0,115200n8 rdinit=/qa-init"]
        with (run / "serial.log").open("wb") as log, subprocess.Popen(
                command, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.STDOUT) as vm:
            try:
                test_association(vm, log)
            finally:
                vm.kill()
    print("Usuário comum conectou ao ponto de acesso WPA2 pelos rádios mac80211_hwsim.")


if __name__ == "__main__":
    main()
