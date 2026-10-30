# v1.0.0 verification — 30 October 2026

Implemented against the supplied `Brief`, which ends at searching by player name or Steam64 ID.

## Passed in this development environment

- `go test -race -cover ./...`: application tests passed with the race detector; root package statement coverage was 79.1%. The release utility is exercised by archive and extracted-binary checks rather than unit coverage.
- `go vet ./...`: no findings.
- JavaScript syntax checks for the dashboard and browser smoke tool.
- Opt-in private corpus scan: 1,050 accounts read without skips. An independent calculation matched the summary; every source SHA-256 hash and the directory entry count remained unchanged. No account identities, balances or contents were added to source fixtures or screenshots.
- Chrome smoke checks: empty-state onboarding, adding and persisting two server configurations, exact large balances, unique-player counts, search, sorting, pagination, player details, escaped player text, partial scans, complete failures with retained results, recovery, disabled servers, renaming and removing configurations without deleting source files.
- Desktop (1440 px), tablet (820 px), and mobile (390 px) browser checks. Screenshots containing synthetic data are under ignored `artifacts/`. Large totals remain readable. The browser recorded no page errors or external requests.
- Extracted Linux archive smoke checks: standalone embedded assets, version output, settings persistence beside the executable despite a different working directory, understandable port-conflict errors, graceful shutdown, and refusal to use a player JSON file as configuration.
- Linux x64 ELF and Windows x64 PE builds, both with CGO disabled. Both release archive SHA-256 hashes verified. Archive members checked against the explicit five-file allowlist: executable, README, VERSION, example config and acceptance checklist.
- Git ignore patterns checked in an isolated temporary repository for corpus records, local configuration, screenshots and distribution files. The workspace's provided `.git` directory is an empty read-only placeholder, not an initialised repository. Nothing was committed, pushed or published.

## Platform limitation

Windows was cross-compiled successfully. There is no Windows runtime in this environment, so Windows Server runtime behaviour, filesystem locking and configuration replacement still need the Windows acceptance steps in [TESTING.md](TESTING.md). Linux and Chrome checks do not substitute for those checks.

## Repeating the checks

See [TESTING.md](TESTING.md) and the root README. `tools/browser-smoke.cjs` uses Playwright as a separate development tool; `tools/cli-smoke.py` uses Python 3.12 or newer and the built Linux archive. Neither is an application runtime dependency. Both smoke tools need permission to open a temporary localhost listening socket in restricted development environments.
