package main

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
)

func TestPowerShellReportsHiddenFailure(t *testing.T) {
	err := powershell("throw 'synthetic-launch-failure'", nil)
	if err == nil || !strings.Contains(err.Error(), "synthetic-launch-failure") {
		t.Fatalf("expected captured PowerShell diagnostic, got %v", err)
	}
}

func TestPowerShellPreservesModeChoiceExitCode(t *testing.T) {
	err := powershell("exit 10", nil)
	var exitError *exec.ExitError
	if !errors.As(err, &exitError) || exitError.ExitCode() != 10 {
		t.Fatalf("expected wrapped exit code 10, got %v", err)
	}
}
