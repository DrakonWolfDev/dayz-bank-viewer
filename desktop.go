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
	if action == "enable" {
		return enableUnattended(config)
	}
	if action == "disable" {
		return disableUnattended(config)
	}
	config = effectiveConfig(config)
	switch action {
	case "dashboard":
		state, err := readInstance(config)
		if err != nil {
			return err
		}
		fmt.Println(state.URL)
		return nil
	case "status":
		state, err := readInstance(config)
		if err != nil {
			fmt.Println("Stopped or unavailable")
			return nil
		}
		address, _ := url.Parse(state.URL)
		mode := "Running as you"
		if managedInstall(config) {
			mode = "Unattended service"
		}
		fmt.Println(mode)
		if state.Identity != "" {
			fmt.Println("Account:", state.Identity)
		}
		fmt.Printf("Running · version %s\nPort: %s\nDashboard: %s\n", state.Version, address.Port(), state.URL)
		return nil
	case "start":
		_, _, err := ensureRunning(effectiveConfig(config))
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
		_, _, err := ensureRunning(effectiveConfig(config))
		return err
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
func desktopWindow(config string) error {
	if err := prepareDesktop(config); err != nil {
		return err
	}
	state, created, err := ensureRunning(effectiveConfig(config))
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
		if managedInstall(effectiveConfig(config)) {
			status.WriteString("Mode: unattended service\n")
		} else {
			status.WriteString("Mode: running as you\n")
		}
		boot, _ := exec.Command("systemctl", "is-enabled", "dayz-bank-viewer.service").Output()
		startup := strings.TrimSpace(string(boot))
		if !managedInstall(effectiveConfig(config)) {
			startup = "Off — running as you"
		} else if startup == "" || startup == "not-found" {
			startup = "Unavailable"
		}
		status.WriteString("Automatic startup: " + startup + "\n")
		if current, e := readInstance(effectiveConfig(config)); e == nil {
			address, _ := url.Parse(current.URL)
			status.WriteString("Running\nPort: " + address.Port() + "\nDashboard: " + current.URL)
		} else {
			status.WriteString("Stopped or unavailable")
		}
		iconPath := filepath.Join(filepath.Dir(exe), "web", "app.ico")
		if installedBinary(exe) {
			iconPath = "/usr/share/icons/hicolor/scalable/apps/dayz-bank-viewer.svg"
		}
		options := []string{"--list", "--window-icon=" + iconPath, "--no-markup", "--title=DayZ Bank Viewer", "--width=530", "--height=570", "--text=" + status.String(), "--column=Action", "Open dashboard", "Start viewer", "Stop viewer", "Restart viewer", "Check for updates"}
		if managedInstall(effectiveConfig(config)) {
			options = append(options, "Switch back to desktop mode")
		} else {
			options = append(options, "Run without signing in…")
		}
		options = append(options, "About and licence")
		cmd := dialogCommand(options...)
		raw, e := cmd.Output()
		if e != nil {
			return nil
		}
		choice := strings.TrimSpace(string(raw))
		var actionErr error
		switch choice {
		case "Open dashboard":
			current, _, e := ensureRunning(effectiveConfig(config))
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
				if dialogCommand("--question", "--text="+message+"\nOpen the official release page?").Run() == nil {
					actionErr = openBrowser(result.URL)
				}
			} else {
				dialogCommand("--info", "--text="+message).Run()
			}
		case "Run without signing in…":
			actionErr = controlAction(config, "enable")
		case "Switch back to desktop mode":
			actionErr = controlAction(config, "disable")
		case "About and licence":
			dialogCommand("--info", "--title=About DayZ Bank Viewer", "--text=DayZ Bank Viewer "+version+"\nA Wolf Forge Project 🐾\nRead-only local banking dashboard.\nGNU Affero General Public License v3 only.\n"+`<a href="https://wolf-forge.co.uk/">Wolf Forge</a>`+"\n"+`<a href="https://github.com/DrakonWolfDev/dayz-bank-viewer/blob/main/LICENSE">Licence (AGPL-3.0-only)</a>`).Run()
		}
		if actionErr != nil {
			dialogCommand("--error", "--text="+actionErr.Error()).Run()
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

func dialogCommand(args ...string) *exec.Cmd {
	cmd := exec.Command("zenity", args...)
	prepareDialog(cmd)
	return cmd
}
