# Verification and acceptance

## Automated checks

GitHub Actions runs the checks below and native Windows tests for each push/PR. See [AUTOMATION.md](AUTOMATION.md) for development artifacts, tagged publication and website delivery. Run `python3 -m unittest discover -s tools -p '*_test.py'` to verify release inventory, checksum failures, immutable public releases and website dispatch boundaries.

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
5. Launch a second instance with the same configuration and check the ownership error. Occupy the configured port with a different application and verify fallback to one of the next twenty ports.
6. Stop with Ctrl+C and restart from a different working directory. Confirm the configuration beside the executable is still used.
7. Check the Windows control window at 100%, 150% and 200% scaling, in high contrast and with a smaller resized window. Confirm no PowerShell console remains open or flashes during launch, polling, updates or service actions; the control window and UAC prompts must remain visible. Verify the actual wolf-and-coin logo, charcoal/gold panels, system colours in high contrast, grouped buttons, visible keyboard focus, Enter to open the dashboard, selectable address text and scrolling without clipped actions. Verify running/stopped status and the actual address on Windows PowerShell 5.1, including non-ASCII account and configuration paths. During a slow update check or service action, move/resize/close the window and confirm it keeps responding. Every button must execute its displayed action and refresh the state afterwards. Start should be disabled while running; Stop/Restart should be disabled while stopped. Stop an unattended service and confirm Switch back to desktop mode remains available. Check every action, including both startup-mode transitions.
8. On Server Core, silently install and run the README's elevated `-headless -service-setup` command with valid and inaccessible folders. Check process exit codes and no dialogs on failure. Reboot without signing in, connect through the documented SSH tunnel and verify Settings and refresh. Confirm failed setup preserves the previous service state.

## V1.1–V1.4 acceptance

Run `python3 tools/lifecycle-smoke.py`, `python3 tools/cli-smoke.py`, and the browser smoke after packaging. The lifecycle script stubs native desktop tools so it can verify browser-launch policy without opening real windows; it does not validate native widgets or installer execution.

For each actual target OS, use an isolated VM with synthetic banking records and record the results:

1. Install the setup EXE / DEB on a clean host without development tools. Verify menu entry, icon, installed version, licence and source links. Windows service runs as LocalService; Linux as dayz-bank-viewer.
2. Launch from the menu. On a fresh installation confirm the viewer runs as the signed-in user, uses personal settings, reads a private banking folder that user can read, and opens without an administrator prompt. Confirm the system service remains stopped/disabled, startup readiness, actual port and browser opening. Launch again and verify one service and no extra tab. Confirm the desktop icon and status at 100%, 150% and 200% scaling and keyboard access to every control.
3. Occupy the preferred port; confirm actual port is within the next twenty. Occupy the whole range and confirm a clear failure. Release the ports and retry.
4. Add synthetic records through browser Settings, granting only read access to the service account. Repeat banking acceptance. Compare source directory contents and hashes before and after.
5. Check updates while current, with a newer stable release, offline, with a GitHub rate-limit response, and with malformed data. Confirm failures leave the dashboard usable. No updater should download or install anything automatically.
6. Stop and restart from the desktop; confirm unrelated processes remain. Enable boot startup, reboot without logging in, and verify the dashboard is reachable from localhost and recovers after terminating only its own service process. No browser should open at boot. Disable startup, reboot and confirm it stays stopped.
7. Upgrade while running, then while stopped. Configuration and boot preference must survive; a previously running native service must restart. Test paths containing spaces.
8. Uninstall with the control window open. Verify the service is stopped, disabled and removed, application files/menu entries are removed, and no app processes remain. Configuration, logs, exported reports and source hashes must survive.
9. On Linux test ordinary removal, reinstall with retained settings, and explicit purge. Purge may remove only known configuration/runtime files. Banking sources, logs, exports and unknown user files must survive. On Windows inspect retained ProgramData and normal uninstall registration cleanup.
10. Test portable archives separately, including startup from another working directory and headless operation without display variables. Check the default localhost binding and that existing origin/Host restrictions still apply.

Authentication and remote transfer remain outside scope. Actual platform lifecycle verification is required before describing an installer as production verified.

## Desktop/unattended mode transitions (1.4.1)

1. Upgrade a running 1.4.0 service with existing settings. On first desktop launch choose Run as me. Verify settings are copied to the personal directory, original service settings remain, the old service is stopped/disabled after confirmation, and the viewer reads the original private home path as the signed-in user. Relaunch and confirm no repeat migration prompt, extra runtime or browser tab.
2. Repeat the upgrade choice selecting Keep the unattended service; then verify subsequent launches retain that choice. Cancel the upgrade choice and verify the service/settings remain unchanged.
3. From desktop mode select Run without signing in. Verify the account/folder explanation and administrator prompt. Deny approval and confirm desktop mode stays running. With an inaccessible folder, approve setup and verify access failure identifies the service account/path, boot startup remains disabled, previous service configuration/startup preference is restored, and the desktop viewer restarts. Source data and ACLs must remain unchanged.
4. With service-readable folders, repeat setup. Verify access succeeds under the actual service identity before startup is enabled; the desktop runtime stops and service mode is remembered. Reboot without login and verify recovery and dashboard readiness.
5. Change Settings in service mode, then Switch back to desktop mode. Confirm current service settings are copied back, the prior personal copy is backed up, unattended startup stops/disables and the viewer restarts as the signed-in user. Test this with the service stopped as well as running.
6. Upgrade/remove with a personal runtime and control window open. Verify only this installed application's processes close, package files are removed/replaced, personal/service settings are retained, and unrelated applications remain running. Test both platforms and multiple signed-in users.

## Automatic banking formats

The Go suite uses synthetic Banking/KR, DC Banking and Expansion ATM records. It checks schema detection, ambiguous and unknown records, strict integer types, exact totals, separate ID namespaces, capacity bonuses and unchanged source contents.

For each real mod/version, select its player account folder from the README, refresh, and confirm the detected label, balances and identifiers against the server files. Verify Expansion names are either unavailable or labelled as coming from local ADM logs, Steam64 IDs remain unavailable, and DC capacity bonuses remain separate from balances. Do not use real player data in shared screenshots. Repeat partial/failure/recovery acceptance for each format.

## ADM name lookup

Synthetic tests cover old/new connection syntax, BOM/CRLF, player-list style entries, name changes, chat-text rejection, missing matches, automatic profiles-folder discovery, explicit folder persistence, bounded tail/file reads, cancellation and symlink rejection. The browser check confirms that log-derived names are searchable, labelled by source, and leave Steam64 unavailable. Missing logs must leave balances usable. Verify copied real ADM files against known accounts locally when available.


## Shared accounts and deeper ADM history

Add a KR clan source (`KR_BANKING/ClanDataBase`) or Expansion party source (`ExpansionMod/Groups`) with account type **Clans & parties**. Verify the shared total independently, switch between Players and Clans & parties, and check that totals, source options, names, IDs, sorting, search and details follow the selected view. Break and restore one shared account; verify cached clan results and diagnostics while player totals remain complete. Changing source account type must invalidate its cache. Disabled clan sources must be excluded from aggregate clan totals.

Expansion binary versions 39–42 are supported; reject unsupported versions, truncated fields, invalid lengths/counts, invalid UTF-8 and trailing bytes. Native `.bin` accounts take precedence over same-stem legacy JSON. Synthetic tests cover all four versions; the opt-in private test currently accepts version 39 so its expected money can be calculated independently from that version’s final int32 field:

```sh
DAYZ_BANK_TEST_GROUP_CORPUS=/absolute/path/to/ExpansionMod/Groups go test -run TestPrivatePartyCorpusReadOnly -v
```

The private test reports counts and verification only, checks independent totals and unchanged source hashes, and never installs data as fixtures. Keep original backups outside Git. ADM tests must reach a required name in the 60th newest file and exclude the 61st; lookup stays limited to 4 MiB per file and 240 MiB per refresh and stops early when all needed player names are found.


## Server folder discovery

In Settings, enter a server directory or full backup and detect folders. Verify profile/config/SC and Log_Storage case variants, multiple player banking mods, shared clan/party folders, direct ADM logs and per-source log choices. Check the mission template and its configuration path, including mismatches between backup folder names and configuration contents. Several mission settings must require a choice. Unknown map suffixes stay visible without inventing an active map.

Discovery alone must leave viewer configuration, account snapshots and source files unchanged. Save directly from the detected-folder preview, or use Add another server to stage a second root; Cancel discards unsaved sources and Save persists them. Existing source paths must be marked to prevent duplicate additions. Empty/unverified folders remain unchecked, custom configuration paths stay inside the chosen root, child symlinks and unrelated backup subtrees are not traversed, and passwords/account contents must never appear in preview JSON or rendered content. Changing the root while a request is pending must invalidate its preview.

The optional real-backup check logs only the inferred map, counts and verification result:

```sh
DAYZ_BANK_TEST_SERVER_ROOT=/absolute/path/to/server-backup go test -run TestPrivateServerDiscoveryReadOnly -v
```

The supplied 2023 backup exposed one player folder, one party folder and one ADM location. Its serverDZ.cfg suggested Chernarus while the custom Config.cfg contained Namalsk. Both configurations must be discovered and available for explicit selection; this is evidence for requiring review rather than assuming the active launch configuration.


## Guided settings layout

Fresh setup shows Settings help, the server root, collapsed configuration override and adjacent Detect server folders / Manually add server choices. Scheduled refresh, Connection, Save/Cancel and Updates appear after choosing a route. Detection must keep the duplicate manual source fields hidden, allow saving reviewed choices directly, and let Add another server stage a root without writing configuration. Manual setup reveals source fields and an Add another server button. Existing configuration remains editable through a collapsed source control when reopening Settings.

Connection starts collapsed; invalid required fields inside a closed details control must reveal the control for correction. Save/Cancel appear before the expanded Updates section. A successful save must close Settings and reveal the account table; failed saves must leave Settings open. The manual button appears on the left and the detection button on the right. A failed save must retain detected selections and editable mission/name choices for correction.
