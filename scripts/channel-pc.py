#!/usr/bin/env python3
"""Assina o manifesto do canal PC com a chave Ed25519 do mantenedor."""

import argparse
from base64 import b64encode
from configparser import ConfigParser
from datetime import datetime, timedelta, timezone
from hashlib import file_digest
import json
import os
from pathlib import Path
import shutil
import subprocess
from tempfile import TemporaryDirectory
from urllib.parse import urlsplit


def sign(build, bundle, keyring, key, url, days, desktop=False):
    source = urlsplit(url)
    if source.scheme != "https" or not source.hostname or source.username is not None:
        raise ValueError("Informe a URL HTTPS do bundle sem credenciais.")
    if days < 1:
        raise ValueError("Informe a validade do manifesto em dias.")
    host = build / "host/bin"
    target = "melkor-pc-x86_64"
    with TemporaryDirectory(prefix="melkor-channel-") as directory:
        work = Path(directory)
        verified = work / "bundle.raucb"
        with bundle.open("rb") as source_bundle, verified.open("wb") as snapshot:
            shutil.copyfileobj(source_bundle, snapshot)
        contents = work / "contents"
        env = os.environ | {"PATH": f"{host.resolve()}{os.pathsep}{os.defpath}"}
        subprocess.run([host / "rauc", f"--keyring={keyring}", "extract", verified, contents],
                       env=env, check=True, stdout=subprocess.DEVNULL)
        manifest = ConfigParser(interpolation=None)
        manifest.read(contents / "manifest.raucm")
        if manifest.get("update", "compatible", fallback=None) != target:
            raise ValueError("Somente um bundle do PC pode entrar no canal.")
        image = contents / "rootfs.squashfs"
        revision = int(subprocess.check_output([host / "unsquashfs", "-cat", image, "etc/melkor-update/revision"]))
        if not 0 < revision <= 4294967295:
            raise ValueError("O bundle do PC precisa conter uma revisão de canal positiva.")
        with verified.open("rb") as stream:
            digest = file_digest(stream, "sha256").hexdigest()
        expires = datetime.now(timezone.utc).replace(microsecond=0) + timedelta(days=days)
        release = dict(schema=1, target=target, revision=revision, expires=expires.isoformat(), url=url, sha256=digest)
        if desktop:
            subprocess.check_output([host / "unsquashfs", "-cat", image, "etc/dinit.d/graphical-login"])
            release["profile"] = "desktop"
        payload = json.dumps(release).encode("utf-8")
        signed = work / "payload"
        signed.write_bytes(payload)
        signature = subprocess.check_output([host / "openssl", "pkeyutl", "-sign", "-rawin",
                                             "-inkey", key, "-in", signed])
    if len(signature) != 64:
        raise ValueError("Use uma chave privada Ed25519 para assinar o manifesto.")
    return json.dumps(dict(payload=b64encode(payload).decode(), signature=b64encode(signature).decode()))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("build", type=Path)
    parser.add_argument("bundle", type=Path)
    parser.add_argument("--keyring", required=True, type=Path)
    parser.add_argument("--key", required=True, type=Path)
    parser.add_argument("--url", required=True)
    parser.add_argument("--days", type=int, default=90)
    parser.add_argument("--desktop", action="store_true")
    parser.add_argument("--output", required=True, type=Path)
    args = parser.parse_args()
    try:
        document = sign(args.build, args.bundle, args.keyring, args.key, args.url, args.days, args.desktop)
        with TemporaryDirectory(prefix=".channel-", dir=args.output.parent) as directory:
            candidate = Path(directory) / args.output.name
            candidate.write_text(document + "\n", encoding="utf-8")
            candidate.replace(args.output)
    except (OSError, ValueError, subprocess.CalledProcessError) as error:
        parser.exit(1, f"{error}\n")


if __name__ == "__main__":
    main()
