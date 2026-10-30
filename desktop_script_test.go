package main

import (
	"compress/gzip"
	"encoding/base64"
	"io"
	"regexp"
	"strings"
	"testing"
	"unicode/utf16"
)

func TestWindowsDesktopFitsCommandLine(t *testing.T) {
	script, err := desktopPowerShellScript()
	if err != nil {
		t.Fatal(err)
	}
	// Reserve room for powershell.exe, switches and NUL within CreateProcess's limit.
	encodedLength := base64.StdEncoding.EncodedLen(len(utf16.Encode([]rune(script))) * 2)
	if encodedLength+512 > 32767 {
		t.Fatalf("desktop script needs %d encoded characters; exceeds the Windows launcher budget", encodedLength)
	}
}

func TestWindowsDesktopPayloadRoundTrip(t *testing.T) {
	script, err := desktopPowerShellScript()
	if err != nil {
		t.Fatal(err)
	}
	match := regexp.MustCompile(`FromBase64String\('([^']+)'\)`).FindStringSubmatch(script)
	if len(match) != 2 {
		t.Fatal("bootstrap is missing the embedded desktop payload")
	}
	reader, err := gzip.NewReader(base64.NewDecoder(base64.StdEncoding, strings.NewReader(match[1])))
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	decoded, err := io.ReadAll(reader)
	if err != nil || string(decoded) != windowsDesktop {
		t.Fatalf("desktop payload did not survive compression: %v", err)
	}
}
