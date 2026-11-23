from hashlib import sha256
from pathlib import Path
import runpy
import subprocess
from tempfile import TemporaryDirectory
import unittest
from unittest.mock import patch


SCRIPT = Path(__file__).resolve().parents[1] / "scripts/pc-desktop-layer.py"
LAYER = runpy.run_path(str(SCRIPT))


class DesktopLayerTest(unittest.TestCase):
    def test_verifica_cache_e_download_antes_de_publicar_pacote(self):
        payload = b"pacote assinado"
        digest = sha256(payload).hexdigest()
        line = f"{digest} main pacote.apk"
        with TemporaryDirectory() as scratch:
            cache = Path(scratch)
            dest = cache / "pacote.apk"

            def download(args, **_):
                self.assertIn("--proto-redir", args)
                self.assertTrue(args[-1].endswith("/main/x86_64/pacote.apk"))
                args[-2].write_bytes(payload)

            with patch.object(subprocess, "run", side_effect=download) as run:
                self.assertEqual(LAYER["fetch"](cache, line), dest)
                self.assertEqual(dest.read_bytes(), payload)
                self.assertEqual(LAYER["fetch"](cache, line), dest)
                self.assertEqual(run.call_count, 1)
                dest.write_bytes(b"corrompido")
                with self.assertRaisesRegex(ValueError, "cache"):
                    LAYER["fetch"](cache, line)
                dest.unlink()
                with self.assertRaisesRegex(ValueError, "download"):
                    LAYER["fetch"](cache, f"{'0' * 64} main pacote.apk")
                self.assertFalse(dest.exists())

    def test_monta_apks_fixados_sem_rede_e_entrega_a_camada(self):
        payload = b"pacote assinado"
        with TemporaryDirectory() as scratch:
            work = Path(scratch)
            cache, output = work / "cache", work / "result/layer.tar.gz"
            cache.mkdir()
            (cache / "pacote.apk").write_bytes(payload)
            lock = work / "packages.lock"
            lock.write_text(f"{sha256(payload).hexdigest()} main pacote.apk\n")

            def install(args, **_):
                self.assertIn("--network=none", args)
                self.assertIn("--no-network", args[-1])
                stage = Path(next(arg for arg in args if arg.endswith(":/work")).removesuffix(":/work"))
                self.assertEqual((stage / "packages/pacote.apk").read_bytes(), payload)
                greeter_patch = stage / "packages/noctalia-greeter-protocol-headers.patch"
                if "build-noctalia-greeter" in args[-1]:
                    self.assertIn("export CC_LD=mold CXX_LD=mold", args[-1])
                    self.assertEqual(greeter_patch.read_bytes(), LAYER["GREETER_PATCH"].read_bytes())
                else:
                    self.assertFalse(greeter_patch.exists())
                (stage / "layer.tar.gz").write_bytes(b"layer")

            assemble = LAYER["assemble"]
            with patch.dict(assemble.__globals__, {"LOCK": lock, "SDK_LOCK": lock}), \
                    patch.object(subprocess, "run", side_effect=install):
                for profile in ("alpine", "session"):
                    assemble(cache, output, profile)
                    self.assertEqual(output.read_bytes(), b"layer")
