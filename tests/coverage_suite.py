"""Executa a suíte de testes e aplica o mínimo de 80% por linguagem."""

import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
from tempfile import mkdtemp
import xml.etree.ElementTree as ET


ROOT = Path(__file__).resolve().parents[1]
MINIMUM = 80


def require_coverage(label, covered, total):
    if not total:
        raise SystemExit(f"{label}: relatório de cobertura vazio.")
    print(f"{label}: {100 * covered / total:.1f}% ({covered}/{total})", flush=True)
    if 100 * covered < MINIMUM * total:
        raise SystemExit(f"{label}: cobertura abaixo de {MINIMUM}%.")


def go_tests(directory):
    profile = directory / "go.cover"
    env = os.environ | {"GOTOOLCHAIN": "local", "CGO_ENABLED": "1"}
    subprocess.run(["go", "test", "-race", f"-coverprofile={profile}", "./..."],
                   cwd=ROOT / "apps", env=env, check=True)
    with profile.open() as stream:
        next(stream)  # Cabeçalho do formato de cobertura do Go.
        blocks = [tuple(map(int, line.split()[1:])) for line in stream]
    require_coverage("Go (instruções)", sum(size for size, hits in blocks if hits), sum(size for size, _ in blocks))


def shell_coverage(directory):
    scripts = {}
    for folder in ("scripts", "core"):
        for file in (ROOT / folder).rglob("*"):
            if file.is_symlink() or not file.is_file():
                continue
            with file.open("rb") as stream:
                header = stream.readline(256)
            words = header[2:].split()
            if not header.startswith(b"#!") or not words:
                continue
            command = words[0].rsplit(b"/", 1)[-1]
            if command == b"env" and len(words) > 1:
                command = words[2] if words[1] == b"-S" and len(words) > 2 else words[1]
            if command in (b"bash", b"sh"):
                scripts[file.relative_to(ROOT)] = {}

    for report in directory.glob("*/*/cobertura.xml"):
        data = ET.parse(report)
        source = Path(data.findtext("sources/source", ""))
        for entry in data.findall(".//class"):
            parts = (source / entry.attrib["filename"]).parts
            for script, lines in scripts.items():
                if parts[-len(script.parts):] == script.parts:
                    for line in entry.findall("./lines/line"):
                        index = int(line.attrib["number"])
                        lines[index] = lines.get(index, False) or int(line.attrib["hits"]) > 0
                    break

    for script, lines in sorted(scripts.items()):
        require_coverage(str(script), sum(lines.values()), len(lines))
    require_coverage("Shell (linhas)", sum(sum(lines.values()) for lines in scripts.values()),
                     sum(len(lines) for lines in scripts.values()))


def main():
    kcov = shutil.which(os.environ.get("KCOV", "kcov"))
    if kcov is None:
        raise SystemExit("Instale kcov ou defina KCOV com o caminho da ferramenta.")
    reports = ROOT / "out/coverage"
    reports.mkdir(parents=True, exist_ok=True)
    current = Path(mkdtemp(prefix="test-", dir=reports))
    shell = current / "shell"
    shell.mkdir()
    print(f"Relatórios em {current}", flush=True)
    go_tests(current)
    env = os.environ | {
        "COVERAGE_FILE": str(current / "python.cover"),
        "MELKOR_TEST_KCOV": str(Path(kcov).resolve()),
        "MELKOR_TEST_SHELL_REPORT": str(shell),
    }
    coverage = [sys.executable, "-m", "coverage"]
    subprocess.run([*coverage, "run", "--branch", "--source=scripts", "-m", "unittest", "discover", "-s", "tests"],
                   cwd=ROOT, env=env, check=True)
    subprocess.run([*coverage, "report", "--show-missing", "--precision=1"], cwd=ROOT, env=env, check=True)
    summary = current / "python.json"
    subprocess.run([*coverage, "json", "-o", str(summary)], cwd=ROOT, env=env, check=True)
    totals = json.loads(summary.read_text())["totals"]
    require_coverage("Python (instruções e ramos)", totals["covered_lines"] + totals["covered_branches"],
                     totals["num_statements"] + totals["num_branches"])
    shell_coverage(shell)


if __name__ == "__main__":
    main()
