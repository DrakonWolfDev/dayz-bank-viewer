# DayZ Bank Viewer identity

The application mark is an angular wolf head stamped into a gold coin. The upright ears, broad cheek ruffs and tapered muzzle give the product a wolf identity. The minted circular rim provides the banking cue; charcoal and muted amber retain the dashboard's existing survival-oriented palette.

This replaces the previous classical bank-building mark. Wolf Forge provenance remains in the dashboard footer with its established paw-print signature.

`tools/icon/main.go` defines the shared vector geometry and palette. Regenerate all application icon assets from the repository root:

```sh
go run ./tools/icon
```

This writes:

- `web/favicon.svg`: scalable dashboard, browser and Linux application icon.
- `web/app.png`: 256-pixel raster preview.
- `web/app.ico`: Windows icon containing individually rendered 16, 32, 48, 64, 128 and 256-pixel images.

The renderer uses the Go standard library, with antialiasing at each output size. No image service, font, third-party logo or external asset is required to regenerate the mark. Rebuild local packages with `go run ./tools/package` after changing it.
