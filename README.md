# DayZ Bank Viewer

A small, self-hosted, read-only banking dashboard for DayZ administrators. One executable serves the dashboard and reads locally accessible banking directories. No database, Node.js runtime, external services, fonts, analytics or internet connection is needed to run it.

## Install and launch

Download the **Windows x64 setup EXE** or **Debian/Ubuntu x64 DEB** from the project's [GitHub releases](https://github.com/DrakonWolfDev/dayz-bank-viewer/releases). Version **1.4.2** is a [prerelease for server testing](https://github.com/DrakonWolfDev/dayz-bank-viewer/releases/tag/v1.4.2). Download it manually while home-server and production-server acceptance are pending. Windows runtime validation is still required.

- **Windows:** run the setup EXE and launch **DayZ Bank Viewer** from the Start menu. Installation requires administrator permission. The installer registers a Windows Service with manual startup by default.
- **Debian / Ubuntu:** run `sudo apt install ./dayz-bank-viewer-1.4.2-linux-amd64.deb`, then launch **DayZ Bank Viewer** from the application menu. Apt supplies desktop dependencies. A systemd service is installed with boot startup disabled by default.

The small desktop window runs the viewer as the account that opened it. It opens the browser once when starting the viewer and displays its status and actual dashboard address. Use **Open dashboard** to open it again. Settings in the browser remain the primary interface. Add an absolute local banking directory, for example `D:/DayZ/Server1/Banking/PlayerDataBase` or `/srv/dayz/profiles/Banking/PlayerDataBase`.

The desktop offers start, stop, restart, update checks and About/licence. Normal desktop use runs as **you**, with your existing folder permissions and personal configuration. It does not require administrator permission or start the system service. Closing the control window leaves your viewer running until you stop it or sign out; desktop mode does not promise operation after sign-out.

**Run without signing in…** is a separate, optional setup step. It explains the service account and selected folders, copies a snapshot of your settings, and checks folder/file access while running as that account. Only a successful check enables boot startup. If setup fails, previous service settings/startup preference are restored and your desktop viewer is restarted. Banking files and their permissions are never changed by setup. Administrators can grant read-only access if they need this mode.

**Switch back to desktop mode** preserves current service settings, disables/stops the service and starts the viewer as you again. Service operations use the platform's administrator permission prompt. Boot startup needs no user session, never opens a browser and recovers after unexpected failure.

When upgrading from 1.4.0, the first desktop launch offers **Run as me (recommended)** or **Keep the unattended service**. Choosing desktop mode copies existing settings without deleting the old service configuration, then stops/disables the old service. Switching an existing service may require administrator permission once. A fresh installation needs no service setup for normal use.

Packages need no Python, Go or Node installation. Windows desktop controls use built-in Windows PowerShell and .NET Framework Windows Forms. Linux desktop controls need Zenity, xdg-utils and PolicyKit. Server Core supports the headless service rather than the desktop window. See [platform verification](docs/VERIFICATION.md) before production rollout.

## Portable and headless operation

The ZIP and tar.gz remain available. Extract into a writable application folder, away from banking directories. Run `dayz-bank-viewer -desktop` for the control window, or `dayz-bank-viewer -headless` for a foreground process (Ctrl+C stops it). On Linux make the executable executable first. On Windows append `.exe`.

The configured port defaults to **20784**. If occupied by another application, the viewer binds the next available port, trying at most twenty additional ports. The log, desktop status and discovery file report the actual address. Instances using the same canonical configuration cannot run twice. Separate configurations intentionally support separate viewers.

```sh
./dayz-bank-viewer -config /absolute/path/config.json -control status
./dayz-bank-viewer -config /absolute/path/config.json -control start
./dayz-bank-viewer -config /absolute/path/config.json -control restart
./dayz-bank-viewer -config /absolute/path/config.json -control stop
./dayz-bank-viewer -control updates
```

Portable `start` detaches the service process and writes startup messages to `config.json.log`. Reconnecting does not open another browser tab. Installed desktop copies use personal settings and run as the signed-in account. After explicitly selecting unattended mode, the control window uses the registered native service. On a headless Linux machine use `sudo systemctl start|stop|restart dayz-bank-viewer`; `sudo systemctl enable|disable dayz-bank-viewer` controls boot startup. On Windows an elevated PowerShell can use `Start-Service`, `Stop-Service`, `Restart-Service` and `Set-Service DayZBankViewer -StartupType Automatic|Manual`.

For headless setup, an administrator can run `sudo /usr/bin/dayz-bank-viewer -service-setup "/absolute/path/to/config.json"` on Linux. This copies that configuration, checks access as the service account and enables startup only if the check succeeds.

Only the optional unattended service runs as a separate identity. The Linux service runs as the restricted `dayz-bank-viewer` user; Windows uses `NT AUTHORITY\LocalService`. Grant that identity read/traverse access to the selected banking directories using your normal filesystem permissions. It needs no write permission there. Desktop controls report unavailable paths through the existing scan warnings. Do not make banking directories writable to fix a read-permission problem.

## Folder permissions

In desktop mode the viewer uses the same account as the person who opens it. It can read the folders that account can read. Choose your banking folder in Settings as before; there is no extra service identity to configure.

In optional unattended mode, folders must be readable by `dayz-bank-viewer` on Linux or `NT AUTHORITY\LocalService` on Windows. The setup checks each enabled folder and its existing JSON files before enabling boot startup, and identifies the account/path if access is blocked. Future files need inherited read permissions too. An administrator can use targeted ACLs or group permissions; no banking write access is needed. The setup never grants access or changes source permissions automatically.

A private Linux home directory can block traversal even when the target folder is readable. If `~/Dev` is a symlink to an already accessible `/mnt/shared/Dev`, selecting the direct `/mnt/shared/Dev/.../PlayerDataBase` path can avoid that private parent. This workaround is relevant to unattended mode; normal desktop mode can use your home path directly.

## Updates and uninstall

Settings shows the installed version and a **Check for updates** button. The desktop offers the same check. Only a manual check contacts GitHub; no records or folder paths are sent. Offline and rate-limit failures leave the viewer usable. A newer stable release offers a link to its official downloads. Nothing is downloaded or installed automatically.

Use **Installed apps / Add or remove programs** on Windows or `sudo apt remove dayz-bank-viewer` on Linux. Removal stops and unregisters the application service and removes package-owned files and shortcuts. User configuration, logs, exports and banking sources are retained. `sudo apt purge dayz-bank-viewer` explicitly removes known application configuration/state; it retains logs, exports, unknown user files and all banking records. Windows retains `%ProgramData%\DayZ Bank Viewer` for deliberate user cleanup. Personal desktop settings under your account are retained on both platforms, including on package purge. Never point cleanup commands at a banking source.

Upgrades retain configuration and boot-start preferences and restart a previously running native service. Portable users should stop their old viewer before replacing its binary.

## In-app help

Choose **Help** in the dashboard header or **Settings help** inside Settings for an offline guide to every setting, account folders, two banking mods on one server, ADM names, refreshes and scan problems. Opening help from Settings preserves unsaved edits.

## Configuration

Portable `config.json` lives **beside the executable**, regardless of the working directory. Installed desktop copies use `~/.config/DayZ Bank Viewer/config.json` on Linux (or the configured `XDG_CONFIG_HOME`) and `%APPDATA%\DayZ Bank Viewer\config.json` on Windows. Optional unattended services retain `/var/lib/dayz-bank-viewer/config.json` and `%ProgramData%\DayZ Bank Viewer\config.json` respectively. Service selection is remembered per user beside personal settings. Private runtime credentials and public port/status discovery sit beside the configuration; neither contains account records. On first launch it is optional; Settings creates it when saved. `config.example.json` is an empty starting point. You can also create:

```json
{
  "listen": "127.0.0.1:20784",
  "servers": [
    {
      "name": "Chernarus",
      "path": "D:/DayZ/Server1/Banking/PlayerDataBase",
      "enabled": true
    }
  ],
  "refresh_times": ["06:00", "18:00"]
}
```

Server IDs are generated automatically if omitted and persisted when Settings is saved. Use forward slashes or escaped backslashes in Windows JSON paths. Directories may be missing temporarily; scans report the problem without discarding cached results. Changes made through Settings apply immediately, except the listen address, which requires an app restart. Changes made in a text editor require a restart.

```sh
dayz-bank-viewer -config /path/to/config.json
dayz-bank-viewer -listen 127.0.0.1:20785
dayz-bank-viewer -version
```

On Windows, use `./dayz-bank-viewer.exe` in PowerShell. `-listen` overrides the address for that run only. An occupied port triggers bounded automatic fallback; exhaustion produces a clear error. Keep configuration outside banking folders; saves use a temporary file and replacement, and a failed save leaves the active configuration unchanged.

The default binding accepts access from this machine only. There is **no built-in login or TLS**. If you intentionally bind to a LAN address or `0.0.0.0`, restrict access to trusted administrators with a firewall or authenticated reverse proxy. Do not expose the unauthenticated interface directly to the internet. A reverse proxy must preserve the public `Host` and `Origin` and connect to a listener configured for that use. Cross-origin browser requests are rejected; no CORS access is provided.

## Detect server folders

In **Settings → Find a server’s folders**, enter the absolute path to a server installation or complete backup and choose **Detect server folders**. This is a read-only preview: it does not save configuration or start full account scans.

Discovery checks the selected directory itself and direct `profiles`, `profile`, `config` and `SC` directories, ignoring case. It looks for the supported Banking/KR, DC Banking and Expansion player folders, KR clan and Expansion party folders, direct ADM logs and `Log_Storage`. Multiple banking mods and separate clan folders appear as separate choices. It does not recurse through arbitrary backups or follow linked child directories. Each directory is limited to 10,000 entries; account sampling checks at most eight eligible files per folder, each capped at 1 MiB.

The `template` inside `class Missions` in a configuration file suggests a map name. Detection checks `.cfg`, `.dz`, `.conf` and `.config` files in the selected server directory and recognised profile folders, including custom names such as `Config.cfg`. Up to 64 files are inspected, prioritising conventional `serverDZ.cfg`, `server.cfg` and `server.dz` names. Files without mission settings do not appear in the selector. The preview shows the exact template and configuration path. Several mission settings require a choice; the collapsed custom configuration option provides a manual override inside the chosen server directory if automatic detection misses a file. Comments are ignored, but preprocessor directives and malformed configurations require manual review. Configuration files are read up to 1 MiB; no server passwords, player records or log text are returned by discovery. Mission suffixes not recognised as a map are shown as written.

**This is a suggested configuration, not proof of the active map.** Backups may contain unused default configurations, and launch parameters may choose another configuration or override the mission. Review the detected mission, change the suggested source name if necessary, and tick only the folders you want. A recognised account sample selects a folder by default; empty or unverified folders stay unchecked. Already configured folders are marked to avoid duplicate additions. Expansion log choices default to combining its recognised profile and server-root log folders within the existing 60-file budget; an explicit selection uses just that directory.

**Manually add server** sits on the left and **Detect server folders** on the right. For a new setup, the remaining sections appear after choosing either route. Detection keeps its reviewed sources in the preview: choose refresh times and **Save settings** directly. **Add another server** stages the selected sources and clears the root field for another server; nothing is persisted until Save. The manual route reveals editable source fields. Reopening Settings clears the discovery directory and custom configuration fields, ready for another server. Saved sources and refresh times are preserved. Existing sources can be edited through the collapsed **Edit connected sources** control. **Cancel** discards unsaved additions.

**Scheduled refresh** is shown after choosing a route. **Connection** stays collapsed. **Save settings** and **Cancel** sit above **Application updates**, which stays expanded. Saving closes Settings and returns to the account table. Reopen Settings whenever you want to check for application updates. DayZ allows custom server configuration filenames and `-profiles` paths, so automatic discovery cannot cover every installation. See the [official server configuration reference](https://community.bohemia.net/wiki/DayZ:Server_Configuration).

## Scans and schedules

- Enabled servers scan at startup, manually, and at optional daily times. Settings saves also trigger a scan through the dashboard. Disabled servers retain cached results but are excluded from All servers totals.
- Times use the **host machine's local timezone**, shown in Settings, rather than the browser's timezone. Leave times blank to disable scheduled refresh. A repeated daylight-saving minute runs once; a skipped minute and scans missed during downtime are skipped. Schedules are checked at startup and on minute boundaries.
- Each server has its own cache, timestamps and status. A server cannot have overlapping scans. Up to four servers scan concurrently to bound disk activity.
- A successful scan replaces the in-memory snapshot as a whole. A partial scan publishes only valid accounts and clearly reports every skipped filename. The last successful timestamp advances only for complete scans.
- If every JSON file is unreadable, or the directory cannot be opened, cached results stay visible with a failure warning. An accessible directory with no JSON files is a valid empty result. Caches are in memory and reset when the app exits.
- Transient file-read and JSON errors receive three attempts, with 40 ms and 80 ms delays. Individual files are limited to 1 MiB. Only direct `.json` children are read, without recursion; account symlinks and non-regular files are skipped. Point each server directly at its player database folder.
- Repointing a server resets its cache; any scan of the previous path is discarded. Renaming a server keeps its cached accounts. Disabling a server stops new scans; an already running read may finish.

## Banking format and precision

The viewer automatically detects supported player-account schemas from the fields inside each JSON file. Existing configurations need no changes. Select the account folder itself; detection does not search parent folders or recurse through profiles.

| Format shown | Player account folder under server profiles | Required fields |
| --- | --- | --- |
| Banking / KR Banking compatible | `Banking/PlayerDataBase` or `KR_BANKING/PlayerDataBase` | `m_Steam64ID`, `m_PlayerName`, `m_OwnedCurrency` |
| DayZ-Banking (DC Banking) | `DC_Banking/PlayerDatabase` | `m_PlainID`, `m_Username`, `m_OwnedCurrency` |
| DayZ Expansion ATM | `ExpansionMod/ATM` | `PlayerID`, `MoneyDeposited` |

The dashboard shows detected formats for each source and in account details. A folder may contain multiple supported schemas. Unknown files, settings files, invalid records and records with multiple recognised ID fields are reported as skipped; no balance-field guessing is used. Empty folders have no detected format until valid records exist. Labels describe schema compatibility: Banking and KR Banking share their player fields, so account contents cannot prove which mod produced them.

Banking and DC Banking require a 17-digit string Steam64 ID and a string player name. Expansion stores a DayZ player ID (`PlayerIdentity.GetId()`), with no player name or Steam64 ID in its ATM account file. Expansion accounts retain their original DayZ ID for search and details, and never invent a Steam64 mapping. Local ADM logs can supply a last observed player name; accounts without a match display **Unnamed player**. Network unique counts refer to unique player IDs: Steam64 and DayZ IDs use separate namespaces, so the same person across both ID systems may count twice.

**Only the stored bank balance determines totals and rankings:** `m_OwnedCurrency` for Banking/DC Banking and `MoneyDeposited` for Expansion. Banking's optional `m_BonusCurrency`, `m_PayCheckBonus` and `m_ClanID` appear separately. DC Banking's `m_MaxOwnedCurrencyBonus` increases account capacity and is shown separately as **Account capacity bonus**. Bonuses never increase the displayed balance. Shared KR clan and Expansion party accounts use the separate **Clans & parties** view described below. A player’s `m_ClanID` indicates membership; it does not turn their personal balance into clan funds. Expansion settings-editor exports are configuration, not player account databases.

Currency fields require signed 64-bit JSON integers. Totals use arbitrary precision. API currency values and identifiers are strings; the dashboard uses `BigInt`, including for sorting and formatting, so values above JavaScript's safe integer limit remain exact. Negative integer balances are preserved. Unknown fields are ignored. UTF-8 with or without a BOM is supported.

Duplicate IDs within one identity namespace and directory are reported and skipped after the first valid file in filename order. The same ID on different sources counts as multiple accounts but one unique player ID. Summary cards cover the selected source, regardless of search. Ranks reflect balance ordering within that source and remain stable when searching or sorting by name. Select the Rank heading to sort from first to last; select it again to reverse the order.

The `AccountReader` boundary and schema registry in `banking_formats.go` keep format adapters separate from scanning and the dashboard. This adds local read-only compatibility; it does not install mods or load third-party scripts. The adapters were checked against upstream storage definitions and synthetic fixtures; the supplied local Expansion ATM corpus also passed count, independently calculated total and source-hash checks. Validation of other real-server versions remains necessary.

Storage references: [KR Banking player data](https://github.com/Krypton91/KR_Banking/blob/main/scripts/3_Game/KR_JsonDatabaseHandler.c), [DC Banking player data](https://github.com/DennisVidal/DayZ-Banking/blob/main/Scripts/4_World/BankingData.c), [Expansion ATM data](https://github.com/salutesh/DayZ-Expansion-Scripts/blob/experimental/DayZExpansion/Market/Scripts/4_World/DayZExpansion_Market/Systems/Market/ExpansionMarketATM_Data.c) and [Expansion identity creation](https://github.com/salutesh/DayZ-Expansion-Scripts/blob/experimental/DayZExpansion/Market/Scripts/4_World/DayZExpansion_Market/Systems/Market/ExpansionMarketModule.c).

## Player names from local admin logs

Expansion account refreshes can look up names in local `.ADM` files. Matching uses the exact DayZ ID from the log, never the player name. This supplies display names only; it does not convert IDs to Steam64 or merge identity namespaces. Existing names supplied by banking records remain authoritative.

For the recognised `profiles/ExpansionMod/ATM` layout (also `profile`, `config` and `SC`), the viewer automatically checks direct `.ADM` children of `profiles`, plus existing `profiles/log_storage` and `server/log_storage` folders. The sibling server-root folder is inferred when the profiles folder is named `profiles`, `profile`, `config` or `SC`; when the profiles directory is the server root itself, its own `log_storage` is checked. Log folder names are matched without case sensitivity. Files across these locations share one recency order and read budget. For other layouts, set **Admin log directory (optional)** in that source's Settings to the absolute folder containing its logs. The equivalent per-server JSON setting is `"log_path": "/srv/dayz/profiles"`. Blank means automatic discovery for the standard Expansion layout only. Each source uses its own logs; there is no recursive or system-wide search.

The server must already produce admin logs (`-adminlog`). During account refreshes, the viewer checks up to 60 regular `.ADM` files, newest modification time first, with filename order breaking ties. It searches at most the last 4 MiB of each file and 240 MiB per source refresh. The search stops early once all needed names are found. Sixty files can cover fifteen days at four restarts per day, but deleted logs, log rotation, byte limits and players absent from those logs can still leave names unavailable. The last usable player entry within a file wins, with newer files taking precedence over older files. Keep original log modification times when copying archives if name recency matters. Only IDs needed by currently unnamed accounts are retained in memory.

Logs are read-only and symlinks are skipped. Missing/unreadable logs and read limits appear separately in scan information without making valid banking totals incomplete. Unmatched names stay unavailable. Very long log lines (over 256 KiB) stop that file's search and report an issue. Lookup happens on account refresh, not dashboard polling. No log contents are served or saved, and account details identify names obtained from ADM logs. Names are historical labels and can change between sessions.

If using unattended mode, its service account also needs read/traverse access to the log directory. Changing the configured log directory clears cached names and accounts until the next refresh.


### Shared clan and party accounts

Use **Account view → Clans & parties** for shared balances. In Settings, add a separate source, set **Account type → Clans & parties**, and select the shared account folder:

| Format | Folder beneath profiles | Shared balance |
| --- | --- | --- |
| KR Banking clans | `KR_BANKING/ClanDataBase` | `m_BankAmount` |
| Expansion parties | `ExpansionMod/Groups` | `MoneyDeposited` |

KR records require `m_ClanID` and `m_ClanName`. Expansion supports native `.bin` party versions **39–42** and legacy JSON with `PartyID` and `PartyName`. Binary versions outside this range are reported as unreadable; money is never inferred from an unknown layout. The reader validates every field through the end of the file and caps account files at 1 MiB. A `.bin` file takes precedence over same-stem legacy JSON, which is reported as skipped to avoid counting stale balances twice. No native DayZ runtime or server mod is required.

Each view has its own source selection, totals, rankings, search and account details. Clan IDs and owner details are shown as supplied; member lists, invitations, markers and clan logs are not returned by the API. Group IDs are local to their server and format. Player ADM lookup is only performed for unnamed player accounts, not clans. Banking and DC Banking have no supported shared-account storage adapter; player clan tags are never used to invent shared accounts. Existing sources default to Players, so existing totals are unchanged.

The binary reader was checked against upstream storage code, synthetic versions 39–42 and a private 19-party version 39 backup; counts and an independently calculated total matched, and every source SHA-256 hash remained unchanged. Other real-server versions still need validation.

Storage references: [KR clan database](https://github.com/Krypton91/KR_Banking/blob/main/scripts/3_Game/KR_BankingClanDatabase.c), [Expansion party storage](https://github.com/salutesh/DayZ-Expansion-Scripts/blob/experimental/DayZExpansion/Groups/Scripts/4_World/DayZExpansion_Groups/ExpansionPartyData.c).

## Build and verify

Go **1.22 or newer** is the only build requirement. All application dependencies are in the Go standard library. From this folder:

```sh
go test -race ./...
go vet ./...
go build -buildvcs=false -trimpath -o build/dayz-bank-viewer .
go run ./tools/icon
go run ./tools/package
```

If your environment restricts the normal Go cache, set `GOCACHE` to a writable absolute directory. On Windows, use `go build -buildvcs=false -o build/dayz-bank-viewer.exe .`. For development, `go run . -config ./config.json` avoids putting settings next to Go's temporary executable.

Building the DEB and setup EXE also requires `dpkg-deb` and NSIS `makensis` on the build machine (not on users' machines). Set `MAKENSIS` to an alternate executable path and `NSISDIR` if using an extracted NSIS toolchain.

The package command builds Windows x64 and Linux x64 with CGO disabled, embeds the web assets and timezone database, and writes both archives, the DEB, the setup EXE and `SHA256SUMS` into `dist/`. Unpacked executables are in `dist/windows-amd64/` and `dist/linux-amd64/`. Archives use an explicit allowlist covering the executable, README, example configuration, VERSION, acceptance checklist, application icon, licence and Go notices. `VERSION` is the application/build version source.

Cross-compiling a Windows executable does not establish that it has been run on Windows Server. Perform the Windows acceptance steps in [docs/TESTING.md](docs/TESTING.md) before production rollout.

## Private test corpus

`corpus/` is local test input only. It is ignored by Git, is never embedded or served, and is excluded from the package allowlist. Local `config.json`, binaries, screenshots and test tooling are also ignored. No private records are copied into committed fixtures; automated tests use synthetic accounts.

To check a supplied corpus locally, explicitly point the opt-in test at its account directory:

```sh
DAYZ_BANK_TEST_CORPUS="$PWD/corpus/PlayerDataBase" go test -run TestPrivateCorpusReadOnly -v .
```

The test compares independently calculated totals and checks source SHA-256 hashes before and after scanning. It reports only the account count and verification result. Keep any extra private copies, exported data or configurations outside source control too.

## HTTP API

All responses are uncached. There are no file-download or arbitrary file-reading endpoints.

| Request | Behaviour |
| --- | --- |
| `GET /api/version` | Installed application version |
| `POST /api/updates` | Manual stable GitHub release check; no installation |
| `GET /api/dashboard` | Cached enabled-server accounts, exact summaries, per-server scan metadata and issues |
| `GET /api/dashboard?server=ID` | Cached accounts for one server, including a disabled server |
| `GET /api/config` | Current persisted configuration, including local paths |
| `PUT /api/config` | Validate and save the complete configuration; JSON body required |
| `POST /api/refresh` | Queue enabled servers; `{}` for all or `{"server_id":"ID"}` for one; returns 202 and a started count |

Dashboard polling reads memory; it never initiates file scans. A refresh of a disabled or already scanning server starts zero new scans. The dashboard shows cached data while scanning and polls less often when idle or pauses polling in a hidden tab.

## Licence and source

[GNU Affero General Public License v3 only](LICENSE) (`AGPL-3.0-only`), matching Wolf Launcher. [Licence and third-party information](LICENSING.md). [Version boundaries](docs/RELEASES.md).

[A Wolf Forge Project 🐾](https://wolf-forge.co.uk/)
