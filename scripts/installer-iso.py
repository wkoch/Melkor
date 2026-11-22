#!/usr/bin/env python3
"""Monta a ISO UEFI com o core de instalação dentro do initramfs."""

import argparse
import gzip
import hashlib
import os
from pathlib import Path
import shutil
import subprocess
import tempfile


MENU = """serial --unit=0 --speed=115200
terminal_input console serial
terminal_output console serial
set timeout=5
set default=0

menuentry 'Instalar Melkor' {
    linux /bzImage quiet console=tty0
    initrd /rootfs.cpio.gz
}

menuentry 'Instalar Melkor (serial)' {
    linux /bzImage quiet console=tty0 console=ttyS0,115200n8 melkor.serial=1
    initrd /rootfs.cpio.gz
}
"""


def asset(output, name):
    checksums = dict(line.split()[::-1] for line in (output / "SHA256SUMS").read_text().splitlines())
    path = output / "build/images" / name
    with path.open("rb") as source:
        digest = hashlib.file_digest(source, "sha256").hexdigest()
    if digest != checksums.get(name):
        raise ValueError(f"SHA-256 divergente no build {output.name}: {name}")
    return path, digest


def prepare_media(installer, pc, output):
    kernel, _ = asset(installer, "bzImage")
    initramfs, _ = asset(installer, "rootfs.cpio.gz")
    core, sha256 = asset(pc, "rootfs.squashfs")
    boot, _ = asset(pc, "efi-part/EFI/BOOT/bootx64.efi")
    host = installer / "build/host/bin"
    board = Path(__file__).resolve().parents[1] / "core/board"
    env = os.environ | {"SOURCE_DATE_EPOCH": "0"}
    with tempfile.TemporaryDirectory(prefix=".installer-", dir=output.parent) as tmp:
        work = Path(tmp)
        tree = work / "iso"
        crypto = work / "payload/melkor"
        crypto.mkdir(parents=True)
        for source, target in ((core, "rootfs.squashfs"), (boot, "bootx64.efi"), (board / "pc/grub.cfg", "grub-pc.cfg")):
            shutil.copyfile(source, crypto / target)
        (crypto / "core.sha256").write_text(sha256 + "\n")
        entries = [crypto, *sorted(crypto.iterdir())]
        for entry in entries:
            os.utime(entry, (0, 0))
        archive = work / "payload.cpio"
        with archive.open("wb") as stream:
            subprocess.run(["cpio", "--quiet", "--reproducible", "-o", "-H", "newc", "-R", "0:0"],
                           input="".join(f"{p.relative_to(crypto.parent)}\n" for p in entries).encode(),
                           stdout=stream, cwd=crypto.parent, check=True)

        (tree / "boot/grub").mkdir(parents=True)
        (tree / "EFI/BOOT").mkdir(parents=True)
        (tree / "melkor-install.marker").touch()
        (tree / "boot/grub/grub.cfg").write_text(MENU)
        shutil.copyfile(kernel, tree / "bzImage")
        with (tree / "rootfs.cpio.gz").open("wb") as dst, initramfs.open("rb") as base, archive.open("rb") as extra:
            shutil.copyfileobj(base, dst)
            with gzip.GzipFile(filename="", fileobj=dst, mode="wb", mtime=0, compresslevel=1) as compressed:
                shutil.copyfileobj(extra, compressed)

        bootstrap = work / "bootstrap.cfg"
        bootstrap.write_text("search --no-floppy --file --set=root /melkor-install.marker\n"
                             "configfile /boot/grub/grub.cfg\n")
        loader = tree / "EFI/BOOT/bootx64.efi"
        modules = installer / "build/target/lib/grub/x86_64-efi"
        subprocess.run([host / "grub-mkimage", "-d", modules, "-O", "x86_64-efi", "-p", "/boot/grub", "-o", loader,
                        "-c", bootstrap, "boot", "linux", "fat", "iso9660", "part_gpt", "part_msdos",
                        "normal", "serial", "terminal", "search", "search_fs_file", "configfile"], check=True)
        efi = tree / "boot/efi.img"
        with efi.open("wb") as stream:
            stream.truncate(16 << 20)
        subprocess.run(["mkfs.vfat", "-F", "16", "-i", "4D454C4B", efi], check=True)
        subprocess.run([host / "mcopy", "-s", "-i", efi, tree / "EFI", "::"], check=True)
        image = work / "melkor-installer.iso"
        subprocess.run([host / "xorriso", "-as", "mkisofs", "-R", "-V", "MELKOR_INSTALL_x86_64",
                        "--sort-weight", "10", "/rootfs.cpio.gz", "--sort-weight", "9", "/bzImage",
                        "--sort-weight", "8", "/EFI/BOOT/bootx64.efi", "--sort-weight", "7", "/boot/grub/grub.cfg",
                        "-e", "boot/efi.img", "-no-emul-boot", "-o", image, tree], env=env, check=True)
        os.link(image, output)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("installer", type=Path)
    parser.add_argument("pc", type=Path)
    parser.add_argument("output", type=Path)
    args = parser.parse_args()
    try:
        prepare_media(args.installer.resolve(), args.pc.resolve(), args.output.resolve())
    except (OSError, ValueError, subprocess.CalledProcessError) as error:
        parser.exit(1, f"{error}\n")


if __name__ == "__main__":
    main()
