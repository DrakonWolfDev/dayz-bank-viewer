package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"
)

type folderAccess struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	Error string `json:"error,omitempty"`
}
type accessReport struct {
	Identity string         `json:"identity"`
	Folders  []folderAccess `json:"folders"`
}

func checkBankingAccess(ctx context.Context, c Config) accessReport {
	identity := "viewer account"
	if u, err := user.Current(); err == nil {
		identity = u.Username
	}
	report := accessReport{Identity: identity, Folders: []folderAccess{}}
	for _, s := range c.Servers {
		if !s.Enabled {
			continue
		}
		item := folderAccess{Name: s.Name, Path: s.Path}
		entries, err := os.ReadDir(s.Path)
		if err != nil {
			item.Error = "Cannot open this folder. Give " + identity + " read access to it and traversal access to its parent folders."
		} else {
			for _, entry := range entries {
				if ctx.Err() != nil {
					item.Error = "Folder check was cancelled."
					break
				}
				if entry.Type()&os.ModeSymlink != 0 || entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".json") {
					continue
				}
				info, e := entry.Info()
				if e != nil {
					item.Error = "Cannot inspect one or more account files. Check read and traversal access for " + identity + "."
					break
				}
				if !info.Mode().IsRegular() {
					continue
				}
				f, e := os.Open(filepath.Join(s.Path, entry.Name()))
				if e != nil {
					item.Error = "Cannot read one or more account files. Give " + identity + " read access to existing and newly created account files."
					break
				}
				f.Close()
			}
		}
		report.Folders = append(report.Folders, item)
	}
	return report
}
func serviceAccessCheck(config string) error {
	state, _, err := waitForInstance(config)
	if err != nil {
		return err
	}
	raw, err := instanceRequest(state, "GET", "/access")
	if err != nil {
		return err
	}
	var report accessReport
	if err = json.Unmarshal(raw, &report); err != nil {
		return err
	}
	var problems []string
	for _, folder := range report.Folders {
		if folder.Error != "" {
			problems = append(problems, folder.Name+" — "+folder.Path+"\n"+folder.Error)
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("unattended startup was not enabled. Desktop mode remains available.\n\n%s\n\nAn administrator can grant the service account read-only access using folder permissions or ACLs. No source permissions were changed", strings.Join(problems, "\n\n"))
	}
	if len(report.Folders) == 0 {
		return errors.New("no enabled banking folders were found; unattended startup was not enabled")
	}
	return nil
}

type setupOperations struct {
	control func(string) error
	check   func(string) error
	own     func(string) error
	enabled func() bool
}

func configureUnattended(request string) error {
	if !serviceAdministrator() {
		return errors.New("unattended service setup requires administrator permission")
	}
	raw, err := os.ReadFile(request)
	if err != nil {
		return err
	}
	if len(raw) > 1<<20 {
		return errors.New("settings exceed 1 MiB")
	}
	var c Config
	if err = json.Unmarshal(raw, &c); err != nil {
		return err
	}
	destination := systemConfigPath()
	if err = validateConfig(&c, destination); err != nil {
		return err
	}
	return configureService(c, destination, setupOperations{nativeServiceDirect, serviceAccessCheck, ownServiceConfig, nativeServiceEnabled})
}

// Configuration is snapshotted and restored on failure; source folders are never modified.
func configureService(c Config, destination string, ops setupOperations) (result error) {
	if err := validateConfig(&c, destination); err != nil {
		return err
	}
	before, err := os.ReadFile(destination)
	existed := err == nil
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
		return err
	}
	if existed {
		backup := destination + ".before-setup-" + time.Now().UTC().Format("20060102T150405.000000000")
		if err = os.WriteFile(backup, before, 0600); err != nil {
			return err
		}
	}
	wasEnabled := ops.enabled()
	_, e := readInstance(destination)
	wasRunning := e == nil
	if err = ops.control("stop"); err != nil {
		return err
	}
	defer func() {
		if result == nil {
			return
		}
		var rollback []error
		if e := ops.control("stop"); e != nil {
			rollback = append(rollback, e)
		}
		if existed {
			// Restore through a temporary file, rather than exposing a partial configuration.
			f, e := os.CreateTemp(filepath.Dir(destination), ".restore-*")
			if e == nil {
				_, e = f.Write(before)
				closeErr := f.Close()
				if e == nil {
					e = closeErr
				}
				if e == nil {
					e = os.Rename(f.Name(), destination)
				}
				os.Remove(f.Name())
			}
			if e == nil {
				e = ops.own(destination)
			}
			if e != nil {
				rollback = append(rollback, e)
			}
		} else if e := os.Remove(destination); e != nil && !errors.Is(e, os.ErrNotExist) {
			rollback = append(rollback, e)
		}
		action := "disable"
		if wasEnabled {
			action = "enable"
		}
		if e := ops.control(action); e != nil {
			rollback = append(rollback, e)
		}
		if wasRunning {
			if e := ops.control("start"); e != nil {
				rollback = append(rollback, e)
			}
		}
		if len(rollback) > 0 {
			result = fmt.Errorf("%w; restoring the previous service state also failed: %v", result, rollback)
		}
	}()
	if err = saveConfig(destination, c); err != nil {
		return err
	}
	if err = ops.own(destination); err != nil {
		return err
	}
	if err = ops.control("start"); err != nil {
		return err
	}
	if err = ops.check(destination); err != nil {
		return err
	}
	return ops.control("enable")
}
