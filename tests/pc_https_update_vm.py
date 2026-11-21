#!/usr/bin/env python3
"""Valida download por TLS e promoção do slot do PC em uma VM descartável."""

import argparse
from contextlib import contextmanager
from functools import partial
from http.server import SimpleHTTPRequestHandler, ThreadingHTTPServer
import os
from pathlib import Path
import re
import shutil
import ssl
import subprocess
import tempfile
from threading import Thread
from types import SimpleNamespace

from m1_update_vm import (
    BOOT, BUNDLE, DOCUMENT, confirm, create_bundle, fixture_images, grub_state, guest, machine, slot, slot_hash,
)


@contextmanager
def https_server(work, cert, key):
    handler = partial(SimpleHTTPRequestHandler, directory=work)
    with ThreadingHTTPServer(("127.0.0.1", 0), handler) as server:
        tls = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
        tls.load_cert_chain(cert, key)
        server.socket = tls.wrap_socket(server.socket, server_side=True)
        worker = Thread(target=server.serve_forever, daemon=True)
        worker.start()
        try:
            yield f"https://10.0.2.2:{server.server_port}"
        finally:
            server.shutdown()
            worker.join()


def fixtures(args, work):
    http = work / "http"
    http.mkdir()
    bundle = http / "update.raucb"
    fixture_images(args.build, work, outcomes=("healthy",))
    create_bundle(args.build / "host/bin/rauc", work / "healthy.squashfs",
                  args.cert, args.key, args.cert, "2.0-https-test", bundle,
                  compatible=BUNDLE["PC_COMPATIBLE"], slot_size=BUNDLE["PC_SLOT_SIZE"])
    data = bytearray(bundle.read_bytes())
    signature_size = int.from_bytes(data[-8:], "big")
    signature_start = len(data) - 8 - signature_size
    assert 4096 < signature_start < len(data) - 8
    data[signature_start + signature_size // 2] ^= 1
    invalid = http / "invalid.raucb"
    invalid.write_bytes(data)

    seed_files = work / "seed"
    seed_files.mkdir()
    tls_cert, tls_key = seed_files / "tls.pem", work / "tls.key"
    subprocess.run(["openssl", "req", "-x509", "-newkey", "rsa:2048", "-nodes", "-days", "2",
                    "-subj", "/CN=Melkor HTTPS lab", "-addext", "subjectAltName=IP:10.0.2.2",
                    "-out", tls_cert, "-keyout", tls_key], check=True)
    seed = work / "seed.ext4"
    with seed.open("wb") as stream:
        stream.truncate(16 << 20)
    subprocess.run(["mke2fs", "-q", "-F", "-t", "ext4", "-m", "0", "-O", "^has_journal",
                    "-d", seed_files, seed], check=True)
    return http, tls_cert, tls_key, seed, BOOT["file_digest"](bundle), BOOT["file_digest"](invalid)


def exercise(args, work):
    password = args.root_password_file.read_bytes().strip()
    if not re.fullmatch(rb"[0-9a-f]{32}", password):
        raise ValueError("Use a senha descartável gerada por openssl rand -hex 16.")
    firmware = SimpleNamespace(
        ovmf_code=Path(code) if (code := os.environ.get("OVMF_CODE")) else None,
        ovmf_vars=Path(variables) if (variables := os.environ.get("OVMF_VARS")) else None)
    code, firmware_vars = BOOT["find_firmware"](firmware)
    accel = os.environ.get("QEMU_ACCEL", "auto")
    if accel == "auto":
        accel = "kvm" if os.access("/dev/kvm", os.R_OK | os.W_OK) else "tcg"
    if accel not in ("kvm", "tcg"):
        raise ValueError("QEMU_ACCEL deve ser auto, kvm ou tcg.")
    timeout = 120 if accel == "kvm" else 300

    http, cert, key, seed, digest, invalid_digest = fixtures(args, work)
    disk, variables = work / "disk.img", work / "OVMF_VARS.fd"
    subprocess.run(["cp", "--reflink=auto", "--sparse=always", "--", args.image, disk], check=True)
    shutil.copyfile(firmware_vars, variables)
    update = "/usr/sbin/melkor-core-update --https"
    ca = "SSL_CERT_FILE=/run/seed/tls.pem"
    trusted = f"{ca} {update}"
    clean = 'set -- /persist/rauc/.download-*; test ! -e "$1"'

    with https_server(http, cert, key) as url:
        bundle = f"{url}/update.raucb"
        invalid = f"{url}/invalid.raucb"
        with machine(work, "01-download-a", disk, seed, code, variables, timeout, accel, network=True) as console:
            BOOT["login"](console, b"root", password, rb"^[^\n]*# ")
            slot(console, "A")
            confirm(console, "A")
            guest(console, f"printf 'persistiu\\n' > {DOCUMENT} && chown melkor {DOCUMENT}")
            guest(console, "mkdir -p /run/seed && mount -t ext4 -o ro /dev/vdb /run/seed")
            guest(console, "for attempt in $(seq 1 20); do "
                  "ip -4 route show default | grep -q . && exit 0; sleep 1; done; exit 1")
            original = grub_state(console)
            a_hash, b_hash = slot_hash(console, "a"), slot_hash(console, "b")
            guest(console, f"{ca} flock /run/melkor/rauc.lock {update} {bundle} {digest}", allowed=(1,))
            for command, expected in ((f"{update} {bundle} {digest}", b"x509:"),
                                      (f"{trusted} {bundle} {'0' * 64}", b"SHA-256"),
                                      (f"{trusted} {invalid} {invalid_digest}", b"signature verification failed")):
                result = guest(console, command, allowed=(1,))
                assert expected.lower() in result.lower(), f"erro esperado ausente: {expected}"
                assert grub_state(console) == original, "rejeição alterou o estado de boot"
                assert slot_hash(console, "b") == b_hash, "rejeição gravou no slot inativo"
                guest(console, clean)
            guest(console, f"{trusted} {bundle} {digest}")
            guest(console, clean)
            assert grub_state(console)[b"ORDER"] == b"B A"
            assert slot_hash(console, "a") == a_hash, "atualização alterou o slot ativo"
            guest(console, "sync")

    with machine(work, "02-first-b", disk, seed, code, variables, timeout, accel) as console:
        BOOT["login"](console, b"root", password, rb"^[^\n]*# ")
        slot(console, "B")
        confirm(console, "B")
        guest(console, "grep -Fx healthy /etc/melkor-test-generation")
        guest(console, f"grep -Fx persistiu {DOCUMENT}")
        guest(console, "sync")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("build", type=Path)
    parser.add_argument("image", type=Path)
    parser.add_argument("--cert", type=Path, required=True)
    parser.add_argument("--key", type=Path, required=True)
    parser.add_argument("--root-password-file", type=Path, required=True)
    parser.add_argument("--log-dir", type=Path, required=True)
    args = parser.parse_args()
    args.log_dir.mkdir(parents=True, exist_ok=True)
    work = Path(tempfile.mkdtemp(prefix="https-", dir=args.log_dir.resolve()))
    print(f"Evidências em {work}", flush=True)
    exercise(args, work)
    print("TLS, SHA-256, assinatura RAUC, promoção A → B e persistência verificados.")


if __name__ == "__main__":
    main()
