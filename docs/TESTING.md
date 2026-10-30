# Verification and acceptance

## Automated checks

Run `go test -race -cover ./...` and `go vet ./...` from the project root. Tests use temporary directories and synthetic player files. They cover:

- Required fields, optional values, BOMs, invalid types, integer limits, duplicate IDs, retry recovery and cancellation.
- Full, partial, empty and failed scans, recovery, cache retention and immutable snapshot publication.
- Independent server scans, overlap prevention and discarding a result after its path changes.
- Exact network totals, unique players, disabled-server exclusion, safe configuration removal and concurrent readers.
- Configuration validation and persistence, failed writes, host-local schedules and the repeated daylight-saving hour.
- Embedded assets, API precision, unknown paths, JSON-only writes, origin restrictions and loopback Host protection.

`TestPrivateCorpusReadOnly` is opt-in; see the README. Never use live account values as committed test expectations or public screenshots.

An optional repeatable Chrome smoke test is provided as `tools/browser-smoke.cjs`. Install Playwright in a separate development tools folder, set `NODE_PATH` to that folder's `node_modules`, then run `node tools/browser-smoke.cjs dist/linux-amd64/dayz-bank-viewer`. Set `DAYZ_BROWSER` if Chrome is not at `/usr/bin/google-chrome`. This tool uses temporary synthetic fixtures and a temporary localhost port; it leaves screenshots under ignored `artifacts/`. Node and Playwright are development tools only and are not included in the app or release archives.

## Browser acceptance

Use a current browser supporting native dialogs, `BigInt` and Fetch. All frontend resources must load locally. Use synthetic files for screenshots or shared results.

1. Start with no configuration. Confirm the empty state opens Settings.
2. Add two banking folders. Save, wait for scans, switch servers and verify network account counts versus unique players.
3. Search by name and Steam ID. Sort names and balances in both directions. Page through more than 50 accounts. Check that a balance above 9,007,199,254,740,991 stays exact.
4. Open a player's details. Confirm bonus currency and paycheck bonus are separate from the ranked balance.
5. Introduce a broken JSON file. Refresh and check incomplete warnings and filenames. Make all account files unreadable, refresh, and check that the previous results remain with an error. Restore valid files and check recovery.
6. Disable, rename, repoint and remove a configuration. Check cached results and scan states, and verify source files are unchanged.
7. Set daily times, save, reopen Settings, restart, and verify settings persistence. Check blank times disable scheduling. Change the listen address and verify the restart notice.
8. Navigate controls and dialogs with the keyboard, Escape to close, verify focus visibility and dialog focus return. Check desktop, tablet and narrow layouts; the account table may scroll horizontally.
9. Disconnect network access and confirm localhost operation. Inspect browser requests for unexpected external traffic or console errors.

## Windows Server acceptance

The Linux environment can cross-compile the Windows PE executable, but Windows runtime checks require a Windows host:

1. Extract the Windows ZIP, launch the EXE without Go or Node installed, and visit localhost.
2. Add a banking path containing spaces. Save settings twice to verify replacement of an existing config on the target filesystem.
3. Test unreadable/temporarily locked account files, a missing directory and recovery after it returns.
4. Repeat the browser acceptance flow, including host-local schedules, and verify file hashes.
5. Launch a second instance on the same port and check the clear port-conflict error. Start with a different `-listen` port.
6. Stop with Ctrl+C and restart from a different working directory. Confirm the configuration beside the executable is still used.

Production installation as an OS service, authentication, remote transfer and other banking formats are outside the supplied v1 brief.
