package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestInstalledDesktopUsesPersonalSettings(t *testing.T) {
	for _, platform := range []string{"linux", "windows"} {
		personal, service := configLocations("/programs/viewer", platform, "/private/user/config", "/shared/programdata", true)
		if personal != filepath.Join("/private/user/config", "DayZ Bank Viewer", "config.json") {
			t.Fatal(personal)
		}
		if personal == service {
			t.Fatal("desktop settings must not use service identity or storage")
		}
	}
	personal, _ := configLocations("/portable/viewer", "linux", "/private/user/config", "", false)
	if personal != "/portable/config.json" {
		t.Fatal(personal)
	}
	config := filepath.Join(t.TempDir(), "config.json")
	if effectiveConfig(config) != config {
		t.Fatal("default mode is not personal")
	}
	if err := setDesktopMode(config, true); err != nil {
		t.Fatal(err)
	}
	if effectiveConfig(config) != systemConfigPath() {
		t.Fatal("explicit service selection not retained")
	}
	if err := setDesktopMode(config, false); err != nil {
		t.Fatal(err)
	}
	if effectiveConfig(config) != config {
		t.Fatal("switching back retained service routing")
	}
}
func TestUnattendedSetupChecksAccessBeforeEnabling(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "service.json")
	folder := t.TempDir()
	c := defaultConfig()
	c.Servers = []ServerConfig{{ID: "test", Name: "Test", Path: folder, Enabled: true}}
	var operations []string
	ops := setupOperations{
		control: func(action string) error { operations = append(operations, action); return nil },
		own:     func(string) error { return nil }, enabled: func() bool { return false },
		check: func(path string) error {
			operations = append(operations, "check")
			saved, err := loadConfig(path)
			if err != nil {
				return err
			}
			if saved.Servers[0].Path != folder {
				t.Fatal("settings were not copied")
			}
			return nil
		},
	}
	if err := configureService(c, destination, ops); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(operations, []string{"stop", "start", "check", "enable"}) {
		t.Fatal(operations)
	}
}
func TestFailedServiceSetupPreservesPreviousSettings(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "service.json")
	original := defaultConfig()
	original.RefreshTimes = []string{"06:00"}
	if err := saveConfig(destination, original); err != nil {
		t.Fatal(err)
	}
	originalRaw, _ := os.ReadFile(destination)
	next := defaultConfig()
	next.RefreshTimes = []string{"18:00"}
	var operations []string
	ops := setupOperations{control: func(action string) error { operations = append(operations, action); return nil }, own: func(string) error { return nil }, enabled: func() bool { return false }, check: func(string) error { return errors.New("service cannot read this folder") }}
	if err := configureService(next, destination, ops); err == nil || !strings.Contains(err.Error(), "cannot read") {
		t.Fatal(err)
	}
	restored, _ := os.ReadFile(destination)
	if string(restored) != string(originalRaw) {
		t.Fatal("existing settings changed after failed setup")
	}
	for _, action := range operations {
		if action == "enable" {
			t.Fatal("boot startup enabled despite denied access")
		}
	}
	backups, _ := filepath.Glob(destination + ".before-setup-*")
	if len(backups) != 1 {
		t.Fatal("previous settings backup missing")
	}
}
func TestReadOnlyFolderAccessChecks(t *testing.T) {
	folder := filepath.Join(t.TempDir(), "private banking")
	if err := os.Mkdir(folder, 0700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(folder, "synthetic.json")
	original := []byte("synthetic input need not be parsed for a permission check")
	if err := os.WriteFile(file, original, 0600); err != nil {
		t.Fatal(err)
	}
	c := defaultConfig()
	c.Servers = []ServerConfig{{ID: "a", Name: "Private", Path: folder, Enabled: true}, {ID: "b", Name: "Missing", Path: filepath.Join(folder, "missing"), Enabled: true}, {ID: "c", Name: "Disabled", Path: "/unavailable", Enabled: false}}
	report := checkBankingAccess(context.Background(), c)
	if len(report.Folders) != 2 || report.Identity == "" || report.Folders[0].Error != "" || report.Folders[1].Error == "" {
		t.Fatal(report)
	}
	info, _ := os.Stat(file)
	if info.Mode().Perm() != 0600 {
		t.Fatal("access checks changed source permissions")
	}
	after, _ := os.ReadFile(file)
	if string(after) != string(original) {
		t.Fatal("permission checks changed banking data")
	}
	entries, _ := os.ReadDir(folder)
	if len(entries) != 1 {
		t.Fatal("permission checks wrote to banking folder")
	}
}

func TestServiceAccessCheckReportsAccountAndPath(t *testing.T) {
	root := t.TempDir()
	config := filepath.Join(root, "config.json")
	missing := filepath.Join(root, "unavailable banking")
	c := defaultConfig()
	c.Servers = []ServerConfig{{ID: "a", Name: "My server", Path: missing, Enabled: true}}
	if err := saveConfig(config, c); err != nil {
		t.Fatal(err)
	}
	cleanup, err := publishInstance(config, "http://127.0.0.1:20784", func() {})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	err = serviceAccessCheck(config)
	if err == nil || !strings.Contains(err.Error(), missing) || !strings.Contains(err.Error(), "My server") || !strings.Contains(err.Error(), "read access") {
		t.Fatal(err)
	}
	if err = os.Mkdir(missing, 0700); err != nil {
		t.Fatal(err)
	}
	if err = serviceAccessCheck(config); err != nil {
		t.Fatal(err)
	}
}
