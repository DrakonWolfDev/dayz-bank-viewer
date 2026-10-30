package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

const maxDiscoveryEntries = 10000
const maxDiscoveryConfigFiles = 64

var profileDirectoryNames = map[string]bool{"profiles": true, "profile": true, "config": true, "sc": true}
var serverConfigNames = map[string]bool{"serverdz.cfg": true, "server.cfg": true, "server.dz": true}

type DetectedMission struct {
	ConfigPath string `json:"config_path"`
	Template   string `json:"template"`
	Map        string `json:"map"`
}
type DetectedSource struct {
	AlreadyConfigured bool     `json:"already_configured"`
	Path              string   `json:"path"`
	Profile           string   `json:"profile"`
	AccountType       string   `json:"account_type"`
	Format            string   `json:"format"`
	Label             string   `json:"label"`
	Verified          bool     `json:"verified"`
	Note              string   `json:"note"`
	LogDirectories    []string `json:"log_directories"`
}
type Discovery struct {
	Root           string            `json:"root"`
	Missions       []DetectedMission `json:"missions"`
	Sources        []DetectedSource  `json:"sources"`
	LogDirectories []string          `json:"log_directories"`
	Warnings       []string          `json:"warnings"`
}

// Inspect only the selected directory, named profile children and known storage
// paths. Do not recurse through backups, mods, other servers or symlinked paths.
func discoverServer(ctx context.Context, root, configPath string) (Discovery, error) {
	d := Discovery{Missions: []DetectedMission{}, Sources: []DetectedSource{}, LogDirectories: []string{}, Warnings: []string{}}
	root = strings.TrimSpace(root)
	if !filepath.IsAbs(root) {
		return d, errors.New("use an absolute server directory path")
	}
	root = filepath.Clean(root)
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		return d, errors.New("cannot open server directory")
	}
	root = resolved
	d.Root = root
	entries, err := discoveryEntries(ctx, root)
	if err != nil {
		return d, fmt.Errorf("cannot read server directory: %w", err)
	}
	profiles := []string{root}
	for _, e := range entries {
		if profileDirectoryNames[strings.ToLower(e.Name())] && e.IsDir() && e.Type()&os.ModeSymlink == 0 {
			profiles = append(profiles, filepath.Join(root, e.Name()))
		}
	}
	configFiles := []string{}
	for _, profile := range profiles {
		es, err := discoveryEntries(ctx, profile)
		if err != nil {
			d.Warnings = append(d.Warnings, "Could not inspect "+profile)
			continue
		}
		for _, e := range es {
			if isServerConfigCandidate(e.Name()) && e.Type().IsRegular() {
				configFiles = append(configFiles, filepath.Join(profile, e.Name()))
			}
		}
		if containsADM(es) {
			d.LogDirectories = append(d.LogDirectories, profile)
		}
		for _, path := range knownDirectories(ctx, profile, "log_storage") {
			d.LogDirectories = append(d.LogDirectories, path)
		}
	}
	if configPath != "" {
		configPath = strings.TrimSpace(configPath)
		if !filepath.IsAbs(configPath) {
			return d, errors.New("use an absolute server configuration file path")
		}
		configPath = filepath.Clean(configPath)
		rel, err := filepath.Rel(root, configPath)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return d, errors.New("choose a configuration file inside the selected server directory")
		}
		// Reject links in every path component, not just the file itself.
		resolved, err := filepath.EvalSymlinks(configPath)
		if err != nil || resolved != configPath {
			return d, errors.New("server configuration must be a regular file without symlinked components")
		}
		configFiles = []string{configPath}
	}
	// Keep conventional names first when applying the read bound, but inspect
	// other configuration filenames as well: launchers may use any filename.
	sort.Slice(configFiles, func(i, j int) bool {
		a, b := serverConfigNames[strings.ToLower(filepath.Base(configFiles[i]))], serverConfigNames[strings.ToLower(filepath.Base(configFiles[j]))]
		if a != b {
			return a
		}
		return configFiles[i] < configFiles[j]
	})
	if len(configFiles) > maxDiscoveryConfigFiles {
		d.Warnings = append(d.Warnings, fmt.Sprintf("Configuration search limited to %d files. Use the optional override for a file outside this selection.", maxDiscoveryConfigFiles))
		configFiles = configFiles[:maxDiscoveryConfigFiles]
	}
	for _, path := range configFiles {
		if err := ctx.Err(); err != nil {
			return d, err
		}
		data, err := readDiscoveryFile(path, 1<<20)
		if err != nil {
			d.Warnings = append(d.Warnings, "Could not read server configuration: "+path)
			continue
		}
		missions, err := parseMissionTemplates(string(data))
		if err != nil {
			d.Warnings = append(d.Warnings, "Could not parse mission settings in "+path+": "+err.Error())
			continue
		}
		for _, mission := range missions {
			d.Missions = append(d.Missions, DetectedMission{ConfigPath: path, Template: mission, Map: missionMap(mission)})
		}
	}
	if len(d.Missions) == 0 {
		d.Warnings = append(d.Warnings, "No mission template found in the server configuration. You can supply a custom configuration file or enter a server name below.")
	}
	if len(d.Missions) > 1 {
		d.Warnings = append(d.Warnings, "Several mission settings were found. Choose the configuration and mission you want; discovery cannot determine which launch command is in use.")
	}
	layouts := []struct {
		parts               []string
		kind, format, label string
	}{
		{[]string{"Banking", "PlayerDataBase"}, "player", "banking", "Banking / KR Banking compatible"},
		{[]string{"KR_BANKING", "PlayerDataBase"}, "player", "banking", "Banking / KR Banking compatible"},
		{[]string{"DC_Banking", "PlayerDatabase"}, "player", "dc_banking", "DayZ-Banking (DC Banking)"},
		{[]string{"ExpansionMod", "ATM"}, "player", "expansion", "Expansion ATM"},
		{[]string{"KR_BANKING", "ClanDataBase"}, "clan", "kr_clan", "KR Banking clans"},
		{[]string{"ExpansionMod", "Groups"}, "clan", "expansion_party", "Expansion parties"},
	}
	for _, profile := range profiles {
		for _, layout := range layouts {
			for _, path := range knownDirectories(ctx, profile, layout.parts...) {
				if err := ctx.Err(); err != nil {
					return d, err
				}
				candidate := DetectedSource{Path: path, Profile: profile, AccountType: layout.kind, Format: layout.format, Label: layout.label, LogDirectories: []string{}}
				var reader AccountReader = AutoBankingReader{}
				if layout.kind == "clan" {
					reader = AutoClanReader{}
				}
				files, err := discoveryEntries(ctx, path)
				if err != nil {
					candidate.Note = "Folder could not be read; check access before adding it."
				} else {
					attempts := 0
					foundFile := false
					for _, e := range files {
						ext := strings.ToLower(filepath.Ext(e.Name()))
						if !e.Type().IsRegular() || (ext != ".json" && !(layout.kind == "clan" && ext == ".bin")) {
							continue
						}
						foundFile = true
						if attempts >= 8 {
							break
						}
						attempts++
						if ctx.Err() != nil {
							return d, ctx.Err()
						}
						data, err := readDiscoveryFile(filepath.Join(path, e.Name()), 1<<20)
						if err != nil {
							continue
						}
						account, err := reader.Read(strings.NewReader(string(data)))
						if err == nil && account.Format == layout.format {
							candidate.Verified = true
							break
						}
					}
					if candidate.Verified {
						candidate.Note = "Recognised account sample. A full scan runs after saving."
					} else if foundFile {
						candidate.Note = "No supported account was recognised in the first 8 eligible files. Check this folder before adding it."
					} else {
						candidate.Note = "No eligible account files found. Leave unchecked unless this is the folder you want."
					}
				}
				if layout.format == "expansion" {
					candidate.LogDirectories = adminLogDirectories(ServerConfig{Path: path})
				}
				d.Sources = append(d.Sources, candidate)
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return d, err
	}
	return d, nil
}

func discoveryEntries(ctx context.Context, path string) ([]os.DirEntry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	entries, err := f.ReadDir(maxDiscoveryEntries + 1)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	if len(entries) > maxDiscoveryEntries {
		return nil, fmt.Errorf("directory exceeds discovery limit of %d entries", maxDiscoveryEntries)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	return entries, nil
}
func knownDirectories(ctx context.Context, root string, parts ...string) []string {
	if len(parts) == 0 {
		return []string{root}
	}
	es, err := discoveryEntries(ctx, root)
	if err != nil {
		return nil
	}
	out := []string{}
	for _, e := range es {
		if strings.EqualFold(e.Name(), parts[0]) && e.IsDir() && e.Type()&os.ModeSymlink == 0 {
			out = append(out, knownDirectories(ctx, filepath.Join(root, e.Name()), parts[1:]...)...)
		}
	}
	return out
}
func containsADM(entries []os.DirEntry) bool {
	for _, e := range entries {
		if e.Type().IsRegular() && strings.EqualFold(filepath.Ext(e.Name()), ".adm") {
			return true
		}
	}
	return false
}
func readDiscoveryFile(path string, limit int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("not a regular file")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errors.New("file exceeds discovery read limit")
	}
	return data, nil
}

func missionMap(template string) string {
	suffix := template
	if i := strings.LastIndex(template, "."); i >= 0 {
		suffix = template[i+1:]
	}
	names := map[string]string{"chernarusplus": "Chernarus", "enoch": "Livonia", "sakhal": "Sakhal", "namalsk": "Namalsk", "deerisle": "Deer Isle", "takistanplus": "Takistan"}
	if name := names[strings.ToLower(suffix)]; name != "" {
		return name
	}
	return suffix
}

type configToken struct {
	text   string
	quoted bool
}

func configTokens(data string) ([]configToken, error) {
	if !utf8.ValidString(data) {
		return nil, errors.New("configuration is not UTF-8")
	}
	data = strings.TrimPrefix(data, "\ufeff")
	tokens := []configToken{}
	for i := 0; i < len(data); {
		c := data[i]
		if c == ' ' || c == '\t' || c == '\r' || c == '\n' {
			i++
			continue
		}
		if i+1 < len(data) && data[i:i+2] == "//" {
			for i < len(data) && data[i] != '\n' {
				i++
			}
			continue
		}
		if i+1 < len(data) && data[i:i+2] == "/*" {
			end := strings.Index(data[i+2:], "*/")
			if end < 0 {
				return nil, errors.New("unterminated comment")
			}
			i += end + 4
			continue
		}
		if c == '#' {
			return nil, errors.New("preprocessor directives need manual configuration review")
		}
		if c == '"' {
			i++
			var b strings.Builder
			closed := false
			for i < len(data) {
				if data[i] == '"' {
					i++
					closed = true
					break
				}
				if data[i] == '\\' && i+1 < len(data) && (data[i+1] == '"' || data[i+1] == '\\') {
					i++
				}
				b.WriteByte(data[i])
				i++
			}
			if !closed {
				return nil, errors.New("unterminated string")
			}
			tokens = append(tokens, configToken{b.String(), true})
			continue
		}
		if strings.ContainsRune("{};=:", rune(c)) {
			tokens = append(tokens, configToken{string(c), false})
			i++
			continue
		}
		start := i
		for i < len(data) && !strings.ContainsRune(" \t\r\n{};=:\"", rune(data[i])) {
			if i+1 < len(data) && (data[i:i+2] == "//" || data[i:i+2] == "/*") {
				break
			}
			i++
		}
		if i == start {
			return nil, errors.New("unexpected configuration token")
		}
		tokens = append(tokens, configToken{data[start:i], false})
	}
	return tokens, nil
}
func parseMissionTemplates(data string) ([]string, error) {
	tokens, err := configTokens(data)
	if err != nil {
		return nil, err
	}
	scopes := []string{}
	pending := ""
	missions := []string{}
	seen := map[string]bool{}
	for i, t := range tokens {
		if t.quoted {
			continue
		}
		if strings.EqualFold(t.text, "class") && i+1 < len(tokens) && !tokens[i+1].quoted {
			pending = tokens[i+1].text
		}
		switch t.text {
		case "{":
			scopes = append(scopes, pending)
			pending = ""
		case "}":
			if len(scopes) == 0 {
				return nil, errors.New("unbalanced braces")
			}
			scopes = scopes[:len(scopes)-1]
			pending = ""
		case ";":
			pending = ""
		}
		inMissions := false
		for _, scope := range scopes {
			if strings.EqualFold(scope, "Missions") {
				inMissions = true
			}
		}
		if inMissions && strings.EqualFold(t.text, "template") && i+3 < len(tokens) && tokens[i+1].text == "=" && tokens[i+2].quoted && tokens[i+3].text == ";" {
			value := tokens[i+2].text
			if len(value) == 0 || len(value) > 160 || strings.IndexFunc(value, func(r rune) bool {
				return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '_' || r == '-')
			}) >= 0 {
				return nil, errors.New("invalid mission template")
			}
			if !seen[value] {
				missions = append(missions, value)
				seen[value] = true
			}
		}
	}
	if len(scopes) > 0 {
		return nil, errors.New("unbalanced braces")
	}
	return missions, nil
}

// Directory names suggest where to look; a Missions/template declaration is
// still required before a configuration can appear as a selectable mission.
func isServerConfigCandidate(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".cfg", ".dz", ".conf", ".config":
		return true
	}
	return false
}
