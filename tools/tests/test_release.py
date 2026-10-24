import importlib.util
import json
from pathlib import Path
import plistlib
import tempfile
import unittest
from unittest.mock import patch
import zipfile

import yaml

ROOT = Path(__file__).resolve().parents[2]
spec = importlib.util.spec_from_file_location("release", ROOT / "tools/release.py")
release = importlib.util.module_from_spec(spec)
spec.loader.exec_module(release)


class ReleaseTests(unittest.TestCase):
    def test_supported_targets_have_unique_versioned_names(self):
        names = [release.artifact_name(target, fmt) for target, (_, _, formats) in release.TARGETS.items() for fmt in formats]
        self.assertEqual(len(names), len(set(names)))
        self.assertTrue(all("v" + release.metadata()["version"] in name for name in names))

    def test_invalid_formats_fail_before_build(self):
        with self.assertRaises(ValueError):
            release.selected_formats("windows-amd64", "dmg")
        with self.assertRaises(ValueError):
            release.selected_formats("linux-amd64", "deb,deb")

    def test_cross_compilation_is_rejected(self):
        with patch.object(release, "host_target", return_value="windows-amd64"):
            with self.assertRaisesRegex(RuntimeError, "Native builds required"):
                release.doctor("darwin-arm64", ("zip",))

    def test_sync_versions_and_identity_are_consistent_and_idempotent(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            files = ["build/release.json", "build/config.yml", "frontend/package.json", "frontend/package-lock.json", "build/windows/wails.exe.manifest", "build/darwin/Info.plist", "build/darwin/Info.dev.plist"]
            for file in files:
                (root / file).parent.mkdir(parents=True, exist_ok=True)
                (root / file).write_bytes((ROOT / file).read_bytes())
            metadata = json.loads((root / "build/release.json").read_text(encoding="utf-8"))
            metadata["version"] = "1.2.3-beta.1"
            (root / "build/release.json").write_text(json.dumps(metadata), encoding="utf-8")
            with patch.object(release, "ROOT", root):
                release.sync_metadata()
                before = {str(p): p.read_bytes() for p in root.rglob("*") if p.is_file()}
                release.sync_metadata()
                self.assertEqual(before, {str(p): p.read_bytes() for p in root.rglob("*") if p.is_file()})
            self.assertEqual((root / "backend/pkg/version/version").read_text().strip(), "v1.2.3-beta.1")
            self.assertEqual(plistlib.loads((root / "build/darwin/Info.plist").read_bytes())["CFBundleShortVersionString"], "1.2.3")
            self.assertEqual(json.loads((root / "build/windows/info.json").read_text(encoding="utf-8"))["fixed"]["file_version"], "1.2.3")
            self.assertEqual(yaml.safe_load((root / "build/config.yml").read_text(encoding="utf-8"))["info"]["version"], "1.2.3-beta.1")

    def test_portable_package_contains_no_user_data(self):
        with tempfile.TemporaryDirectory() as directory:
            work = Path(directory)
            binary = work / "fire-poker.exe"
            binary.write_bytes(b"test-executable")
            (work / "data.db").write_bytes(b"private")
            output = work / "out"
            output.mkdir()
            release.package_windows(work, binary, output, "windows-amd64", ("portable",))
            archive = next(output.glob("*.zip"))
            with zipfile.ZipFile(archive) as package:
                self.assertEqual(set(package.namelist()), {"fire-poker.exe", "start-fire-poker.cmd", "README.txt"})
                self.assertIn(b"FIRE_POKER_DATA_DIR", package.read("start-fire-poker.cmd"))
            release.checksums(output)
            checksum = (output / "SHA256SUMS.txt").read_text()
            release.checksums(output)
            self.assertEqual(checksum, (output / "SHA256SUMS.txt").read_text())
            release.verify(output)
            archive.write_bytes(b"corrupted")
            with self.assertRaisesRegex(ValueError, "Checksum mismatch"):
                release.verify(output)

    def test_installer_keeps_user_data_and_uses_generated_identity(self):
        installer = (ROOT / "build/windows/nsis/project.nsi").read_text(encoding="utf-8")
        self.assertIn('!include "brand.nsh"', installer)
        self.assertNotIn('RMDir /r $INSTDIR', installer)
        self.assertNotIn('RMDir /r "$AppData', installer)

    def test_failed_package_does_not_publish_partial_outputs(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "build").mkdir()
            (root / "build/release.json").write_bytes((ROOT / "build/release.json").read_bytes())
            with patch.object(release, "ROOT", root), patch.object(release, "doctor"), patch.object(release, "sync_metadata"), patch.object(release, "run"), patch.object(release, "package_windows", side_effect=RuntimeError("installer failed")):
                with self.assertRaisesRegex(RuntimeError, "installer failed"):
                    release.build("windows-amd64", ("portable", "installer"), False)
            self.assertFalse((root / "release").exists())
            self.assertEqual(list((root / ".release-work").iterdir()), [])


if __name__ == "__main__":
    unittest.main()
