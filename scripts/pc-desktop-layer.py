#!/usr/bin/env python3
"""Monta runtimes Wayland de aplicativos a partir de pacotes fixados."""

import argparse
from concurrent.futures import ThreadPoolExecutor
from hashlib import file_digest
from pathlib import Path
import shutil
import subprocess
from tempfile import TemporaryDirectory

BASE = "docker.io/library/alpine@sha256:0f49f331d9212aff11b932863be4eb01cdf962969aa86dab7d1ee21ac6f5f52b"
DEBIAN_BASE = "docker.io/library/debian@sha256:7378260c0a098d27ac206225db5e7d92b8c7c84ea7afccc86eeb513756bbd392"
REPO = "https://dl-cdn.alpinelinux.org/alpine/v3.24"
LOCK = Path(__file__).resolve().parents[1] / "apps/desktop/alpine-3.24.lock"
DEBIAN_LOCK = LOCK.with_name("debian-13.lock")
SDK_LOCK = LOCK.with_name("alpine-3.24-sdk.lock")

INSTALL = r"""
mkdir /work/runtime
apk --no-network --repositories-file /dev/null --keys-dir /etc/apk/keys \
    --root /work/runtime --initdb --no-progress add --force-non-repository /packages/*.apk
cd /work/runtime
mkdir -p home run proc tmp
touch etc/resolv.conf
printf '%s\n' 'melkor:x:1000:1000:Melkor:/home/browser:/bin/sh' > etc/passwd
find . -type f \( -perm -4000 -o -perm -2000 \) -exec chmod a-s {} +
tar -czf /work/layer.tar.gz .
"""

INSTALL_DEBIAN = r"""
export DEBIAN_FRONTEND=noninteractive
printf '#!/bin/sh\nexit 101\n' > /usr/sbin/policy-rc.d
chmod 0755 /usr/sbin/policy-rc.d
apt-get --yes --no-install-recommends \
    -o Dir::Etc::sourcelist=/dev/null -o Dir::Etc::sourceparts=/dev/null install /packages/*.deb
cd /work
install -m 0755 /packages/Blanc-1.51.0.AppImage ./blanc.AppImage
./blanc.AppImage --appimage-extract > blanc-extract.log
mv squashfs-root /opt/blanc
find /opt -type f \( -perm -4000 -o -perm -2000 \) -exec chmod a-s {} +
printf '%s\n' 'melkor:x:1000:1000:Melkor:/home/browser:/bin/sh' >> /etc/passwd
tar --exclude=./var/cache/apt --exclude=./var/lib/apt/lists --exclude='./dev/*' \
    --exclude='./sys/*' --exclude=./work --exclude=./packages --exclude=./etc/hosts --exclude=./etc/hostname \
    --exclude='./proc/*' --exclude='./tmp/*' --exclude='./run/*' -C / -czf /work/layer.tar.gz .
"""

BUILD_SESSION = r"""
apk --no-network --repositories-file /dev/null --no-progress add --force-non-repository /packages/*.apk
export TMPDIR=/work
mkdir -p /usr/include/stb
for header in /usr/include/stb_*.h; do ln -s "$header" /usr/include/stb/; done
mkdir /work/src-noctalia /work/src-umbriel
tar -xzf /packages/noctalia-v5.6.0.tar.gz -C /work/src-noctalia --strip-components=1
tar -xzf /packages/umbriel.tar.gz -C /work/src-umbriel --strip-components=1
meson setup /work/build-umbriel /work/src-umbriel --prefix=/opt/melkor-desktop/usr \
    --buildtype=release -Dtests=disabled -Djemalloc=disabled -Dlocaled=disabled
meson setup /work/build-noctalia /work/src-noctalia --prefix=/opt/melkor-desktop/usr \
    --buildtype=release -Dtests=disabled -Djemalloc=disabled
for app in umbriel noctalia; do
    meson compile -C "/work/build-$app" -j 6
    meson install -C "/work/build-$app" --destdir /work/stage --no-rebuild
    install -Dm644 "/work/src-$app/LICENSE" "/work/stage/opt/melkor-desktop/usr/share/licenses/$app/LICENSE"
done
tar -C /work/stage/opt/melkor-desktop -czf /work/layer.tar.gz usr
"""


def fetch(cache, line):
    digest, source, filename = line.split()
    url = source if source.startswith("https://") else f"{REPO}/{source}/x86_64/{filename}"
    dest = cache / filename

    def matches(path):
        with path.open("rb") as stream:
            return file_digest(stream, "sha256").hexdigest() == digest

    if dest.is_file():
        if not matches(dest):
            raise ValueError(f"SHA-256 divergente no cache: {filename}")
        return dest
    with TemporaryDirectory(dir=cache) as scratch:
        candidate = Path(scratch) / filename
        subprocess.run(["curl", "-fsSL", "--proto", "=https", "--proto-redir", "=https",
                        "--connect-timeout", "15", "--max-time", "300", "--retry", "2",
                        "--output", candidate, url], check=True)
        if not matches(candidate):
            raise ValueError(f"SHA-256 divergente no download: {filename}")
        candidate.replace(dest)
    return dest


def assemble(cache, output, profile="alpine"):
    cache.mkdir(parents=True, exist_ok=True)
    output.parent.mkdir(parents=True, exist_ok=True)
    with TemporaryDirectory(dir=output.parent) as directory, ThreadPoolExecutor(max_workers=6) as pool:
        work = Path(directory)
        packages = work / "packages"
        packages.mkdir()
        lock, base, install = {
            "alpine": (LOCK, BASE, INSTALL),
            "chromium": (DEBIAN_LOCK, DEBIAN_BASE, INSTALL_DEBIAN),
            "session": (SDK_LOCK, BASE, BUILD_SESSION),
        }[profile]
        for package in pool.map(lambda line: fetch(cache, line), lock.read_text().splitlines()):
            shutil.copyfile(package, packages / package.name)
        subprocess.run(["podman", "run", "--rm", "--network=none", "--security-opt", "label=disable",
                        "-v", f"{work}:/work", "-v", f"{packages}:/packages:ro", base, "sh", "-ec", install],
                       check=True)
        (work / "layer.tar.gz").replace(output)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("cache", type=Path)
    parser.add_argument("output", type=Path)
    parser.add_argument("--profile", choices=("alpine", "chromium", "session"), default="alpine")
    args = parser.parse_args()
    assemble(args.cache.resolve(), args.output.resolve(), args.profile)


if __name__ == "__main__":
    main()
