# V1.1–V1.4 changes

The current local development version is 1.4.1. Nothing has been published to GitHub or deployed to the Wolf Forge website as part of this work.

## V1.1 — Identity and copy

A clearer local bank icon, installed application icons, header strapline, and centred Wolf Forge provenance link with decorative paw prints. The duplicate overview strapline and server independence footer are removed. The dashboard reports the compiled version instead of a hardcoded string.

## V1.2 — Table readability

Very subtle alternate row tints. Hover and keyboard focus take precedence. The existing dark theme, sorting, filtering, pagination and banking calculations remain in place; no new theme or banking adapter is introduced.

## V1.3 — Manual updates

Settings and the desktop control check the official `DrakonWolfDev/dayz-bank-viewer` GitHub latest stable release. The implementation follows Wolf Updater's bounded request and stable-version selection patterns in Go, without introducing a Python runtime. It validates three-part release versions, limits payload size to 1 MiB, rejects redirects, times out after ten seconds, and distinguishes current, available and failed checks. Requests are manual; repeated dashboard checks share a thirty-second cache. No banking records or paths are sent. Opening a release page is a separate user action; no download or installation is automatic.

## V1.4 — Desktop, installation and lifecycle

NSIS Windows installer, Debian package, separate native service/desktop entry points, socket ownership and bounded port fallback, single-instance locks per canonical configuration, readiness discovery, local authenticated shutdown for portable instances, privileged native service controls, optional boot startup, automatic failure recovery, upgrade and removal scripts.

Windows uses Windows PowerShell and .NET Framework Windows Forms supplied with supported Windows desktop installations. Linux desktop controls use distribution-provided Zenity, xdg-utils and PolicyKit; the service needs no graphical session. The Go executable itself has no third-party Go module dependencies. Linux package dependencies are installed by apt. The Windows desktop does not support Server Core without its graphical components; the service remains usable headlessly.

The application is AGPL-3.0-only, matching Wolf Launcher. Include corresponding source for the exact binary revision before any public release. Packages include the licence and Go notices. Builds are unsigned; signing and real platform acceptance remain release tasks.

## V1.4.1 — Desktop use without service permissions

Installed desktop launches now use the signed-in account and a personal configuration directory. They never implicitly start the restricted system service. A one-time upgrade choice preserves 1.4.0 settings and lets users switch to desktop mode or keep the existing service. Unattended startup is an explicit, guided option: snapshot settings, run folder/file access checks as the actual service identity, then enable boot startup only on success. Failed setup restores previous service configuration/startup preference and restarts the desktop viewer. Switching back imports current service settings and stops/disables unattended operation. Banking source contents and permissions remain unchanged.

Normal desktop startup, stop/restart, browser launch and status use the signed-in user's process. Native service controls remain available only for the selected unattended configuration. Package removal/upgrade also stops personal instances loaded from the installed executable. Windows runtime and real system-service lifecycle acceptance remain required before production distribution.
