package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func copyTo(src, dest string, mode os.FileMode) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}
	return os.WriteFile(dest, data, mode)
}
func installer(version, kind string) (string, error) {
	if kind == "exe" {
		tool := os.Getenv("MAKENSIS")
		if tool == "" {
			tool = "makensis"
		}
		cmd := exec.Command(tool, "-DVERSION="+version, "installer.nsi")
		cmd.Dir = "packaging/windows"
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("Windows installer needs NSIS (makensis or MAKENSIS): %w", err)
		}
		return "dist/dayz-bank-viewer-" + version + "-windows-amd64-setup.exe", nil
	}
	stage, err := os.MkdirTemp("", "dayz-deb-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(stage)
	control := "Package: dayz-bank-viewer\nVersion: " + version + "\nArchitecture: amd64\nMaintainer: Wolf Forge\nSection: utils\nPriority: optional\nDepends: adduser, zenity, xdg-utils, policykit-1\nDescription: Read-only local DayZ banking dashboard\n A Wolf Forge project. Includes optional desktop and systemd service controls.\n"
	if err = os.MkdirAll(filepath.Join(stage, "DEBIAN"), 0755); err != nil {
		return "", err
	}
	if err = os.WriteFile(filepath.Join(stage, "DEBIAN/control"), []byte(control), 0644); err != nil {
		return "", err
	}
	for _, script := range []string{"postinst", "prerm", "postrm"} {
		if err = copyTo("packaging/linux/"+script, filepath.Join(stage, "DEBIAN", script), 0755); err != nil {
			return "", err
		}
	}
	files := map[string]string{
		"dist/linux-amd64/dayz-bank-viewer":        "usr/bin/dayz-bank-viewer",
		"packaging/linux/dayz-bank-viewer.service": "lib/systemd/system/dayz-bank-viewer.service",
		"packaging/linux/dayz-bank-viewer.desktop": "usr/share/applications/dayz-bank-viewer.desktop",
		"web/favicon.svg":                          "usr/share/icons/hicolor/scalable/apps/dayz-bank-viewer.svg",
		"README.md":                                "usr/share/doc/dayz-bank-viewer/README.md",
		"LICENSE":                                  "usr/share/doc/dayz-bank-viewer/copyright",
		"dist/linux-amd64/GO-LICENSE":              "usr/share/doc/dayz-bank-viewer/GO-LICENSE",
		"dist/linux-amd64/GO-PATENTS":              "usr/share/doc/dayz-bank-viewer/GO-PATENTS",
	}
	for src, dest := range files {
		mode := os.FileMode(0644)
		if dest == "usr/bin/dayz-bank-viewer" {
			mode = 0755
		}
		if err = copyTo(src, filepath.Join(stage, dest), mode); err != nil {
			return "", err
		}
	}
	out := "dist/dayz-bank-viewer-" + version + "-linux-amd64.deb"
	cmd := exec.Command("dpkg-deb", "--root-owner-group", "--build", stage, out)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err = cmd.Run(); err != nil {
		return "", err
	}
	return out, nil
}
