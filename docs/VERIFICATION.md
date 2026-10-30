# V1.4.3 release verification — 30 October 2026

`VERSION` is **1.4.3**. The stable release is built from the matching immutable `v1.4.3` tag through [Release build](AUTOMATION.md), which reruns native Windows tests, Linux race/coverage tests, Linux/Windows vet, packaging and synthetic runtime/browser checks before publication. All four packages, corresponding source and five SHA-256 checksums are verified before the complete release becomes public. Website downloads are promoted only after publication and verification of the public release.

The preceding automation commit `a0aad9f` passed [GitHub Actions](https://github.com/DrakonWolfDev/dayz-bank-viewer/actions/runs/41112700118), including native Windows tests and the website dispatch permission check. The versioned release changes only VERSION and documentation. Target-host acceptance remains the maintainer's 30 October confirmation recorded below. Builds remain unsigned; no private banking data is used in automated checks.

## Publication record

The immutable `v1.4.3` tag points to `05465ec727fa7a80c001ea33464cf26e228eb6ff`. Its [release run](https://github.com/DrakonWolfDev/dayz-bank-viewer/actions/runs/41114200052) passed preflight, native Windows tests and the complete Linux packaging/smoke job. Publication stopped after upload because the draft was queried through the published-tag endpoint, which returned HTTP 404. No partial public release was created.

The six draft files were downloaded and compared byte for byte with the successful Actions artifact. All five SHA-256 checksums passed, and the source archive matched a fresh `git archive` of `v1.4.3` exactly. The existing draft was then published as the [1.4.3 stable release](https://github.com/DrakonWolfDev/dayz-bank-viewer/releases/tag/v1.4.3), without replacing any assets or moving the tag. The separate website-update action was started for that published release. A subsequent automation-only fix uses the draft-capable release listing for verification and adds a regression test; it does not change the tagged application or released files.

The readiness review and older reports below are historical evidence. Statements about the then-unversioned working tree and disabled website downloads describe that earlier checkpoint.

---

# Release readiness review — 30 October 2026

## Current result

Automated checks pass for the current working tree. The maintainer confirmed on 30 October 2026 that **all target-host checks passed**, including the latest Windows control-window changes and installer/service lifecycle. This is maintainer-reported acceptance; this workspace did not independently run those native-host checks. Earlier pending statements below are historical and are superseded by this confirmation for the current candidate.

The current source includes changes made after the published `v1.4.2` tag (`6f9897c`). `VERSION` remains `1.4.2`; packages rebuilt locally during this review are development verification artifacts and must not replace the published 1.4.2 assets. Prepare the next patch release as **1.4.3**, commit the final source, then build all distributed artifacts from its matching tag. Do not describe the current working tree or these local packages as the published 1.4.2 source.

## Rechecked in this review

- `go test -race -cover ./...`: pass; application statement coverage 60.9%. The private corpus checks remain opt-in; this run used synthetic fixtures.
- `go vet ./...` and Windows x64 `go vet ./...`: pass.
- `go run ./tools/package`: Linux and Windows x64 portable archives, DEB and NSIS setup EXE build successfully. The extracted NSIS toolchain is selected with `MAKENSIS` and `NSISDIR`.
- `python3 tools/package-smoke.py`: archive allowlists, licence notices, Debian scripts, service identity, executable modes, installer PE header and all four SHA-256 checksums pass.
- `python3 tools/cli-smoke.py` and `python3 tools/lifecycle-smoke.py`: pass against the rebuilt packages. Native desktop tools are stubbed by the lifecycle smoke.
- Chrome / Playwright `tools/browser-smoke.cjs`: pass for onboarding, settings, precision, mixed banking formats, ADM names, player/shared views, discovery, scan failure/recovery and responsive layouts. No unexpected external requests or page errors.
- `git diff --check`: pass. Existing uncommitted implementation changes and new tests remain in the working tree.

The sandbox required a writable `GOCACHE` and permission to use localhost sockets for runtime/browser checks. No live banking data was needed.

## Website and final release steps

The existing Wolf Forge static site is the chosen destination. The product page, catalogue entry and synthetic dashboard image were deployed through website commit `2bd4be9`; the live page/assets and manifest were verified. Stable download availability remains false until a complete stable release is published. See [WEBSITE.md](WEBSITE.md) for paths, verification and promotion. See [RELEASE-CHECKLIST.md](RELEASE-CHECKLIST.md) for the remaining release order.

Builds remain unsigned; disclose that in the release and website. The earlier reports are retained below as dated evidence.

---

# V1.4.2 prerelease verification — 30 October 2026

This build is prepared for home-server testing followed by production-server acceptance. Linux and Windows x64 binaries and installers are built from the matching v1.4.2 source tag. Automated coverage includes server/configuration discovery, manual/detected setup, independent player/clan views, bounded ADM history, sorting and read-only source handling.

The release checks cover the Go race suite, Linux/Windows vet, browser regression, extracted Linux package operation, archive allowlists, installer metadata and SHA-256 checksums. Native Windows runtime and real booted service lifecycle acceptance remain pending. The prerelease is not promoted to website downloads or the stable update checker. See [TESTING.md](TESTING.md).

The report below records the earlier 1.4.1 development verification and is retained as historical context.

---

# V1.4.1 verification — 30 October 2026

Implemented against `Brief1.1-1.4`. Version boundaries are recorded in [RELEASES.md](RELEASES.md). These are development packages, with no GitHub publication or website deployment.

## Desktop mode follow-up

The signed-in user is now the default for installed desktop launches. Configuration routing, explicit mode selection, read-only folder checks and failure rollback are covered by tests. The process smoke asserts that the runtime identity matches the signed-in account and stubs `pkexec` to fail if desktop startup unexpectedly requests elevation. Source fixtures and private source permissions remain unchanged.

Unattended service setup is tested with simulated native-service operations for successful ordering (check access before enabling boot startup) and failure rollback (restore existing settings, preserve a backup, do not enable startup on denied access). These are behavioural tests of the transaction; they do not establish that PolicyKit, UAC or real booted service transitions have been exercised.

The Linux control window has been checked on a virtual display. Real install/upgrade migration, native service access checks and Windows dialogs still require the target-host steps below.

## Verified in this environment

- Go tests pass with the race detector. Coverage includes the original banking tests and new update/control checks. Desktop/SCM wrappers are platform acceptance work, so overall statement coverage is not a measure of Windows verification.
- `go vet ./...` passes for Linux and Windows x64. Both binaries cross-compile without CGO. JavaScript and Debian maintainer-script syntax checks pass.
- Update tests cover numeric version comparison, older/current/newer releases, drafts, prereleases, malformed tags/JSON, missing releases, rate limits, oversized payloads, timeout and redirect refusal. Requests use synthetic local HTTP servers, not the live GitHub release state.
- Runtime tests cover socket fallback and continued ownership, duplicate lock refusal and release, authenticated stop, public discovery without credentials, stale state and refusal to send credentials to an external control address.
- Process lifecycle smoke passes with temporary configurations: occupied preferred port, duplicate refusal, reconnect, restart, stop, startup readiness, simultaneous desktop launches producing one browser open, subsequent reconnect producing no extra tab, and retained configuration. Browser launch is stubbed for this test.
- Chrome smoke passes: branding and footer link, installed version, update result/link states with intercepted responses, onboarding, two servers, exact large balances, search/sort/pagination, player details, partial/failed scan recovery, disabling/renaming/removing configurations, unchanged synthetic source files, and desktop/tablet/mobile layouts. No unexpected external requests or page errors.
- Native Linux Zenity control opens on an isolated Xvfb display with version, status, port, URL and action list, and closes with Escape. The portable service then stops cleanly. This is an actual widget smoke test; boot services, PolicyKit approval and every GUI action are not covered by it. Screenshot: ignored `artifacts/linux-control.png`.
- Extracted Linux release smoke passes: assets, version, configuration persistence/location from another working directory, duplicate refusal, graceful shutdown and refusal to overwrite a player file passed as configuration.
- NSIS 3.09 builds the Windows setup EXE without compiler warnings. `dpkg-deb` builds the Linux package. Package inspection checks archive allowlists, licences/notices, Debian script syntax/executable modes, restricted systemd identity/recovery settings, executable modes, the Windows PE installer and all four SHA-256 checksums.

## Not yet verified on target hosts

No Windows runtime or disposable booted systemd VM is available in this workspace. The following require the isolated host checklist in [TESTING.md](TESTING.md):

- Windows Forms, UAC elevation, SCM startup/shutdown/recovery and Windows filesystem permissions at runtime.
- Actual DEB/EXE installation, upgrade, boot without login, startup enable/disable, removal/purge and preservation of user data on both platforms.
- Real desktop browser integration, accessibility/scaling and signed distribution. Builds are unsigned.
- Discovery of a published 1.4.1 release; no such publication was performed here.

Source banking implementations were preserved. No future adapter, periodic scan feature, mini API, remote banking connection or Discord integration was added. Existing configured daily scans remain operational.
