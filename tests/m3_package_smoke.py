#!/usr/bin/env python3
"""Exercita a importação com Podman rootless e uma cópia local do RPM tree do Fedora."""

import argparse
import hashlib
import json
import os
from pathlib import Path
import shlex
import subprocess
import tempfile


DEBIAN = "docker.io/library/python@sha256:d98cc29d3266e535dd9da1099688d9cb3ca4753813571e071763d17725460962"


def digest(path):
    with path.open("rb") as stream:
        return hashlib.file_digest(stream, "sha256").hexdigest()


def make_deb(work, name, dependency, version, run):
    package = work / f"{name}-src"
    for directory in ("DEBIAN", "usr/bin", "usr/share/melkor-probe"):
        (package / directory).mkdir(parents=True)
    (package / "DEBIAN/control").write_text(
        f"Package: melkor-import-probe\nVersion: {version}\nArchitecture: all\n"
        f"Maintainer: Melkor Test\nDepends: {dependency}\nDescription: Teste da instalação isolada\n"
    )
    scripts = {
        "DEBIAN/postinst": """#!/bin/sh
set -eu
test "$(ls /sys/class/net)" = lo
command -v tree >/dev/null
printf 'postinst executado sem rede\\n' > /usr/share/melkor-probe/marker
""",
        "usr/bin/melkor-import-probe": """#!/bin/sh
set -eu
test "$(id -u)" -ne 0
grep -Fx 'postinst executado sem rede' /usr/share/melkor-probe/marker
test "$(ls /sys/class/net)" = lo
test ! -e /run/podman/podman.sock
test ! -w /usr/bin
tree --version
case "$1" in
    gravar) printf 'dado isolado\\n' > "$HOME/prova";;
    ler) grep -Fx 'dado isolado' "$HOME/prova";;
esac
""",
    }
    for path, script in scripts.items():
        file = package / path
        file.write_text(script)
        file.chmod(0o755)
    run("build-" + name, "podman", "run", "--rm", "--network=none", "--pull=missing",
        "--volume", f"{work}:/out:Z", "--entrypoint", "/usr/bin/dpkg-deb", DEBIAN,
        "--build", f"/out/{package.name}", f"/out/{name}.deb")
    return work / f"{name}.deb"


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("melkor", type=Path)
    parser.add_argument("rpm", type=Path)
    parser.add_argument("sha256")
    parser.add_argument("--log-dir", required=True, type=Path)
    args = parser.parse_args()
    args.log_dir.mkdir(parents=True, exist_ok=True)
    work = Path(tempfile.mkdtemp(prefix="pacotes-", dir=args.log_dir.resolve()))
    print(f"Evidências em {work}", flush=True)
    profile = work / "perfil"
    env = os.environ | {"MELKOR_PERFIL": str(profile)}
    report = {"concluido": []}

    def run(label, *command, rejected=False, message=None):
        with (work / f"{label}.log").open("w") as log:
            log.write(shlex.join(map(str, command)) + "\n")
            log.flush()
            result = subprocess.run(command, stdout=log, stderr=subprocess.STDOUT, env=env, timeout=900)
        if result.returncode != int(rejected):
            raise RuntimeError(f"{label} saiu com {result.returncode}, esperado {int(rejected)}.")
        if message and message not in (work / f"{label}.log").read_text(errors="replace"):
            raise RuntimeError(f"{label} não mostrou o resultado esperado.")
        report["concluido"].append(label)

    melkor = args.melkor.resolve()
    try:
        rpm = args.rpm.resolve(strict=True)
        if digest(rpm) != args.sha256:
            raise ValueError("SHA-256 do RPM de teste divergente.")
        deb = make_deb(work, "dependencia", "tree", "1.0-1", run)
        broken = make_deb(work, "ausente", "melkor-dependencia-inexistente", "2.0-1", run)

        run("importar-deb", melkor, "importar", "deb", deb, digest(deb), "/usr/bin/melkor-import-probe", "verificar")
        run("importar-rpm", melkor, "importar", "rpm", rpm, args.sha256, "/usr/bin/tree", "--version")
        run("gravar", melkor, "executar", "deb", "gravar", message="postinst executado sem rede")
        run("ler", melkor, "executar", "deb", "ler", message="dado isolado")
        run("executar-rpm", melkor, "executar", "rpm", "--version", message="tree v")

        active = (profile / "atual").readlink()
        run("rejeitar-hash", melkor, "importar", "deb", deb, "0" * 64, "/usr/bin/melkor-import-probe",
            rejected=True, message="SHA-256 do pacote divergente")
        run("rejeitar-dependencia", melkor, "importar", "deb", broken, digest(broken), "/usr/bin/melkor-import-probe",
            rejected=True, message="melkor-dependencia-inexistente")
        corrupt = work / "corrompido.rpm"
        content = bytearray(rpm.read_bytes())
        content[-1] ^= 1
        corrupt.write_bytes(content)
        run("rejeitar-rpm", melkor, "importar", "rpm", corrupt, digest(corrupt), "/usr/bin/tree",
            rejected=True, message="Signature verification failed")
        if (profile / "atual").readlink() != active:
            raise RuntimeError("Uma importação reprovada alterou a geração ativa.")
        run("remover-deb", melkor, "remover", "deb")
        run("executar-removido", melkor, "executar", "deb", "ler", rejected=True)
        run("rpm-apos-remocao", melkor, "executar", "rpm", "--version", message="tree v")
        if (profile / "dados/deb/prova").read_text() != "dado isolado\n":
            raise RuntimeError("Os dados do pacote removido foram alterados.")
        report["codigo_saida"] = 0
        print("Importação, scripts offline, dados, rejeições e remoção verificados.")
    except (OSError, RuntimeError, ValueError, subprocess.TimeoutExpired) as error:
        report.update(codigo_saida=1, erro=str(error))
        print(error)
    finally:
        (work / "result.json").write_text(json.dumps(report, indent=2) + "\n")
    return report["codigo_saida"]


if __name__ == "__main__":
    raise SystemExit(main())
