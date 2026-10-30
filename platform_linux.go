package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var portInUse = syscall.EADDRINUSE

func acquireLock(path string) (func(), error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, fmt.Errorf("viewer is already running for this configuration")
	}
	return func() { syscall.Flock(int(f.Fd()), syscall.LOCK_UN); f.Close() }, nil
}
func detach(cmd *exec.Cmd)               { cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true} }
func openBrowser(url string) error       { return exec.Command("xdg-open", url).Start() }
func serviceHost(run func() error) error { return run() }
func nativeService(action string) error {
	if action == "desktop" {
		return elevatedSystemctl("disable", "--now")
	}
	return elevatedSystemctl(action)
}
func elevatedSystemctl(actions ...string) error {
	args := append([]string{"systemctl"}, actions...)
	args = append(args, "dayz-bank-viewer.service")
	cmd := exec.Command("pkexec", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("service operation failed: %w: %s", err, output)
	}
	return nil
}
func nativeServiceDirect(action string) error {
	args := map[string][]string{"start": {"start"}, "stop": {"stop"}, "restart": {"restart"}, "enable": {"enable"}, "disable": {"disable"}}
	values, ok := args[action]
	if !ok {
		return fmt.Errorf("unknown service action")
	}
	cmd := exec.Command("systemctl", append(values, "dayz-bank-viewer.service")...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runWindowsDesktop(config string) error {
	return fmt.Errorf("Windows desktop is unavailable on Linux")
}

func secureRuntime(path string) error { return os.Chmod(path, 0600) }
func nativeError(message string) {
	dialogCommand("--error", "--no-markup", "--text="+message).Run()
}

func nativeServiceEnabled() bool {
	return exec.Command("systemctl", "is-enabled", "--quiet", "dayz-bank-viewer.service").Run() == nil
}
func serviceAdministrator() bool { return os.Geteuid() == 0 }
func ownServiceConfig(path string) error {
	u, err := user.Lookup("dayz-bank-viewer")
	if err != nil {
		return err
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return err
	}
	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		return err
	}
	return os.Chown(path, uid, gid)
}
func setupNativeService(request string) error {
	exe, _ := os.Executable()
	cmd := exec.Command("pkexec", exe, "-service-setup", request)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s\nService setup failed: %w", output, err)
	}
	return nil
}
func confirmServiceSetup(text string) bool {
	return dialogCommand("--question", "--no-markup", "--width=600", "--title=Unattended startup", "--text="+text).Run() == nil
}
func choosePreviousMode() (string, error) {
	cmd := dialogCommand("--list", "--radiolist", "--title=Choose how to run", "--width=600", "--height=310", "--text=Your previous build used a system service. Desktop mode can use your existing settings and read files as you. Switching stops unattended startup and may request administrator permission.", "--column=Choose", "--column=Mode", "TRUE", "Run as me (recommended)", "FALSE", "Keep the unattended service")
	raw, err := cmd.Output()
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(string(raw), "Run as me") {
		return "desktop", nil
	}
	return "service", nil
}

func prepareDialog(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGTERM} }
func stopPackagedProcesses() error {
	if !serviceAdministrator() {
		return fmt.Errorf("package lifecycle operations require administrator permission")
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if exe != "/usr/bin/dayz-bank-viewer" {
		return fmt.Errorf("package lifecycle operations apply only to the installed executable")
	}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return err
	}
	var pids []int
	for _, entry := range entries {
		pid, e := strconv.Atoi(entry.Name())
		if e != nil || pid == os.Getpid() {
			continue
		}
		target, e := os.Readlink("/proc/" + entry.Name() + "/exe")
		if e != nil || strings.TrimSuffix(target, " (deleted)") != exe {
			continue
		}
		if e = syscall.Kill(pid, syscall.SIGTERM); e == nil {
			pids = append(pids, pid)
		} else if e != syscall.ESRCH {
			return e
		}
	}
	for i := 0; i < 100; i++ {
		remaining := false
		for _, pid := range pids {
			target, e := os.Readlink(fmt.Sprintf("/proc/%d/exe", pid))
			if e == nil && strings.TrimSuffix(target, " (deleted)") == exe {
				remaining = true
			}
		}
		if !remaining {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("a viewer process is still running; close it and retry the package operation")
}
