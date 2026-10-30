"""Release boundary tests; no GitHub writes or live banking data."""

import hashlib
import io
import json
from pathlib import Path
import tarfile
import tempfile
import unittest
from unittest.mock import patch

import release


class ReleaseTests(unittest.TestCase):
    def fixture(self, folder):
        version = "1.4.3"
        files = release.artifacts(version)
        for name in files.values():
            (folder / name).write_bytes(b"synthetic package")
        with tarfile.open(folder / files["source"], "w:gz") as archive:
            contents = b"1.4.3\n"
            info = tarfile.TarInfo("dayz-bank-viewer-1.4.3/VERSION")
            info.size = len(contents)
            archive.addfile(info, io.BytesIO(contents))
        (folder / "SHA256SUMS").write_text("".join(
            f"{hashlib.sha256((folder / name).read_bytes()).hexdigest()}  {name}\n"
            for name in files.values() if name != "SHA256SUMS"
        ))
        return files

    def test_rejects_unsafe_and_prerelease_tags(self):
        for tag in ["1.4.3", "v1.4.3rc1", "v1.4", "v1.4.3\n", "../../main", "--help"]:
            with self.subTest(tag=tag), self.assertRaises(ValueError):
                release.version_from_tag(tag)

    def test_complete_packages_then_tampered_binary(self):
        with tempfile.TemporaryDirectory() as temp:
            folder = Path(temp)
            files = self.fixture(folder)
            release.verify(folder, "1.4.3")
            (folder / files["windows"]).write_bytes(b"tampered")
            with self.assertRaisesRegex(ValueError, "corrupt"):
                release.verify(folder, "1.4.3")

    def test_rejects_missing_duplicate_and_traversal_checksums(self):
        with tempfile.TemporaryDirectory() as temp:
            folder = Path(temp)
            self.fixture(folder)
            checksums = folder / "SHA256SUMS"
            lines = checksums.read_text().splitlines(keepends=True)
            for changed in [lines[:-1], lines + [lines[0]], ["0" * 64 + "  ../secret\n"]]:
                checksums.write_text("".join(changed))
                with self.assertRaises(ValueError):
                    release.verify(folder, "1.4.3")

    def test_rejects_source_version_mismatch_even_with_valid_hash(self):
        with tempfile.TemporaryDirectory() as temp:
            folder = Path(temp)
            files = self.fixture(folder)
            source = folder / files["source"]
            previous = hashlib.sha256(source.read_bytes()).hexdigest()
            with tarfile.open(source, "w:gz") as archive:
                info = tarfile.TarInfo("dayz-bank-viewer-1.4.3/VERSION")
                info.size = 6
                archive.addfile(info, io.BytesIO(b"1.4.2\n"))
            checksums = folder / "SHA256SUMS"
            checksums.write_text(checksums.read_text().replace(previous, hashlib.sha256(source.read_bytes()).hexdigest()))
            with self.assertRaisesRegex(ValueError, "VERSION"):
                release.verify(folder, "1.4.3")

    def test_incomplete_or_prerelease_downloads_never_start(self):
        metadata = {"tag_name": "v1.4.3", "draft": False, "prerelease": False, "assets": []}
        with patch.object(release, "run") as run:
            with self.assertRaisesRegex(ValueError, "six"):
                release.verify_downloads("v1.4.3", metadata)
            metadata["prerelease"] = True
            with self.assertRaisesRegex(ValueError, "channel"):
                release.verify_downloads("v1.4.3", metadata)
            run.assert_not_called()

    def test_existing_public_release_cannot_be_replaced(self):
        with patch.object(release, "existing_release", return_value={"draft": False}):
            with self.assertRaisesRegex(ValueError, "already published"):
                release.require_new_release("v1.4.3")

    def test_older_release_cannot_be_published_as_latest(self):
        previous = {"draft": False, "prerelease": False, "tag_name": "v1.5.0"}
        with patch.object(release, "run", return_value=json.dumps(previous)):
            with self.assertRaisesRegex(ValueError, "Advance VERSION"):
                release.require_newer_version("v1.4.3")

    def test_failed_verification_never_dispatches(self):
        with patch.dict("os.environ", {"SITE_TOKEN": "synthetic"}), \
                patch.object(release, "release", return_value={}), \
                patch.object(release, "verify_downloads", side_effect=ValueError("bad checksum")), \
                patch.object(release, "run") as run:
            with self.assertRaisesRegex(ValueError, "checksum"):
                release.website("v1.4.3")
            run.assert_not_called()

    def test_old_handoff_never_dispatches(self):
        with patch.dict("os.environ", {"SITE_TOKEN": "synthetic"}), \
                patch.object(release, "release", return_value={}), \
                patch.object(release, "verify_downloads"), \
                patch.object(release, "api", return_value={"tag_name": "v1.5.0"}), \
                patch.object(release, "run") as run:
            with self.assertRaisesRegex(ValueError, "latest stable"):
                release.website("v1.4.3")
            run.assert_not_called()

    def test_dispatch_uses_exact_website_contract(self):
        payloads = []

        def capture(*args, **kwargs):
            payloads.append(json.loads(Path(args[args.index("--input") + 1]).read_text()))
            self.assertEqual(kwargs["env"]["GH_TOKEN"], "synthetic-site-token")
            return ""

        with patch.dict("os.environ", {"SITE_TOKEN": "synthetic-site-token"}), \
                patch.object(release, "release", return_value={}), \
                patch.object(release, "verify_downloads"), \
                patch.object(release, "api", return_value={"tag_name": "v1.4.3"}), \
                patch.object(release, "run", side_effect=capture):
            release.website("v1.4.3")
        self.assertEqual(payloads, [{
            "event_type": "desktop_release_published",
            "client_payload": {
                "project": "dayz_bank_viewer", "version": "1.4.3", "tag": "v1.4.3",
                "prerelease": False, "artifacts": release.artifacts("1.4.3"),
            },
        }])


if __name__ == "__main__":
    unittest.main()
