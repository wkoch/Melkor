#!/usr/bin/env python3
"""Ferramentas locais para os testes dos scripts de build."""

import json
import os
from pathlib import Path
import sys


commit = "2f9129e7909bff6d78ae8cfda3d0e73503dfcac6"
tool = Path(sys.argv[0]).name
args = sys.argv[1:]
with Path(os.environ["TEST_TOOL_LOG"]).open("a") as log:
    log.write(json.dumps([tool, *args]) + "\n")

if tool == "git":
    if "bundle" in args:
        Path(args[args.index("create") + 1]).write_text("bundle simulado")
    if "rev-parse" in args:
        print(os.environ.get("TEST_COMMIT", commit))
elif tool == "make":
    build = Path(next(value[2:] for value in args if value.startswith("O=")))
    if args[-1].endswith("_defconfig"):
        build.mkdir(parents=True, exist_ok=True)
        options = [
            "BR2_TOOLCHAIN_BUILDROOT_MUSL", "BR2_GCC_VERSION_14_X", "BR2_INIT_NONE",
            "BR2_PACKAGE_DINIT", "BR2_TARGET_GRUB2_X86_64_EFI",
            "BR2_TARGET_ROOTFS_CPIO_GZIP" if args[-1] in ("melkor_live_defconfig", "melkor_installer_defconfig")
            else "BR2_TARGET_ROOTFS_SQUASHFS",
            "BR2_DOWNLOAD_FORCE_CHECK_HASHES",
        ]
        (build / ".config").write_text("".join(f"{name}=y\n" for name in options
                                            if name != os.environ.get("TEST_MISSING_OPTION")))
        (build / "board").write_text(args[-1].split("_")[1])
    elif args[-1] == "source":
        if os.environ.get("https_proxy") == "http://127.0.0.1:9":
            assert os.environ["NO_PROXY"] == "" and os.environ["no_proxy"] == ""
            if os.environ.get("TEST_OFFLINE_FAILURE"):
                print("fonte fora do cache")
                sys.exit(1)
    elif args[-1] == "show-info":
        print('{"mock":"manifesto"}')
    else:
        (build / "images").mkdir()
        board = (build / "board").read_text()
        (build / "images" / f"melkor-{board}.img").write_text("disco virtual")
        if board == "live":
            (build / "images/melkor-live.iso").write_text("mídia óptica virtual")
        if board == "installer":
            (build / "images/bzImage").write_text("kernel simulado")
            (build / "images/rootfs.cpio.gz").write_text("initramfs simulado")
        tools = build / "host/bin"
        tools.mkdir(parents=True)
        for name in ("gcc", "ld", "readelf"):
            (tools / f"x86_64-buildroot-linux-musl-{name}").symlink_to(Path(__file__).resolve())
elif tool.endswith("-readelf"):
    print("[Requesting program interpreter: /lib/ld-musl-x86_64.so.1]")
elif tool.endswith(("-gcc", "-ld")):
    print(tool)
elif tool == "rpm":
    if os.environ.get("TEST_MISSING_PACKAGE") in args:
        sys.exit(1)
elif tool == "distrobox":
    if args[0] == "list":
        if os.environ.get("TEST_DISTROBOX_FAILURE"):
            sys.exit(1)
        print("id | nome | estado")
        if not os.environ.get("TEST_CONTAINER_ABSENT"):
            print("id | melkor-build | running")
elif tool == "genimage":
    stage = Path(args[args.index("--inputpath") + 1])
    assert (stage / "EFI/BOOT/grub.cfg").is_file()
    assert (stage / "EFI/BOOT/bootx64.efi").read_text() == "EFI"
    if args[-1].endswith("/live/genimage.cfg"):
        assert (stage / "rootfs.cpio.gz").read_text() == "initramfs"
        assert (stage / "bzImage").read_text() == "kernel"
    elif (stage / "persist.ext4").exists():
        assert (stage / "rootfs.squashfs").read_text() == "rootfs"
        assert (stage / "persist.ext4").stat().st_size == 128 << 20
        assert (stage / "EFI/BOOT/grubenv").read_text().splitlines() == [
            "ORDER=A B", "A_OK=1", "A_TRY=0", "B_OK=0", "B_TRY=0",
        ]
    else:
        assert (stage / "rootfs.squashfs").read_text() == "rootfs"
        assert (stage / "bzImage").read_text() == "kernel"
    sys.exit(int(os.environ.get("TEST_GENIMAGE_CODE", "0")))
elif tool == "grub-editenv":
    Path(args[0]).write_text("\n".join(args[2:]) + "\n")
elif tool == "openssl":
    assert args[:3] == ["crl2pkcs7", "-nocrl", "-certfile"] and args[4:] == ["-out", "/dev/null"]
    assert Path(args[3]).read_text().startswith("-----BEGIN CERTIFICATE-----\n")
elif tool == "xorriso":
    assert args[args.index("-e") + 1] == "boot/efi.img" and "-no-emul-boot" in args
    assert args[-1].endswith("/melkor-live-eltorito.vfat")
    tree = Path(next(arg[2:] for arg in args if arg.startswith("/=")))
    assert (tree / "boot/grub/grub.cfg").read_bytes() == (tree / "EFI/BOOT/grub.cfg").read_bytes()
    assert (tree / "rootfs.cpio.gz").read_text() == "initramfs"
elif tool not in ("sudo", "mke2fs", "grub-script-check"):
    sys.exit(f"ferramenta de teste inesperada: {tool}")
