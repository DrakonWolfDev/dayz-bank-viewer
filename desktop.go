package main

import (
	"context"
	_ "embed"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

//go:embed desktop.ps1
var windowsDesktop string

func controlAction(config, action string) error {
	switch action {
	case "status":
		state, err := readInstance(config)
		if err != nil {
			fmt.Println("Stopped or unavailable")
			return nil
		}
		address, _ := url.Parse(state.URL)
		fmt.Printf("Running · version %s\nPort: %s\nDashboard: %s\n", state.Version, address.Port(), state.URL)
		return nil
	case "start":
		_, _, err := ensureRunning(config)
		return err
	case "stop":
		if managedInstall(config) {
			return nativeService("stop")
		}
		return stopInstance(config)
	case "restart":
		if managedInstall(config) {
			return nativeService("restart")
		}
		if _, err := readInstance(config); err == nil {
			if err = stopInstance(config); err != nil {
				return err
			}
		}
		_, _, err := ensureRunning(config)
		return err
	case "enable", "disable":
		if !managedInstall(config) {
			return fmt.Errorf("automatic startup is available for installed copies; install the package and use its default configuration")
		}
		return nativeService(action)
	case "updates":
		result := manualUpdate(context.Background())
		fmt.Println("Installed version:", version)
		fmt.Println(result.Message)
		if result.URL != "" {
			fmt.Println(result.URL)
		}
		if result.Status == "failed" {
			return fmt.Errorf("update check unavailable")
		}
		return nil
	}
	return fmt.Errorf("unknown control action: %s", action)
}
func defaultConfigPath(exe string) string {
	if runtime.GOOS == "linux" && filepath.Clean(exe) == "/usr/bin/dayz-bank-viewer" {
		return "/var/lib/dayz-bank-viewer/config.json"
	}
	if runtime.GOOS == "windows" {
		if _, err := os.Stat(filepath.Join(filepath.Dir(exe), "service.install")); err == nil {
			return filepath.Join(os.Getenv("ProgramData"), "DayZ Bank Viewer", "config.json")
		}
	}
	return filepath.Join(filepath.Dir(exe), "config.json")
}
func managedInstall(config string) bool {
	exe, _ := os.Executable()
	if config != defaultConfigPath(exe) {
		return false
	}
	if runtime.GOOS == "windows" {
		_, err := os.Stat(filepath.Join(filepath.Dir(exe), "service.install"))
		return err == nil
	}
	return exe == "/usr/bin/dayz-bank-viewer"
}
func desktopWindow(config string) error {
	state, created, err := ensureRunning(config)
	if err != nil {
		return err
	}
	if created {
		if err = openBrowser(state.URL); err != nil {
			fmt.Fprintln(os.Stderr, "Open the dashboard at", state.URL)
		}
	}
	if runtime.GOOS == "windows" {
		// Encoded script avoids temporary executable scripts and shell interpolation.
		return runWindowsDesktop(config)
	}
	if _, err := exec.LookPath("zenity"); err != nil {
		return fmt.Errorf("desktop control needs Zenity; dashboard: %s", state.URL)
	}
	exe, _ := os.Executable()
	for {
		var status strings.Builder
		status.WriteString("DayZ Bank Viewer " + version + "\n")
		boot, _ := exec.Command("systemctl", "is-enabled", "dayz-bank-viewer.service").Output()
		startup := strings.TrimSpace(string(boot))
		if !managedInstall(config) {
			startup = "Not installed (portable copy)"
		} else if startup == "" || startup == "not-found" {
			startup = "Unavailable"
		}
		status.WriteString("Automatic startup: " + startup + "\n")
		if current, e := readInstance(config); e == nil {
			address, _ := url.Parse(current.URL)
			status.WriteString("Running\nPort: " + address.Port() + "\nDashboard: " + current.URL)
		} else {
			status.WriteString("Stopped or unavailable")
		}
		iconPath := filepath.Join(filepath.Dir(exe), "web", "app.ico")
		if managedInstall(config) {
			iconPath = "/usr/share/icons/hicolor/scalable/apps/dayz-bank-viewer.svg"
		}
		options := []string{"--list", "--window-icon=" + iconPath, "--no-markup", "--title=DayZ Bank Viewer", "--width=530", "--height=570", "--text=" + status.String(), "--column=Action", "Open dashboard", "Start viewer", "Stop viewer", "Restart viewer", "Check for updates", "Start automatically when the computer starts", "Disable automatic startup", "About and licence"}
		cmd := exec.Command("zenity", options...)
		raw, e := cmd.Output()
		if e != nil {
			return nil
		}
		choice := strings.TrimSpace(string(raw))
		var actionErr error
		switch choice {
		case "Open dashboard":
			current, _, e := ensureRunning(config)
			actionErr = e
			if e == nil {
				actionErr = openBrowser(current.URL)
			}
		case "Start viewer":
			actionErr = controlAction(config, "start")
		case "Stop viewer":
			actionErr = controlAction(config, "stop")
		case "Restart viewer":
			actionErr = controlAction(config, "restart")
		case "Check for updates":
			result := manualUpdate(context.Background())
			message := result.Message
			if result.URL != "" {
				if exec.Command("zenity", "--question", "--text="+message+"\nOpen the official release page?").Run() == nil {
					actionErr = openBrowser(result.URL)
				}
			} else {
				exec.Command("zenity", "--info", "--text="+message).Run()
			}
		case "Start automatically when the computer starts":
			if exec.Command("zenity", "--question", "--text=Enable the installed system service at boot? This changes system settings and requires administrator permission.").Run() == nil {
				actionErr = controlAction(config, "enable")
			}
		case "Disable automatic startup":
			actionErr = controlAction(config, "disable")
		case "About and licence":
			exec.Command("zenity", "--info", "--title=About DayZ Bank Viewer", "--text=DayZ Bank Viewer "+version+"\nA Wolf Forge Project 🐾\nRead-only local banking dashboard.\nGNU Affero General Public License v3 only.\n"+`<a href="https://wolf-forge.co.uk/">Wolf Forge</a>`+"\n"+`<a href="https://github.com/DrakonWolfDev/dayz-bank-viewer/blob/main/LICENSE">Licence (AGPL-3.0-only)</a>`).Run()
		}
		if actionErr != nil {
			exec.Command("zenity", "--error", "--text="+actionErr.Error()).Run()
		}
	}
}
func waitManaged(config string) (instance, bool, error) {
	if err := nativeService("start"); err != nil {
		return instance{}, false, err
	}
	for i := 0; i < 100; i++ {
		if state, err := readInstance(config); err == nil {
			return state, true, nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return instance{}, false, fmt.Errorf("service did not become ready; check the system service log")
}
