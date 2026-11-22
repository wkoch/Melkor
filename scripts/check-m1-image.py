#!/usr/bin/env python3
"""Valida o disco virtual do M1 sem montá-lo no host."""

import argparse
import base64
import json
from pathlib import Path
import subprocess
import sys


MIB = 1024 * 1024
ESP_TYPE = "c12a7328-f81f-11d2-ba4b-00a0c93ec93b"
LINUX_TYPE = "0fc63daf-8483-4772-8e79-3d69d8477de4"
QUAD9_ADDRESSES = ("9.9.9.9", "149.112.112.112", "[2620:fe::fe]", "[2620:fe::9]")


def capture(*command):
    return subprocess.check_output(command, text=True)


def require(condition, message):
    if not condition:
        raise ValueError(message)


def quad9_stamp(address):
    # DoH, DNSSEC e política sem registro de IP de usuário.
    data = b"\x02\x03" + b"\0" * 7
    for field in (address + ":443", "", "dns.quad9.net", "/dns-query"):
        data += bytes([len(field)]) + field.encode("ascii")
    return "sdns://" + base64.urlsafe_b64encode(data).decode("ascii").rstrip("=")


def check_dns(resolver, config):
    nameservers = [line.split()[1:] for line in resolver.splitlines() if line.split()[:1] == ["nameserver"]]
    require(nameservers == [["127.0.0.1"], ["::1"]], "O resolvedor do sistema precisa apontar para loopback.")
    settings = json.loads(config)
    require(isinstance(settings, dict)
            and settings.keys() <= {"listen-addrs", "listen-ports", "timeout", "max-go-routines",
                                    "upstream-mode", "upstream"}
            and settings.get("listen-addrs") == ["127.0.0.1", "::1"] and settings.get("listen-ports") == [53]
            and settings.get("upstream") == [quad9_stamp(address) for address in QUAD9_ADDRESSES],
            "O dnsproxy precisa escutar em loopback e usar apenas os endpoints DoH fixos do Quad9.")


def check(build):
    images = build / "images"
    image = images / "melkor-m1.img"
    require(image.is_file(), f"Disco virtual ausente: {image}. Execute make image BOARD=m1.")
    host = build / "host/bin"

    table = json.loads(capture("sfdisk", "--json", image))["partitiontable"]
    names = [partition["name"] for partition in table["partitions"]]
    require(table["label"] == "gpt" and names == ["esp", "core-a", "core-b", "persist"],
            "A imagem precisa conter ESP, slots A/B e dados, nessa ordem, em GPT.")
    subprocess.run(["sfdisk", "--verify", image], check=True)

    sector_size = table["sectorsize"]
    expected = ((ESP_TYPE, 64, "vfat"), (LINUX_TYPE, 128, "squashfs"),
                (LINUX_TYPE, 128, "squashfs"), (LINUX_TYPE, 128, "ext4"))
    filesystems = []
    for partition, (kind, size_mib, filesystem) in zip(table["partitions"], expected, strict=True):
        name = partition["name"]
        offset = partition["start"] * sector_size
        size = partition["size"] * sector_size
        require(offset % MIB == 0 and size == size_mib * MIB and partition["type"].lower() == kind,
                f"Alinhamento, tamanho ou tipo GPT inválido em {name}.")
        properties = dict(line.split("=", 1) for line in capture(
            "blkid", "-p", "-o", "export", "-O", str(offset), "-S", str(size), image
        ).splitlines())
        require(properties.get("TYPE") == filesystem, f"Sistema de arquivos inesperado em {name}.")
        filesystems.append(properties)

    def esp_file(name):
        offset = table["partitions"][0]["start"] * sector_size
        return subprocess.check_output([host / "mtype", "-i", f"{image}@@{offset}", f"::/EFI/BOOT/{name}"])

    def on_partition(index, name):
        offset = table["partitions"][index - 1]["start"] * sector_size
        return capture(host / "unsquashfs", "-offset", str(offset), "-cat", image, name)

    require(esp_file("BOOTX64.EFI") == (build / "host/share/melkor-limine/BOOTX64.EFI").read_bytes(),
            "Carregador EFI Limine inválido.")
    config = esp_file("limine.conf").decode("utf-8")
    entries = config.split("/Melkor Linux - slot ")
    require(len(entries) == 3 and entries[1].startswith("A\n") and entries[2].startswith("B\n"),
            "Ordem inicial de entradas Limine inválida.")
    kernel = (images / "bzImage").read_bytes()
    partuuids = [partition["uuid"].lower() for partition in table["partitions"][:3]]
    data_uuid = filesystems[3]["UUID"]
    for index, slot in enumerate(("A", "B"), start=2):
        partuuid = partuuids[index - 1]
        entry = entries[index - 1]
        require(f"root=PARTUUID={partuuid} " in entry and f"rauc.slot={slot}\n" in entry
                and f"path: boot():/EFI/BOOT/kernel-{slot}\n" in entry,
                f"O Limine precisa apontar o slot {slot} para o seu PARTUUID.")
        require(esp_file(f"kernel-{slot}") == kernel, f"Kernel inválido no slot {slot}.")
        require('VERSION_ID="m1"' in on_partition(index, "/etc/os-release").splitlines(),
                f"Identidade M1 ausente no slot {slot}.")
        require(f"UUID={data_uuid} /persist" in on_partition(index, "/usr/libexec/melkor-persist"),
                f"O slot {slot} precisa montar a partição de dados pelo UUID de filesystem.")
        setup = on_partition(index, "/usr/libexec/melkor-rauc-setup")
        require(all(uuid in setup for uuid in partuuids),
                f"O RAUC no slot {slot} precisa localizar a ESP e os slots pelos PARTUUIDs da imagem.")
        require("bundle-formats=verity" in on_partition(index, "/etc/rauc/system.conf").splitlines(),
                f"O RAUC no slot {slot} deve exigir bundles no formato verity.")
        check_dns(on_partition(index, "/etc/resolv.conf"), on_partition(index, "/etc/dnsproxy.json"))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("build", type=Path)
    args = parser.parse_args()
    try:
        check(args.build)
    except (OSError, ValueError, KeyError, subprocess.CalledProcessError) as error:
        print(error, file=sys.stderr)
        return 1
    print("GPT, carregador EFI, estado de boot, partições e configurações RAUC e Quad9 verificadas.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
