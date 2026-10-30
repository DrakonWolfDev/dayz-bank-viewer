"""Exercise an extracted Linux release with synthetic configuration only."""
import json
import pathlib
import re
import subprocess
import tarfile
import tempfile
import urllib.request


def main():
    version = pathlib.Path("VERSION").read_text().strip()
    archive = pathlib.Path(f"dist/dayz-bank-viewer-{version}-linux-amd64.tar.gz")
    with tempfile.TemporaryDirectory(prefix="dayz-cli-check-") as temp:
        root = pathlib.Path(temp)
        with tarfile.open(archive) as package:
            package.extractall(root, filter="data")
        folder = next(p for p in root.iterdir() if p.is_dir())
        binary = folder / "dayz-bank-viewer"
        elsewhere = root / "different working directory"
        elsewhere.mkdir()
        assert subprocess.check_output([binary, "-version"], text=True).strip() == "DayZ Bank Viewer " + version
        process = subprocess.Popen(
            [binary, "-listen", "127.0.0.1:0"], cwd=elsewhere,
            stdout=subprocess.DEVNULL, stderr=subprocess.PIPE, text=True,
        )
        try:
            line = process.stderr.readline()
            match = re.search(r"http://127\.0\.0\.1:\d+", line)
            assert match, line
            url = match.group()
            with urllib.request.urlopen(url, timeout=5) as response:
                assert b"DayZ Bank Viewer" in response.read()
            config = {"listen": "127.0.0.1:20784", "servers": [], "refresh_times": ["06:00"]}
            request = urllib.request.Request(
                url + "/api/config", data=json.dumps(config).encode(), method="PUT",
                headers={"Content-Type": "application/json"},
            )
            with urllib.request.urlopen(request, timeout=5) as response:
                assert json.load(response) == config
            assert json.loads((folder / "config.json").read_text()) == config
            assert not (elsewhere / "config.json").exists()
            conflict = subprocess.run(
                [binary, "-listen", url.removeprefix("http://")], cwd=elsewhere,
                capture_output=True, text=True, timeout=5,
            )
            assert conflict.returncode == 1
            assert "already running" in conflict.stderr
        finally:
            process.terminate()
            process.wait(timeout=5)
            process.stderr.close()
        assert process.returncode == 0
        # Accidentally pointing -config at an account file must not overwrite it.
        account = root / "synthetic-account.json"
        contents = '{"m_Steam64ID":"76561198000000001","m_PlayerName":"Synthetic","m_OwnedCurrency":100}'
        account.write_text(contents)
        rejected = subprocess.run([binary, "-config", account], capture_output=True, text=True, timeout=5)
        assert rejected.returncode == 1 and "invalid configuration" in rejected.stderr
        assert account.read_text() == contents
    print("Extracted release checks passed: standalone assets, version, configuration location/persistence, port conflict, clean shutdown, refusing a player file as configuration.")


if __name__ == "__main__":
    main()
