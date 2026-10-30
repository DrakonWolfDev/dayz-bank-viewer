# Wolf Forge website setup

DayZ Bank Viewer belongs on the existing Wolf Forge static website, as confirmed by the maintainer. Its source checkout is `/home/andi/Dev/wolf-forge-static` (resolved here as `/mnt/shared/Dev/wolf-forge-static`). This is a separate repository from the banking viewer.

## Prepared files

- `dayz-bank-viewer.html`, `dayz-bank-viewer.css` and `dayz-bank-viewer.js`: product page, responsive styling and stable download handling.
- `assets/dayz-bank-viewer.png`: existing application mark.
- `assets/dayz-bank-dashboard.png`: real dashboard screenshot generated from five synthetic demonstration accounts. No private banking records are used.
- `projects.html`: product card in On the Workbench while the stable release is being prepared.
- `releases.json`: a `dayz_bank_viewer` entry with stable availability disabled. The 1.4.2 testing prerelease has a separate, explicit GitHub link.
- `tools/update_release.py` and its existing tests: support this product's exact artifact set rather than requiring RPM/AppImage formats the viewer does not ship.

The page reuses the site's navigation, typography, footer and local assets. It explains supported banking formats, player/shared totals, desktop/unattended operation, offline banking reads and remote-access setup. It does not host the banking dashboard or connect to a server.

## Validation

The website release-manifest tests and JavaScript syntax check pass. Browser checks cover desktop, tablet and mobile widths, image loading, horizontal overflow, catalogue navigation, complete stable download activation, missing-source rejection and unavailable-manifest fallback. The screenshot uses synthetic records. Preview screenshots are under the viewer's ignored `artifacts/website-*.png`.

The maintainer confirmed that pushing the website repository to GitHub deploys the site. Website commit `2bd4be9` was pushed to `main` on 30 October 2026. The live [product page](https://wolf-forge.co.uk/dayz-bank-viewer.html), CSS, JavaScript, icon and dashboard image returned HTTP 200 and matched the local files. The live Projects page contains the product link, and the live release manifest keeps stable downloads disabled. No new Sites-hosted copy or hosting migration was needed.

## Stable download promotion

Keep `available: false` until the next stable release has been published and all files have been verified. The page enables download links only for a stable three-part version and a complete official artifact set. A failed manifest fetch keeps the explicit testing link and disabled stable controls.

The website's existing `desktop_release_published` repository-dispatch workflow accepts:

```json
{
  "project": "dayz_bank_viewer",
  "version": "1.4.3",
  "tag": "v1.4.3",
  "prerelease": false,
  "artifacts": {
    "windows": "dayz-bank-viewer-1.4.3-windows-amd64-setup.exe",
    "linux_deb": "dayz-bank-viewer-1.4.3-linux-amd64.deb",
    "windows_portable": "dayz-bank-viewer-1.4.3-windows-amd64.zip",
    "linux_portable": "dayz-bank-viewer-1.4.3-linux-amd64.tar.gz",
    "source": "dayz-bank-viewer-1.4.3-source.tar.gz",
    "checksums": "SHA256SUMS"
  }
}
```

This is a payload example, not a publication record. The viewer's [GitHub Actions release automation](AUTOMATION.md) now builds and verifies tagged source, publishes the complete stable release and dispatches this payload using `WOLF_FORGE_SITE_TOKEN`. The website's existing receiver verifies the URLs before committing its manifest. Confirm the receiver's run and live download links after publication. The separate **Update website downloads** action can retry a handoff without replacing release files. Move the catalogue card to Out in the Wild and update its status when promoting the first stable release.
