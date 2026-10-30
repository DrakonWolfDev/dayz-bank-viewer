package main

import (
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

var portInUse = syscall.Errno(10048)

func acquireLock(path string) (func(), error) {
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	handle, err := syscall.CreateFile(name, syscall.GENERIC_READ|syscall.GENERIC_WRITE, 0, nil, syscall.OPEN_ALWAYS, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, fmt.Errorf("viewer is already running or its configuration is inaccessible: %w", err)
	}
	return func() { syscall.CloseHandle(handle) }, nil
}
func detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x00000200 | 0x08000000}
}
func openBrowser(url string) error {
	return exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", url).Start()
}
func encodedPowerShell(script string) string {
	chars := utf16.Encode([]rune(script))
	raw := make([]byte, len(chars)*2)
	for i, c := range chars {
		binary.LittleEndian.PutUint16(raw[i*2:], c)
	}
	return base64.StdEncoding.EncodeToString(raw)
}
func powershell(script string, env []string) error {
	cmd := exec.Command("powershell.exe", "-NoProfile", "-STA", "-NonInteractive", "-EncodedCommand", encodedPowerShell(script))
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
func runWindowsDesktop(config string) error {
	exe, _ := os.Executable()
	return powershell(windowsDesktop, []string{"VIEWER_EXE=" + exe, "VIEWER_CONFIG=" + config, "VIEWER_VERSION=" + version, fmt.Sprint("VIEWER_PARENT_PID=", os.Getpid())})
}
func serviceCommand(action string) (string, error) {
	commands := map[string]string{"start": "Start-Service DayZBankViewer", "stop": "Stop-Service DayZBankViewer", "restart": "Restart-Service DayZBankViewer", "enable": "Set-Service DayZBankViewer -StartupType Automatic", "disable": "Set-Service DayZBankViewer -StartupType Manual", "desktop": "Set-Service DayZBankViewer -StartupType Manual; Stop-Service DayZBankViewer"}
	command, ok := commands[action]
	if !ok {
		return "", fmt.Errorf("unknown service action")
	}
	return command, nil
}
func nativeServiceDirect(action string) error {
	command, err := serviceCommand(action)
	if err != nil {
		return err
	}
	return powershell("$ErrorActionPreference='Stop'; "+command, nil)
}
func nativeService(action string) error {
	command, err := serviceCommand(action)
	if err != nil {
		return err
	}
	// Native UAC prompt; no credentials or commands interpolated from external input.
	script := encodedPowerShell("$ErrorActionPreference='Stop'; " + command)
	return powershell("$ErrorActionPreference='Stop'; $p=Start-Process powershell.exe -Verb RunAs -Wait -PassThru -ArgumentList '-NoProfile', '-EncodedCommand', '"+script+"'; if($p.ExitCode -ne 0){throw 'Service operation failed'}", nil)
}

// Native SCM host: service startup never creates a desktop or browser.
func serviceHost(run func() error) error {
	dll := syscall.NewLazyDLL("advapi32.dll")
	register := dll.NewProc("RegisterServiceCtrlHandlerExW")
	set := dll.NewProc("SetServiceStatus")
	name, _ := syscall.UTF16PtrFromString("DayZBankViewer")
	type status struct{ Kind, State, Accepted, Win32Exit, SpecificExit, Checkpoint, WaitHint uint32 }
	var handle uintptr
	stopped := make(chan struct{}, 1)
	serviceStop = stopped
	report := func(state, accepted, exit uint32) {
		value := status{Kind: 16, State: state, Accepted: accepted, Win32Exit: exit, WaitHint: 10000}
		set.Call(handle, uintptr(unsafe.Pointer(&value)))
	}
	callback := syscall.NewCallback(func(control, event uint32, data, ctx uintptr) uintptr {
		if control == 1 || control == 5 {
			select {
			case stopped <- struct{}{}:
			default:
			}
			report(3, 0, 0)
		}
		return 0
	})
	var failure error
	mainCallback := syscall.NewCallback(func(argc uint32, argv uintptr) uintptr {
		var e error
		handle, _, e = register.Call(uintptr(unsafe.Pointer(name)), callback, 0)
		if handle == 0 {
			failure = e
			return 0
		}
		report(2, 0, 0)
		serviceReady = func() { report(4, 1|4, 0) }
		failure = run()
		if failure != nil {
			report(1, 0, 1)
		} else {
			report(1, 0, 0)
		}
		return 0
	})
	type entry struct {
		Name     *uint16
		Callback uintptr
	}
	table := [2]entry{{name, mainCallback}, {nil, 0}}
	ok, _, err := dll.NewProc("StartServiceCtrlDispatcherW").Call(uintptr(unsafe.Pointer(&table[0])))
	if ok == 0 {
		return err
	}
	return failure
}

func secureRuntime(path string) error {
	current, err := user.Current()
	if err != nil {
		return err
	}
	cmd := exec.Command("icacls.exe", path, "/inheritance:r", "/grant:r", "*"+current.Uid+":F", "*S-1-5-18:F", "*S-1-5-32-544:F")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("could not protect service control credentials: %w: %s", err, output)
	}
	return nil
}
func nativeError(message string) {
	powershell("Add-Type -AssemblyName System.Windows.Forms; [System.Windows.Forms.MessageBox]::Show($env:VIEWER_ERROR,'DayZ Bank Viewer') | Out-Null", []string{"VIEWER_ERROR=" + message})
}

func nativeServiceEnabled() bool {
	return powershell("$s=Get-Service DayZBankViewer -ErrorAction SilentlyContinue; if(!$s -or $s.StartType -ne 'Automatic'){exit 1}", nil) == nil
}
func serviceAdministrator() bool {
	ok, _, _ := syscall.NewLazyDLL("shell32.dll").NewProc("IsUserAnAdmin").Call()
	return ok != 0
}
func ownServiceConfig(path string) error {
	output, err := exec.Command("icacls.exe", path, "/inheritance:r", "/grant:r", "*S-1-5-19:M", "*S-1-5-18:F", "*S-1-5-32-544:F").CombinedOutput()
	if err != nil {
		return fmt.Errorf("could not grant service access to its settings: %w: %s", err, output)
	}
	return nil
}
func setupNativeService(request string) error {
	exe, _ := os.Executable()
	return powershell(`$ErrorActionPreference='Stop'; $setupArguments='-service-setup "'+$env:VIEWER_REQUEST+'"'; $p=Start-Process -FilePath $env:VIEWER_EXE -ArgumentList $setupArguments -Verb RunAs -Wait -PassThru; if($p.ExitCode -ne 0){throw 'Unattended startup was not enabled. Desktop mode remains available.'}`, []string{"VIEWER_REQUEST=" + request, "VIEWER_EXE=" + exe})
}
func confirmServiceSetup(text string) bool {
	return powershell(`Add-Type -AssemblyName System.Windows.Forms; if([System.Windows.Forms.MessageBox]::Show($env:VIEWER_SETUP_TEXT,'Unattended startup','YesNo') -ne 'Yes'){exit 1}`, []string{"VIEWER_SETUP_TEXT=" + text}) == nil
}
func choosePreviousMode() (string, error) {
	err := powershell(`Add-Type -AssemblyName System.Windows.Forms; $choice=[System.Windows.Forms.MessageBox]::Show('Your previous build used a system service. Choose Yes to run as you (recommended), using your existing settings. This stops unattended startup and may request administrator permission. Choose No to keep the unattended service, or Cancel to leave everything unchanged.','Choose how to run','YesNoCancel'); if($choice -eq 'No'){exit 10}; if($choice -eq 'Cancel'){exit 20}`, nil)
	if err == nil {
		return "desktop", nil
	}
	if e, ok := err.(*exec.ExitError); ok && e.ExitCode() == 10 {
		return "service", nil
	}
	return "", fmt.Errorf("desktop launch cancelled")
}
func prepareDialog(cmd *exec.Cmd) {}
func stopPackagedProcesses() error {
	return fmt.Errorf("use the Windows installer to stop installed viewer processes")
}
