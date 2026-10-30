package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type ServerConfig struct {
	AccountType string `json:"account_type,omitempty"`
	ID          string `json:"id"`
	Name        string `json:"name"`
	Path        string `json:"path"`
	Enabled     bool   `json:"enabled"`
	LogPath     string `json:"log_path,omitempty"`
}
type Config struct {
	Listen       string         `json:"listen"`
	Servers      []ServerConfig `json:"servers"`
	RefreshTimes []string       `json:"refresh_times"`
}

func defaultConfig() Config {
	return Config{Listen: "127.0.0.1:20784", Servers: []ServerConfig{}, RefreshTimes: []string{}}
}
func newID() string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}

func validateConfig(c *Config, configPath string) error {
	if c.Listen == "" {
		c.Listen = defaultConfig().Listen
	}
	_, port, err := net.SplitHostPort(c.Listen)
	if err != nil {
		return errors.New("listen must be a host:port address, for example 127.0.0.1:20784")
	}
	p, err := strconv.Atoi(port)
	if err != nil || p < 1 || p > 65535 {
		return errors.New("listen port must be between 1 and 65535")
	}
	ids := map[string]bool{}
	configAbs, err := filepath.Abs(configPath)
	if err != nil {
		return err
	}
	for i := range c.Servers {
		s := &c.Servers[i]
		s.Name = strings.TrimSpace(s.Name)
		s.Path = strings.TrimSpace(s.Path)
		if s.Name == "" || len(s.Name) > 120 {
			return errors.New("each server needs a name of 1–120 bytes")
		}
		if !filepath.IsAbs(s.Path) {
			return fmt.Errorf("%s: use an absolute banking directory path", s.Name)
		}
		s.Path = filepath.Clean(s.Path)
		if s.AccountType != "" && s.AccountType != "player" && s.AccountType != "clan" {
			return fmt.Errorf("%s: account type must be player or clan", s.Name)
		}
		s.LogPath = strings.TrimSpace(s.LogPath)
		if s.LogPath != "" {
			if !filepath.IsAbs(s.LogPath) {
				return fmt.Errorf("%s: use an absolute admin log directory path", s.Name)
			}
			s.LogPath = filepath.Clean(s.LogPath)
		}
		if s.ID == "" {
			s.ID = newID()
		}
		if len(s.ID) > 80 || strings.ContainsAny(s.ID, "/\\") || ids[s.ID] {
			return errors.New("server IDs must be unique and must not contain slashes")
		}
		ids[s.ID] = true
		rel, err := filepath.Rel(s.Path, configAbs)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return errors.New("keep the viewer configuration outside every banking directory")
		}
	}
	times := map[string]bool{}
	for _, s := range c.RefreshTimes {
		t, err := time.Parse("15:04", s)
		if err != nil || t.Format("15:04") != s {
			return errors.New("refresh times must use 24-hour HH:MM, for example 06:00")
		}
		if times[s] {
			return errors.New("refresh times must be unique")
		}
		times[s] = true
	}
	if c.Servers == nil {
		c.Servers = []ServerConfig{}
	}
	if c.RefreshTimes == nil {
		c.RefreshTimes = []string{}
	}
	return nil
}
func loadConfig(path string) (Config, error) {
	c := defaultConfig()
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	if !strings.HasPrefix(strings.TrimSpace(string(b)), "{") {
		return c, errors.New("configuration must be a JSON object")
	}
	// Refuse to overwrite a player file accidentally passed as -config.
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		return c, fmt.Errorf("invalid configuration: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return c, errors.New("configuration must contain exactly one JSON object")
	}
	if err := validateConfig(&c, path); err != nil {
		return c, err
	}
	return c, nil
}

func saveConfig(path string, c Config) error {
	if err := validateConfig(&c, path); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".dayz-bank-config-*")
	if err != nil {
		return err
	}
	temp := f.Name()
	defer os.Remove(temp)
	if _, err = f.Write(append(data, '\n')); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(temp, path)
}
