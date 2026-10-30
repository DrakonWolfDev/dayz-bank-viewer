# GitHub Actions builds and website updates

## Dev build

**Dev build** runs for pushes to `main`, pull requests and manual runs from the Actions tab. It runs native Windows tests, Linux race/coverage tests, Linux/Windows vet, release-tool tests, package inspection, portable CLI/lifecycle checks and the synthetic browser smoke.

It builds the Windows x64 setup EXE and portable ZIP, Linux x64 DEB and portable tarball, the checked-out commit's source archive and SHA-256 checksums. Download `dayz-bank-viewer-packages` from the completed run; these internal verification artifacts expire after 14 days. They retain the source's VERSION and do not create a public release or change the website. They must not replace an existing version's published files.

## Release build

1. Complete target-host acceptance in [TESTING.md](TESTING.md). Automated Windows tests do not replace manual desktop, installer and reboot acceptance.
2. Advance `VERSION` beyond existing stable releases, update README/release notes and commit/push the final source. The next intended version is `1.4.3`.
3. Create and push its matching immutable tag, for example `git tag -a v1.4.3 -m 'Release 1.4.3'` then `git push origin v1.4.3`. This starts **Release build**. Alternatively, run **Release build** manually with an existing matching tag.
4. The workflow checks that the tag, VERSION and checkout commit agree, the repository is public and the website secret is configured. It then runs the complete Dev build checks against that exact commit.
5. Only after all jobs pass, it assembles a draft GitHub Release, uploads all six files, downloads them again and checks every hash. It publishes the complete stable release and marks it latest. Builds remain unsigned; release notes disclose this.
6. **Update website downloads** verifies the published stable release's exact inventory and downloaded checksums, then sends the `desktop_release_published` event to `DrakonWolfDev/wolf-forge-static`. That repository validates the payload and download URLs before committing `releases.json` through its existing deployment path. Check the website's **Update desktop release** run and live product-page links to confirm deployment; dispatch acceptance alone is not a deployment confirmation.

The six published files are the four binary packages, `dayz-bank-viewer-VERSION-source.tar.gz` and `SHA256SUMS`. The source archive comes from `git archive HEAD`; checksums cover all five downloadable archives/installers. Private corpora, configuration and ignored files are excluded.

## Website credential

Set the repository Actions secret **WOLF_FORGE_SITE_TOKEN** to a fine-grained GitHub token scoped to **DrakonWolfDev/wolf-forge-static** with **Contents: read and write**. GitHub requires that permission for repository dispatch events. Keep the token out of Git, logs and chat. The built-in `GITHUB_TOKEN` handles releases in this repository; only the final website dispatch uses the website token.

The secret must be maintained or rotated before its expiry. Workflow logs report a missing secret or rejected request without printing the credential.

Pushes to `main` also test the credential with a `desktop_release_connection_check` dispatch. The website receiver listens only for `desktop_release_published`, so this tests the real dispatch permission without changing the manifest or deploying a release. This job does not run for pull requests or tagged release builds.

## Recovery

- A failed build publishes nothing. Fix the source, rerun verification and use a new version/tag if the tagged source changes.
- A failed draft upload leaves an unpublished draft. Rerun the failed jobs to finish and verify it. Public release assets are never overwritten by this automation.
- If publication succeeded but the website handoff failed, rerun the failed website job, or manually run **Update website downloads** with the published tag. This verifies and sends the existing release without rebuilding it. Older releases cannot replace the latest stable website links.
- If the website receiver fails after dispatch succeeds, inspect its run in the website repository, correct the cause and rerun it. Existing live downloads remain until a verified manifest is deployed.

Workflow definitions use pinned official GitHub Action commits. Go uses the current stable toolchain; Playwright is pinned to `1.64.1` and installed outside the source tree. Neither Node nor browser test tools are shipped to users. This setup alone does not bump VERSION, create a version tag or publish 1.4.3.
