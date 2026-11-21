#!/usr/bin/env python3
"""Verifica o resolvedor do Melkor pelo console e pela interface virtual."""

import argparse
import json
import os
from pathlib import Path
import re
import runpy
import shlex
import shutil
import socket
import subprocess
import tempfile
from types import SimpleNamespace


BOOT = runpy.run_path(str(Path(__file__).resolve().parents[1] / "scripts/test-boot.py"))
QUAD9 = "(dst host 9.9.9.9 or dst host 149.112.112.112 or dst host 2620:fe::fe or dst host 2620:fe::9)"
TXT = "nslookup -type=TXT -timeout=16 -retry=1 proto.on.quad9.net."


def guest(console, command, expected_exit=0):
    line = f"sh -c {shlex.quote(command)}; printf '\\nMELKOR-DNS-EXIT=%s\\n' \"$?\""
    console.send(line.encode("ascii"))
    result = console.expect(rb"^MELKOR-DNS-EXIT=[0-9]+\r?$", command)
    status = int(result.rsplit(b"=", 1)[1])
    if status != expected_exit:
        raise RuntimeError(f"O comando retornou {status}, esperado {expected_exit}: {command}")
    return result


def check_system(console, family):
    guest(console, f"for attempt in $(seq 1 20); do ip -{family} route show default | grep -q '^default' "
          "&& exit 0; sleep 1; done; exit 1")
    guest(console, r"""test ! -L /etc/resolv.conf &&
        test "$(awk '/^nameserver / {print $2}' /etc/resolv.conf | xargs)" = '127.0.0.1 ::1'"""
          .strip().replace("\n", " "))
    guest(console, "for attempt in $(seq 1 10); do pidof dnsproxy >/dev/null && break; sleep 1; done; "
          'set -- $(pidof dnsproxy); test "$#" -eq 1 && '
          "test \"$(awk '/^Uid:/ {print $2}' /proc/$1/status)\" = \"$(id -u melkor-dns)\" && "
          "grep -Eq '^CapEff:[[:space:]]*0000000000000400$' /proc/$1/status && "
          "grep -Eq '^NoNewPrivs:[[:space:]]*1$' /proc/$1/status")
    guest(console, "for table in tcp udp; do for suffix in '' 6; do "
          "case $suffix in 6) address=00000000000000000000000001000000;; *) address=0100007F;; esac; "
          "test \"$(awk '$2 ~ /:0035$/ {print $2}' /proc/net/$table$suffix | sort -u)\" "
          '= "$address:0035" || exit 1; done; done')


def check_packets(pcap, directory):
    def read(expression):
        return subprocess.check_output(["tcpdump", "-nn", "-r", pcap, expression], stderr=subprocess.DEVNULL)

    https = read(f"tcp dst port 443 and {QUAD9}")
    (directory / "quad9.txt").write_bytes(https)
    if not https:
        raise RuntimeError("Nenhum pacote TCP para o HTTPS do Quad9 foi capturado.")
    other_dns = read(f"(dst port 53 or dst port 853 or dst port 9953) or (dst port 443 and not {QUAD9})")
    (directory / "outros-destinos.txt").write_bytes(other_dns)
    if other_dns:
        raise RuntimeError("A captura contém tráfego DNS fora do DoH do Quad9.")


def block_network(path):
    with socket.socket(socket.AF_UNIX) as monitor:
        monitor.settimeout(5)
        monitor.connect(str(path))
        with monitor.makefile("rwb") as stream:
            greeting = json.loads(stream.readline())
            if "QMP" not in greeting:
                raise RuntimeError("O monitor do QEMU não enviou a saudação QMP.")
            for command in (
                {"execute": "qmp_capabilities"},
                {"execute": "qom-set", "arguments": {"path": "/objects/block", "property": "status", "value": "on"}},
                {"execute": "qom-set", "arguments": {"path": "/objects/trace", "property": "status", "value": "on"}},
            ):
                stream.write(json.dumps(command).encode("ascii") + b"\n")
                stream.flush()
                while "event" in (reply := json.loads(stream.readline())):
                    pass
                if "return" not in reply:
                    raise RuntimeError(f"O monitor do QEMU recusou a alteração do filtro: {reply}")


def check_doh(console, server=""):
    result = guest(console, f"{TXT} {server}".rstrip())
    if not re.search(rb'\btext = "doh"', result):
        raise RuntimeError("O TXT do Quad9 não confirmou DoH.")


def run(plan, work, isolated):
    work.mkdir()
    optical = plan.image.suffix == ".iso"
    disk, variables, capture = work / "disk.img", work / "OVMF_VARS.fd", work / "traffic.pcap"
    if optical:
        disk = plan.image
    else:
        subprocess.run(["cp", "--reflink=auto", "--sparse=always", "--", plan.image, disk], check=True)
    shutil.copyfile(plan.variables, variables)

    def drive(path, options):
        return f"file={str(path).replace(',', ',,')},format=raw,{options}"

    network = "user,id=net,ipv4=off,ipv6=on" if plan.family == 6 else "user,id=net,ipv6=off"
    trace = f"filter-dump,id=trace,netdev=net,file={str(capture).replace(',', ',,')}"
    if isolated:
        trace += ",status=off"
    monitor = work / "qmp.sock"
    command = [plan.qemu, "-machine", "q35", "-accel", plan.accel, "-m", "512",
               "-display", "none", "-serial", "stdio", "-monitor", "none", "-no-reboot",
               "-netdev", network, "-device", "virtio-net-pci,netdev=net",
               "-object", trace,
               "-drive", drive(plan.code, "if=pflash,unit=0,readonly=on"),
               "-drive", drive(variables, "if=pflash,unit=1")]
    if optical:
        command.extend(("-drive", drive(disk, "if=none,id=melkor,media=cdrom"),
                        "-device", "ide-cd,bus=ide.0,drive=melkor,bootindex=1"))
    else:
        command.extend(("-drive", drive(disk, "if=none,id=melkor"),
                        "-device", "virtio-blk-pci,drive=melkor,bootindex=1"))
    if isolated:
        # Em RX do backend, o QEMU percorre os filtros da cauda para a cabeça.
        # A captura precisa ver o tráfego do convidado antes do descarte.
        command.extend(("-qmp", f"unix:{str(monitor).replace(',', ',,')},server=on,wait=off",
                        "-chardev", "null,id=discard",
                        "-object", "filter-redirector,id=block,netdev=net,queue=rx,position=head,"
                                   "outdev=discard,status=off"))
    (work / "qemu.command").write_text(shlex.join(command) + "\n")
    with (work / "serial.log").open("wb") as log, subprocess.Popen(
        command, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.STDOUT
    ) as process:
        try:
            with BOOT["serial_console"](process, log, plan.timeout) as console:
                BOOT["login"](console, b"melkor", b"melkor", rb"^[^\n]*\$ ")
                check_system(console, plan.family)
                if isolated:
                    check_doh(console)
                    block_network(monitor)
                    guest(console, TXT, expected_exit=1)
                else:
                    for server in ("", "127.0.0.1", "::1"):
                        check_doh(console, server)
        finally:
            process.terminate()
            try:
                tail, _ = process.communicate(timeout=5)
            except subprocess.TimeoutExpired:
                process.kill()
                tail, _ = process.communicate()
            log.write(tail)
    check_packets(capture, work)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("image", type=Path)
    parser.add_argument("--log-dir", type=Path, required=True)
    parser.add_argument("--ipv6-only", action="store_true")
    args = parser.parse_args()
    args.log_dir.mkdir(parents=True, exist_ok=True)
    work = Path(tempfile.mkdtemp(prefix="doh-", dir=args.log_dir.resolve()))
    print(f"Evidências em {work}", flush=True)
    report = {"cenarios": [], "codigo_saida": 1}
    try:
        qemu = os.environ.get("QEMU_SYSTEM") or shutil.which("qemu-system-x86_64", path=BOOT["SYSTEM_PATH"])
        if not qemu or not shutil.which("tcpdump"):
            raise RuntimeError("Instale qemu-system-x86_64 e tcpdump.")
        firmware = SimpleNamespace(
            ovmf_code=Path(code) if (code := os.environ.get("OVMF_CODE")) else None,
            ovmf_vars=Path(variables) if (variables := os.environ.get("OVMF_VARS")) else None,
        )
        code, variables = BOOT["find_firmware"](firmware)
        accel = os.environ.get("QEMU_ACCEL", "auto")
        if accel == "auto":
            accel = "kvm" if os.access("/dev/kvm", os.R_OK | os.W_OK) else "tcg"
        if accel not in ("kvm", "tcg"):
            raise ValueError("QEMU_ACCEL deve ser auto, kvm ou tcg.")
        plan = SimpleNamespace(image=args.image.resolve(), code=code, variables=variables, qemu=qemu,
                               accel=accel, timeout=120 if accel == "kvm" else 300, family=6 if args.ipv6_only else 4)
        digest = BOOT["file_digest"]
        report.update(commit=subprocess.check_output(["git", "rev-parse", "HEAD"], text=True).strip(),
                      imagem_sha256=digest(plan.image), firmware_sha256=digest(code),
                      variaveis_uefi_sha256=digest(variables), acelerador=accel, prazo_segundos=plan.timeout,
                      somente_ipv6=args.ipv6_only)
        for name, isolated in (("quad9", False), ("sem-egresso", True)):
            run(plan, work / name, isolated)
            report["cenarios"].append(name)
        report["codigo_saida"] = 0
        print("Quad9 respondeu por DoH. A falha de acesso não recorreu a DNS em texto claro.")
    except (OSError, RuntimeError, ValueError, subprocess.CalledProcessError) as error:
        report["erro"] = str(error)
        print(error)
    finally:
        (work / "result.json").write_text(json.dumps(report, indent=2) + "\n")
    return report["codigo_saida"]


if __name__ == "__main__":
    raise SystemExit(main())
