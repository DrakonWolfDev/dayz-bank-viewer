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
