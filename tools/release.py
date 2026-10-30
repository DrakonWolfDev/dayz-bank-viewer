"""Assemble exact-source releases and hand verified public downloads to Wolf Forge."""

import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import tarfile
import tempfile


REPOSITORY = "DrakonWolfDev/dayz-bank-viewer"
WEBSITE = "DrakonWolfDev/wolf-forge-static"


def run(*args, **kwargs):
    return subprocess.check_output(args, text=True, **kwargs).strip()


def version_from_tag(tag):
    if not re.fullmatch(r"v[0-9]+\.[0-9]+\.[0-9]+", tag):
        raise ValueError("A stable vMAJOR.MINOR.PATCH tag is required")
    return tag[1:]


def artifacts(version):
    prefix = f"dayz-bank-viewer-{version}"
    return {
        "windows": f"{prefix}-windows-amd64-setup.exe",
        "linux_deb": f"{prefix}-linux-amd64.deb",
        "windows_portable": f"{prefix}-windows-amd64.zip",
        "linux_portable": f"{prefix}-linux-amd64.tar.gz",
        "source": f"{prefix}-source.tar.gz",
        "checksums": "SHA256SUMS",
    }


def validate_tag(tag):
    version = version_from_tag(tag)
    if Path("VERSION").read_text().strip() != version:
        raise ValueError("Tag must match VERSION; advance VERSION before tagging a release")
    if run("git", "rev-parse", f"refs/tags/{tag}^{{commit}}") != run("git", "rev-parse", "HEAD"):
        raise ValueError("Release must be built from the exact tagged commit")
    subprocess.run(["git", "diff", "--exit-code", "HEAD"], check=True)
    return version


def verify(folder, version):
    expected = set(artifacts(version).values()) - {"SHA256SUMS"}
    found = set()
    for line in (folder / "SHA256SUMS").read_text().splitlines():
        match = re.fullmatch(r"([0-9a-f]{64})  (.+)", line)
        if not match:
            raise ValueError("Malformed checksum entry")
        digest, name = match.groups()
        if name not in expected or name in found:
            raise ValueError(f"Unexpected or duplicate checksum: {name}")
        data = (folder / name).read_bytes()
        if not data or hashlib.sha256(data).hexdigest() != digest:
            raise ValueError(f"Empty or corrupt release file: {name}")
        found.add(name)
    if found != expected:
        raise ValueError("Release requires four binary packages and matching source")
    with tarfile.open(folder / artifacts(version)["source"]) as archive:
        member = archive.extractfile(f"dayz-bank-viewer-{version}/VERSION")
        if member is None or member.read().decode().strip() != version:
            raise ValueError("Source archive VERSION does not match release")


def prepare(folder):
    version = Path("VERSION").read_text().strip()
    version_from_tag(f"v{version}")
    files = artifacts(version)
    subprocess.run([
        "git", "archive", "--format=tar.gz", f"--prefix=dayz-bank-viewer-{version}/",
        "-o", str(folder / files["source"]), "HEAD",
    ], check=True)
    sums = []
    for name in files.values():
        if name != "SHA256SUMS":
            sums.append(f"{hashlib.sha256((folder / name).read_bytes()).hexdigest()}  {name}\n")
    (folder / "SHA256SUMS").write_text("".join(sums))
    verify(folder, version)
    print("Verified all four packages, exact-commit source and five SHA-256 checksums")


def api(path):
    return json.loads(run("gh", "api", path))


def release(tag):
    return api(f"repos/{REPOSITORY}/releases/tags/{tag}")


def existing_release(tag):
    # Listing distinguishes a genuinely absent release from an API/authentication failure.
    return next((item for item in published_releases() if item["tag_name"] == tag), None)


def published_releases():
    lines = run("gh", "api", "--paginate", f"repos/{REPOSITORY}/releases", "--jq", ".[] | @json")
    return [json.loads(line) for line in lines.splitlines()]


def require_new_release(tag):
    existing = existing_release(tag)
    if existing and not existing["draft"]:
        raise ValueError("Release already published; use Update website downloads to retry its handoff")


def require_newer_version(tag):
    candidate = tuple(map(int, version_from_tag(tag).split(".")))
    for item in published_releases():
        if item["draft"] or item["prerelease"]:
            continue
        previous = tuple(map(int, version_from_tag(item["tag_name"]).split(".")))
        if candidate <= previous:
            raise ValueError("Advance VERSION beyond every published stable release")


def preflight(tag):
    validate_tag(tag)
    if not os.environ.get("SITE_TOKEN"):
        raise ValueError("Configure WOLF_FORGE_SITE_TOKEN before publishing a release")
    if api(f"repos/{REPOSITORY}")["private"]:
        raise ValueError("Website downloads require a public release repository")
    require_new_release(tag)
    require_newer_version(tag)
    # Check credential reachability without printing or persisting it.
    run("gh", "api", f"repos/{WEBSITE}", env=dict(os.environ, GH_TOKEN=os.environ["SITE_TOKEN"]))


def verify_downloads(tag, metadata, *, draft=False):
    version = version_from_tag(tag)
    if metadata["tag_name"] != tag or metadata["draft"] != draft or metadata["prerelease"]:
        raise ValueError("Release channel/state does not match the requested stable release")
    names = [asset["name"] for asset in metadata["assets"]]
    if len(names) != 6 or set(names) != set(artifacts(version).values()):
        raise ValueError("Published release must contain exactly the six declared assets")
    with tempfile.TemporaryDirectory(prefix="dayz-release-download-") as temp:
        run("gh", "release", "download", tag, "--repo", REPOSITORY, "--dir", temp)
        verify(Path(temp), version)
        return (Path(temp) / "SHA256SUMS").read_bytes()


def publish(tag, folder):
    version = validate_tag(tag)
    require_new_release(tag)
    require_newer_version(tag)
    verify(folder, version)
    if existing_release(tag) is None:
        with tempfile.TemporaryDirectory() as temp:
            notes = Path(temp) / "notes.md"
            notes.write_text(
                "Windows x64 setup/portable and Linux x64 DEB/portable packages. "
                "Builds are unsigned. Verify downloads with SHA256SUMS.\n\n"
                "The matching source archive is included. Banking files are read-only; "
                "installation and removal preserve banking source data. "
                "See README and docs/TESTING.md in the source for setup and platform checks.\n"
            )
            run("gh", "release", "create", tag, "--repo", REPOSITORY, "--verify-tag", "--draft",
                "--title", f"DayZ Bank Viewer {version}", "--generate-notes", "--notes-file", str(notes))
    run("gh", "release", "upload", tag, "--repo", REPOSITORY, "--clobber",
        *(str(folder / name) for name in artifacts(version).values()))
    if verify_downloads(tag, release(tag), draft=True) != (folder / "SHA256SUMS").read_bytes():
        raise ValueError("Uploaded checksums differ from the built packages")
    run("gh", "release", "edit", tag, "--repo", REPOSITORY, "--draft=false", "--latest")
    print(f"Published https://github.com/{REPOSITORY}/releases/tag/{tag}")


def website(tag):
    version = version_from_tag(tag)
    if not os.environ.get("SITE_TOKEN"):
        raise ValueError("Configure WOLF_FORGE_SITE_TOKEN before notifying the website")
    metadata = release(tag)
    verify_downloads(tag, metadata)
    # A retry of an older handoff must never move website downloads backwards.
    latest = api(f"repos/{REPOSITORY}/releases/latest")
    if latest["tag_name"] != tag:
        raise ValueError("Only the latest stable release can be promoted on the website")
    payload = {
        "event_type": "desktop_release_published",
        "client_payload": {
            "project": "dayz_bank_viewer", "version": version, "tag": tag,
            "prerelease": False, "artifacts": artifacts(version),
        },
    }
    with tempfile.TemporaryDirectory() as temp:
        path = Path(temp) / "website-release.json"
        path.write_text(json.dumps(payload))
        run("gh", "api", "--method", "POST", f"repos/{WEBSITE}/dispatches", "--input", str(path),
            env=dict(os.environ, GH_TOKEN=os.environ["SITE_TOKEN"]))
    print(f"Website update requested: https://github.com/{WEBSITE}/actions")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=["prepare", "preflight", "publish", "website"])
    parser.add_argument("--tag")
    parser.add_argument("--dist", type=Path, default=Path("dist"))
    args = parser.parse_args()
    if args.command == "prepare":
        prepare(args.dist)
    elif not args.tag:
        parser.error("--tag is required")
    elif args.command == "preflight":
        preflight(args.tag)
    elif args.command == "publish":
        publish(args.tag, args.dist)
    else:
        website(args.tag)


if __name__ == "__main__":
    main()
