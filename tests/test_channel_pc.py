from base64 import b64decode
from pathlib import Path
import json
import runpy
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch


SCRIPT = Path(__file__).resolve().parents[1] / "scripts/channel-pc.py"
sign = runpy.run_path(str(SCRIPT))["sign"]


class ChannelPCTest(unittest.TestCase):
    def test_assina_hash_do_bundle_verificado_e_revisao_do_payload(self):
        with tempfile.TemporaryDirectory() as scratch:
            build = Path(scratch)
            original = build / "update.raucb"
            original.write_bytes(b"bundle")
            key = build / "channel.key"
            anchor = build / "rauc.pem"
            manifest = "[update]\ncompatible=melkor-pc-limine-x86_64\n"
            graphical = True

            def extract(args, **_):
                self.assertEqual(args[1], f"--keyring={anchor}")
                self.assertEqual(args[3].read_bytes(), original.read_bytes())
                args[4].mkdir()
                (args[4] / "manifest.raucm").write_text(manifest)

            def command(args):
                if args[0].name == "unsquashfs":
                    if args[-1] == "etc/dinit.d/graphical-login":
                        if not graphical:
                            raise subprocess.CalledProcessError(1, args)
                        return b"type = process\n"
                    self.assertEqual(args[-1], "etc/melkor-update/revision")
                    return b"3\n"
                self.assertIn("-rawin", args)
                self.assertEqual(json.loads(args[-1].read_bytes())["revision"], 3)
                return bytes(64)

            with patch.object(subprocess, "run", side_effect=extract), \
                 patch.object(subprocess, "check_output", side_effect=command):
                envelope = json.loads(sign(build, original, anchor, key, "https://example.invalid/update.raucb", 1))
                actual = json.loads(b64decode(envelope["payload"]))
                self.assertEqual(actual["target"], "melkor-pc-limine-x86_64")
                self.assertEqual(actual["sha256"], "1e6ed65d77d6364eeaed5a745ba5c4985ae2b700dd85d7cf7f027bdf294a33fc")
                self.assertEqual(len(b64decode(envelope["signature"])), 64)
                self.assertNotIn("profile", actual)
                output = build / "channel.json"
                argv = [str(SCRIPT), str(build), str(original), "--keyring", str(anchor),
                        "--key", str(key), "--url", actual["url"], "--output", str(output)]
                with patch.object(sys, "argv", argv):
                    runpy.run_path(str(SCRIPT), run_name="__main__")
                self.assertEqual(json.loads(output.read_text())["signature"], envelope["signature"])
                with patch.object(sys, "argv", [*argv, "--desktop"]):
                    runpy.run_path(str(SCRIPT), run_name="__main__")
                desktop = json.loads(b64decode(json.loads(output.read_text())["payload"]))
                self.assertEqual(desktop["profile"], "desktop")
                graphical = False
                with self.assertRaises(subprocess.CalledProcessError):
                    sign(build, original, anchor, key, actual["url"], 1, desktop=True)
                manifest = "[update]\ncompatible=melkor-m1-x86_64\n"
                with self.assertRaisesRegex(ValueError, "bundle do PC"):
                    sign(build, original, anchor, key, "https://example.invalid/update.raucb", 1)

    def test_recusa_http_credenciais_e_validade_invalida(self):
        for source in ("http://example.invalid/bundle", "https://u@example.invalid/bundle"):
            with self.assertRaisesRegex(ValueError, "HTTPS"):
                sign(None, None, None, None, source, 1)
        with self.assertRaisesRegex(ValueError, "validade"):
            sign(None, None, None, None, "https://example.invalid/bundle", 0)
