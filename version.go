package main

import (
	_ "embed"
	"strings"
)

//go:embed VERSION
var versionText string
var version = strings.TrimSpace(versionText)
