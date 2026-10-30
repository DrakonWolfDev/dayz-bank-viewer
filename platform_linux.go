package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
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
	args := map[string][]string{"start": {"start"}, "stop": {"stop"}, "restart": {"restart"}, "enable": {"enable"}, "disable": {"disable"}}
	values, ok := args[action]
	if !ok {
		return fmt.Errorf("unknown service action")
	}
	cmd := exec.Command("pkexec", append([]string{"systemctl"}, append(values, "dayz-bank-viewer.service")...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runWindowsDesktop(config string) error {
	return fmt.Errorf("Windows desktop is unavailable on Linux")
}

func secureRuntime(path string) error { return os.Chmod(path, 0600) }
func nativeError(message string) {
	exec.Command("zenity", "--error", "--no-markup", "--text="+message).Run()
}
