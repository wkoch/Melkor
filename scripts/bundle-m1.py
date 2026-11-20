#!/usr/bin/env python3
"""Empacota e assina uma geração do core M1 com a ferramenta RAUC do build."""

import argparse
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tempfile


COMPATIBLE = "melkor-m1-x86_64"
SLOT_SIZE = 128 * 1024 * 1024


def create_bundle(rauc, image, certificate, key, keyring, version, output, compatible=COMPATIBLE):
    if not re.fullmatch(r"[0-9][a-zA-Z0-9._-]{0,63}", version):
        raise ValueError("Informe uma versão iniciada por dígito e sem espaços.")
    for path in (rauc, image, certificate, key, keyring):
        if not path.is_file():
            raise ValueError(f"Arquivo necessário ausente: {path}.")
    if output.exists():
        raise ValueError(f"O bundle já existe: {output}.")
    with image.open("rb") as stream:
        if stream.read(4) != b"hsqs" or image.stat().st_size > SLOT_SIZE:
            raise ValueError("O payload deve ser SquashFS e caber no slot M1 de 128 MiB.")

    with tempfile.TemporaryDirectory(prefix=".bundle-", dir=output.parent) as scratch:
        work = Path(scratch)
        content = work / "content"
        content.mkdir()
        shutil.copyfile(image, content / "rootfs.squashfs")
        (content / "manifest.raucm").write_text(
            f"[update]\ncompatible={compatible}\nversion={version}\n\n"
            "[bundle]\nformat=verity\n\n"
            "[image.rootfs]\nfilename=rootfs.squashfs\n", encoding="ascii")
        candidate = work / "update.raucb"
        env = os.environ | {"PATH": f"{rauc.resolve().parent}{os.pathsep}{os.defpath}"}
        subprocess.run([rauc, "bundle", f"--cert={certificate}", f"--key={key}",
                        f"--signing-keyring={keyring}", content, candidate], env=env, check=True)
        os.link(candidate, output)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("build", type=Path)
    parser.add_argument("--cert", type=Path, required=True)
    parser.add_argument("--key", type=Path, required=True)
    parser.add_argument("--keyring", type=Path, required=True)
    parser.add_argument("--version", required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    try:
        create_bundle(args.build / "host/bin/rauc", args.build / "images/rootfs.squashfs",
                      args.cert, args.key, args.keyring, args.version, args.output)
    except (OSError, ValueError, subprocess.CalledProcessError) as error:
        print(error, file=sys.stderr)
        return 1
    print(f"Bundle assinado em {args.output}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
