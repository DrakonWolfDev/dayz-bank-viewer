# DayZ Bank Viewer

A small, self-hosted, read-only banking dashboard for DayZ administrators. One executable serves the dashboard and reads locally accessible banking directories. No database, Node.js runtime, external services, fonts, analytics or internet connection is needed to run it.

## Start on Windows Server

1. Extract the Windows x64 ZIP into a writable application folder, away from your DayZ banking directories.
2. Run `dayz-bank-viewer.exe`. Keep its console open while using the app.
3. Open **http://127.0.0.1:20784** in your browser.
4. Open **Settings**, choose **Add server**, and enter a name and the absolute path to the folder containing the player JSON files, for example `D:/DayZ/Server1/Banking/PlayerDataBase`.
5. Save settings. The app scans enabled servers and displays their accounts.

Removing a server in Settings removes only its configuration. Original banking files are never edited or deleted. The viewer needs read access to those folders and write access only to its own configuration directory.

## Start on Debian / Ubuntu

Extract the Linux x64 archive into a writable application folder, then:

```sh
chmod +x dayz-bank-viewer
./dayz-bank-viewer
```

Visit the same address and add an absolute directory such as `/srv/dayz/profiles/Banking/PlayerDataBase`. Stop the app using Ctrl+C. Run it independently of DayZ; no server mod or restart is required.

## Configuration

The default `config.json` lives **beside the executable**, regardless of the working directory. On first launch it is optional; Settings creates it when saved. `config.example.json` is an empty starting point. You can also create:

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

On Windows, use `./dayz-bank-viewer.exe` in PowerShell. `-listen` overrides the address for that run only. A port conflict produces an explanatory error and exits. Keep configuration outside banking folders; saves use a temporary file and replacement, and a failed save leaves the active configuration unchanged.

The default binding accepts access from this machine only. There is **no built-in login or TLS**. If you intentionally bind to a LAN address or `0.0.0.0`, restrict access to trusted administrators with a firewall or authenticated reverse proxy. Do not expose the unauthenticated interface directly to the internet. A reverse proxy must preserve the public `Host` and `Origin` and connect to a listener configured for that use. Cross-origin browser requests are rejected; no CORS access is provided.

## Scans and schedules

- Enabled servers scan at startup, manually, and at optional daily times. Settings saves also trigger a scan through the dashboard. Disabled servers retain cached results but are excluded from All servers totals.
- Times use the **host machine's local timezone**, shown in Settings, rather than the browser's timezone. Leave times blank to disable scheduled refresh. A repeated daylight-saving minute runs once; a skipped minute and scans missed during downtime are skipped. Schedules are checked at startup and on minute boundaries.
- Each server has its own cache, timestamps and status. A server cannot have overlapping scans. Up to four servers scan concurrently to bound disk activity.
- A successful scan replaces the in-memory snapshot as a whole. A partial scan publishes only valid accounts and clearly reports every skipped filename. The last successful timestamp advances only for complete scans.
- If every JSON file is unreadable, or the directory cannot be opened, cached results stay visible with a failure warning. An accessible directory with no JSON files is a valid empty result. Caches are in memory and reset when the app exits.
- Transient file-read and JSON errors receive three attempts, with 40 ms and 80 ms delays. Individual files are limited to 1 MiB. Only direct `.json` children are read, without recursion; account symlinks and non-regular files are skipped. Point each server directly at its player database folder.
- Repointing a server resets its cache; any scan of the previous path is discarded. Renaming a server keeps its cached accounts. Disabling a server stops new scans; an already running read may finish.

## Banking format and precision

Each file must be an object with a 17-digit **string** `m_Steam64ID`, a string `m_PlayerName`, and integer `m_OwnedCurrency`. The optional `m_BonusCurrency`, `m_PayCheckBonus`, and string `m_ClanID` appear separately in player details. Unknown fields are ignored. UTF-8 with or without a BOM is supported.

**Only `m_OwnedCurrency` determines balances and rankings.** Individual currency values use signed 64-bit integers. Totals use arbitrary precision. API currency values and Steam IDs are decimal strings; the dashboard uses `BigInt`, including for sorting and formatting, so values above JavaScript's safe integer limit remain exact. Optional bonuses are never added to bank balances. Negative integer balances are preserved.

Duplicate Steam IDs within one directory are reported and skipped after the first valid file in filename order. The same Steam ID on different servers counts as multiple accounts but one unique player in the network view. Summary cards cover the selected banking source, regardless of the current search. Ranks reflect balance ordering within that source and remain stable when searching or sorting by name.

The small `AccountReader` interface isolates the supplied banking format. No other format, remote transfer, database or plugin framework is included in v1.

## Build and verify

Go **1.22 or newer** is the only build requirement. All application dependencies are in the Go standard library. From this folder:

```sh
go test -race ./...
go vet ./...
go build -buildvcs=false -trimpath -o build/dayz-bank-viewer .
go run ./tools/package
```

If your environment restricts the normal Go cache, set `GOCACHE` to a writable absolute directory. On Windows, use `go build -buildvcs=false -o build/dayz-bank-viewer.exe .`. For development, `go run . -config ./config.json` avoids putting settings next to Go's temporary executable.

The package command builds Windows x64 and Linux x64 with CGO disabled, embeds the web assets and timezone database, and writes both archives plus `SHA256SUMS` into `dist/`. Unpacked executables are in `dist/windows-amd64/` and `dist/linux-amd64/`. Archives contain only the executable, this README, example configuration, VERSION and the acceptance checklist. `VERSION` is the application/build version source.

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
| `GET /api/dashboard` | Cached enabled-server accounts, exact summaries, per-server scan metadata and issues |
| `GET /api/dashboard?server=ID` | Cached accounts for one server, including a disabled server |
| `GET /api/config` | Current persisted configuration, including local paths |
| `PUT /api/config` | Validate and save the complete configuration; JSON body required |
| `POST /api/refresh` | Queue enabled servers; `{}` for all or `{"server_id":"ID"}` for one; returns 202 and a started count |

Dashboard polling reads memory; it never initiates file scans. A refresh of a disabled or already scanning server starts zero new scans. The dashboard shows cached data while scanning and polls less often when idle or pauses polling in a hidden tab.
