# Stable release checklist

The 30 October 2026 readiness review passed automated checks, and the maintainer confirmed all target-host acceptance checks passed. The next intended version is **1.4.3**. It has not been versioned, tagged or published by this review.

1. **Complete:** the Wolf Forge product page was deployed through website commit `2bd4be9`. Its live page/assets, Projects link and disabled stable-release manifest were verified. Keep stable downloads disabled until publication. See [WEBSITE.md](WEBSITE.md).
2. Review and commit the existing Windows changes, new Go tests and release documents. Review the untracked `Brief1.1-1.4` separately before deciding whether it belongs in source. Keep private corpora, configuration, screenshots and build tooling out of the release source.
3. Set `VERSION` to `1.4.3`; update the installation examples and release status in README, release notes and verification report. Record any further target-host checks needed if implementation changes after acceptance.
4. Run `go test -race -cover ./...`, Linux and Windows x64 vet, and `git diff --check`. Commit the final versioned source, create `v1.4.3`, and build from that exact clean tagged revision. Do not move the existing `v1.4.2` tag or replace its published files.
5. Run `go run ./tools/package` with `dpkg-deb` and NSIS available. Then run package, CLI, lifecycle and browser smoke checks listed in [TESTING.md](TESTING.md). Test the final installers on target hosts if they differ from the accepted candidate.
6. Produce corresponding source from the same tag, for example `git archive --format=tar.gz --prefix=dayz-bank-viewer-1.4.3/ --output=dist/dayz-bank-viewer-1.4.3-source.tar.gz v1.4.3`. Add its SHA-256 to `dist/SHA256SUMS`; the package tool generates checksums for the four binary packages only. Verify all five entries with `sha256sum -c SHA256SUMS` from `dist`.
7. Publish one complete stable GitHub Release for that tag: Windows setup EXE, Windows portable ZIP, Linux DEB, Linux portable tar.gz, source tar.gz and `SHA256SUMS`. Disclose unsigned builds and preserve the read-only/data-retention guidance. Verify all six download URLs and that the release is not a draft or prerelease.
8. Promote and deploy the website manifest using the exact artifact contract in [WEBSITE.md](WEBSITE.md). Update the catalogue status. Verify the real website's two installer links and all-files link, then check that the application's manual stable update check offers 1.4.3 to older installations.

The current working tree still reports version 1.4.2. Packages rebuilt during this review are development verification artifacts, not replacements for the existing prerelease. A public release must match its own immutable source tag and published corresponding source.
