package main

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
)

// Compress the embedded UI to stay below Windows' command-line limit as the
// desktop grows. The bootstrap uses only built-in .NET and PowerShell 5.1 APIs.
func desktopPowerShellScript() (string, error) {
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	if _, err := writer.Write([]byte(windowsDesktop)); err != nil {
		return "", err
	}
	if err := writer.Close(); err != nil {
		return "", err
	}
	payload := base64.StdEncoding.EncodeToString(compressed.Bytes())
	return fmt.Sprintf(`$b=[Convert]::FromBase64String('%s');
$m=[IO.MemoryStream]::new($b,$false);
$z=[IO.Compression.GZipStream]::new($m,[IO.Compression.CompressionMode]::Decompress);
$r=[IO.StreamReader]::new($z,[Text.Encoding]::UTF8);
try { & ([scriptblock]::Create($r.ReadToEnd())) } finally { $r.Dispose(); $z.Dispose(); $m.Dispose() }`, payload), nil
}
