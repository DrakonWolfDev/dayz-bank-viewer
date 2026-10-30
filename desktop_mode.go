package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

func installedBinary(exe string) bool {
	if runtime.GOOS == "linux" {
		return filepath.Clean(exe) == "/usr/bin/dayz-bank-viewer"
	}
	_, err := os.Stat(filepath.Join(filepath.Dir(exe), "service.install"))
	return err == nil
}
func configLocations(exe, platform, userDir, programData string, installed bool) (string, string) {
	service := "/var/lib/dayz-bank-viewer/config.json"
	if platform == "windows" {
		service = filepath.Join(programData, "DayZ Bank Viewer", "config.json")
	}
	if !installed {
		return filepath.Join(filepath.Dir(exe), "config.json"), service
	}
	if userDir == "" {
		return "", service
	}
	return filepath.Join(userDir, "DayZ Bank Viewer", "config.json"), service
}
func systemConfigPath() string {
	_, service := configLocations("", runtime.GOOS, "", os.Getenv("ProgramData"), true)
	return service
}
func defaultConfigPath(exe string) string {
	dir, _ := os.UserConfigDir()
	personal, _ := configLocations(exe, runtime.GOOS, dir, os.Getenv("ProgramData"), installedBinary(exe))
	return personal
}
func managedInstall(config string) bool {
	exe, _ := os.Executable()
	return installedBinary(exe) && filepath.Clean(config) == filepath.Clean(systemConfigPath())
}
func effectiveConfig(personal string) string {
	if _, err := os.Stat(personal + ".service-mode"); err == nil {
		return systemConfigPath()
	}
	return personal
}
func setDesktopMode(personal string, service bool) error {
	if err := os.MkdirAll(filepath.Dir(personal), 0700); err != nil {
		return err
	}
	if service {
		return os.WriteFile(personal+".service-mode", []byte("Unattended service selected by this user.\n"), 0600)
	}
	if err := os.Remove(personal + ".service-mode"); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.WriteFile(personal+".desktop-mode", []byte("Run as the signed-in user.\n"), 0600)
}
func instanceConfig(config string) (Config, error) {
	state, err := readInstance(config)
	if err != nil {
		return Config{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", state.URL+"/api/config", nil)
	if err != nil {
		return Config{}, err
	}
	client := &http.Client{Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	defer client.CloseIdleConnections()
	res, err := client.Do(req)
	if err != nil {
		return Config{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return Config{}, fmt.Errorf("could not read current service settings")
	}
	var c Config
	if err = json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&c); err != nil {
		return c, err
	}
	return c, validateConfig(&c, config)
}

func lockDesktopMode(personal string) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(personal), 0700); err != nil {
		return nil, err
	}
	unlock, err := acquireLock(personal + ".mode.lock")
	if err != nil {
		return nil, errors.New("another viewer launch or mode change is in progress; finish it in the existing control window")
	}
	var once sync.Once
	return func() { once.Do(unlock) }, nil
}

func waitDesktopModeLock(personal string) (func(), error) {
	var last error
	for i := 0; i < 100; i++ {
		unlock, err := lockDesktopMode(personal)
		if err == nil {
			return unlock, nil
		}
		last = err
		time.Sleep(100 * time.Millisecond)
	}
	return nil, last
}

// A one-time upgrade choice preserves previous settings and never silently stops an existing service.
func prepareDesktop(personal string) error {
	exe, _ := os.Executable()
	if !installedBinary(exe) || managedInstall(personal) || personal != defaultConfigPath(exe) {
		return nil
	}
	unlock, err := waitDesktopModeLock(personal)
	if err != nil {
		return err
	}
	defer unlock()
	for _, suffix := range []string{".service-mode", ".desktop-mode"} {
		if _, err := os.Stat(personal + suffix); err == nil {
			return nil
		}
	}
	legacy := systemConfigPath()
	_, fileErr := os.Stat(legacy)
	_, runningErr := readInstance(legacy)
	if errors.Is(fileErr, os.ErrNotExist) && runningErr != nil {
		return setDesktopMode(personal, false)
	}
	choice, err := choosePreviousMode()
	if err != nil {
		return err
	}
	if choice == "service" {
		return setDesktopMode(personal, true)
	}
	if choice != "desktop" {
		return errors.New("desktop launch cancelled; your existing service and settings were left unchanged")
	}
	if _, err = os.Stat(personal); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	} else if errors.Is(err, os.ErrNotExist) {
		c, e := loadConfig(legacy)
		if e != nil {
			if runningErr != nil {
				if e = nativeService("start"); e != nil {
					return e
				}
				if _, _, e = waitForInstance(legacy); e != nil {
					return e
				}
			}
			c, e = instanceConfig(legacy)
		}
		if e != nil {
			return fmt.Errorf("could not copy previous settings: %w", e)
		}
		if e = os.MkdirAll(filepath.Dir(personal), 0700); e != nil {
			return e
		}
		if e = saveConfig(personal, c); e != nil {
			return e
		}
	}
	if err = nativeService("desktop"); err != nil {
		return err
	}
	return setDesktopMode(personal, false)
}
func waitForInstance(config string) (instance, bool, error) {
	for i := 0; i < 100; i++ {
		if state, e := readInstance(config); e == nil {
			return state, true, nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return instance{}, false, errors.New("service did not become ready; check its startup log")
}
func enableUnattended(personal string) error {
	if effectiveConfig(personal) == systemConfigPath() {
		return errors.New("unattended mode is already selected; switch back to desktop mode before setting it up again")
	}

	exe, _ := os.Executable()
	if !installedBinary(exe) || managedInstall(personal) {
		return errors.New("open the installed desktop viewer to set up unattended startup")
	}
	unlock, err := lockDesktopMode(personal)
	if err != nil {
		return err
	}
	defer unlock()
	c, err := loadConfig(personal)
	if err != nil {
		return err
	}
	var folders []string
	for _, s := range c.Servers {
		if s.Enabled {
			folders = append(folders, s.Name+": "+s.Path)
		}
	}
	if len(folders) == 0 {
		return errors.New("add a banking folder in Settings before setting up unattended startup")
	}
	text := "Run without signing in?\n\nThe desktop viewer already runs as you. Unattended startup uses a separate service account. We will copy your settings and check that account can read these folders before enabling startup:\n\n" + strings.Join(folders, "\n") + "\n\nExisting service settings will be backed up before replacement. If access is blocked, desktop mode stays available. Banking files and their permissions will not be changed. Administrator permission is required."
	if !confirmServiceSetup(text) {
		return nil
	}
	if err = os.MkdirAll(filepath.Dir(personal), 0700); err != nil {
		return err
	}
	snapshot, err := os.CreateTemp(filepath.Dir(personal), ".unattended-*.json")
	if err != nil {
		return err
	}
	request := snapshot.Name()
	snapshot.Close()
	defer os.Remove(request)
	if err = secureRuntime(request); err != nil {
		return err
	}
	if err = saveConfig(request, c); err != nil {
		return err
	}
	// Stop only this user's viewer. A failed setup restores it.
	wasRunning := false
	if _, e := readInstance(personal); e == nil {
		wasRunning = true
		if err = stopInstance(personal); err != nil {
			return err
		}
	}
	if err = setupNativeService(request); err != nil {
		unlock()
		if wasRunning {
			if _, _, restartErr := ensureRunning(personal); restartErr != nil {
				return fmt.Errorf("%w; could not restart desktop viewer: %v", err, restartErr)
			}
		}
		return err
	}
	if err = setDesktopMode(personal, true); err != nil {
		return fmt.Errorf("service enabled, but could not remember the selected mode: %w", err)
	}
	return nil
}
func disableUnattended(personal string) error {
	unlock, err := lockDesktopMode(personal)
	if err != nil {
		return err
	}
	defer unlock()
	if effectiveConfig(personal) != systemConfigPath() {
		return errors.New("this viewer is already running as you; unattended startup has not been selected")
	}
	c, err := instanceConfig(systemConfigPath())
	if err != nil {
		c, err = loadConfig(systemConfigPath())
	}
	if err != nil {
		if err = nativeService("start"); err != nil {
			return err
		}
		if _, _, err = waitForInstance(systemConfigPath()); err != nil {
			return err
		}
		c, err = instanceConfig(systemConfigPath())
		if err != nil {
			return fmt.Errorf("could not preserve service settings: %w", err)
		}
	}
	// Preserve the personal copy before replacing it with settings from the service.
	if _, e := os.Stat(personal); e == nil {
		raw, e := os.ReadFile(personal)
		if e != nil {
			return e
		}
		if e = os.WriteFile(personal+".before-service-"+time.Now().UTC().Format("20060102T150405.000000000"), raw, 0600); e != nil {
			return e
		}
	}
	if err = saveConfig(personal, c); err != nil {
		return err
	}
	if err = nativeService("desktop"); err != nil {
		return err
	}
	if err = setDesktopMode(personal, false); err != nil {
		return err
	}
	unlock()
	_, _, err = ensureRunning(personal)
	return err
}
