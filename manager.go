package main

import (
	"context"
	"errors"
	"math/big"
	"sort"
	"sync"
	"time"
)

type serverState struct {
	config      ServerConfig
	snapshot    *Snapshot // Immutable after publication; swapped under mu.
	scanning    bool
	lastAttempt *time.Time
	lastSuccess *time.Time
	lastError   string
	failedFiles []FileIssue
}
type ServerView struct {
	ServerConfig
	Status      string      `json:"status"`
	LastAttempt *time.Time  `json:"last_attempt"`
	LastSuccess *time.Time  `json:"last_success"`
	Snapshot    *Snapshot   `json:"snapshot"`
	Error       string      `json:"error,omitempty"`
	FailedFiles []FileIssue `json:"failed_files,omitempty"`
}
type Manager struct {
	mu         sync.RWMutex
	config     Config
	configPath string
	states     map[string]*serverState
	reader     AccountReader
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	slots      chan struct{}
	closed     bool
}

func newManager(c Config, path string) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	m := &Manager{config: c, configPath: path, states: map[string]*serverState{}, reader: AutoBankingReader{}, ctx: ctx, cancel: cancel, slots: make(chan struct{}, 4)}
	for _, s := range c.Servers {
		m.states[s.ID] = &serverState{config: s}
	}
	return m
}
func (m *Manager) Config() Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	c := m.config
	c.Servers = append([]ServerConfig{}, c.Servers...)
	c.RefreshTimes = append([]string{}, c.RefreshTimes...)
	return c
}
func (m *Manager) Update(c Config) error {
	if err := validateConfig(&c, m.configPath); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return errors.New("viewer is shutting down")
	}
	if err := saveConfig(m.configPath, c); err != nil {
		return err
	}
	states := map[string]*serverState{}
	for _, s := range c.Servers {
		state := &serverState{config: s}
		if prev := m.states[s.ID]; prev != nil && prev.config.Path == s.Path && prev.config.LogPath == s.LogPath && prev.config.accountType() == s.accountType() {
			// Reuse the state so an in-flight scan remains associated with this path.
			state = prev
			state.config = s
		}
		states[s.ID] = state
	}
	m.states = states
	m.config = c
	return nil
}

func (m *Manager) Refresh(id string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return 0, errors.New("viewer is shutting down")
	}
	if id != "" {
		if _, ok := m.states[id]; !ok {
			return 0, errors.New("server not found")
		}
	}
	count := 0
	for sid, s := range m.states {
		if (id != "" && sid != id) || !s.config.Enabled || s.scanning {
			continue
		}
		s.scanning = true
		t := time.Now()
		s.lastAttempt = &t
		config := s.config
		m.wg.Add(1)
		count++
		go m.scan(sid, s, config)
	}
	return count, nil
}
func (m *Manager) scan(id string, state *serverState, config ServerConfig) {
	defer m.wg.Done()
	var snap *Snapshot
	var err error
	select {
	case m.slots <- struct{}{}:
		reader := m.reader
		if config.accountType() == "clan" {
			reader = AutoClanReader{}
		}
		snap, err = scanDirectory(m.ctx, config.Path, reader)
		if err == nil && config.accountType() == "player" {
			enrichAccountNames(m.ctx, snap, config)
			if m.ctx.Err() != nil {
				err = m.ctx.Err()
			}
		}
		<-m.slots
	case <-m.ctx.Done():
		err = m.ctx.Err()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.states[id] != state {
		return
	} // Removed or repointed while scanning.
	state.scanning = false
	if err != nil {
		state.lastError = err.Error()
		state.failedFiles = nil
		if snap != nil {
			state.failedFiles = snap.Issues
		}
		return
	}
	state.snapshot = snap
	state.lastError = ""
	state.failedFiles = nil
	if len(snap.Issues) == 0 {
		t := snap.ScannedAt
		state.lastSuccess = &t
	}
}
func (m *Manager) Close() { m.mu.Lock(); m.closed = true; m.cancel(); m.mu.Unlock(); m.wg.Wait() }

func (m *Manager) Views() []ServerView {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]ServerView, 0, len(m.config.Servers))
	for _, c := range m.config.Servers {
		s := m.states[c.ID]
		status := "not_scanned"
		switch {
		case !c.Enabled:
			status = "disabled"
		case s.scanning:
			status = "scanning"
		case s.lastError != "":
			status = "error"
		case s.snapshot != nil && len(s.snapshot.Issues) > 0:
			status = "partial"
		case s.snapshot != nil:
			status = "ready"
		}
		out = append(out, ServerView{ServerConfig: c, Status: status, LastAttempt: s.lastAttempt, LastSuccess: s.lastSuccess, Snapshot: s.snapshot, Error: s.lastError, FailedFiles: s.failedFiles})
	}
	return out
}

type Row struct {
	Account
	ServerID   string `json:"server_id"`
	ServerName string `json:"server_name"`
	Rank       int    `json:"rank"`
}
type Dashboard struct {
	Servers       []ServerView `json:"servers"`
	Accounts      []Row        `json:"accounts"`
	AccountCount  int          `json:"account_count"`
	UniquePlayers int          `json:"unique_players"`
	Total         string       `json:"total"`
	Highest       Amount       `json:"highest"`
	Complete      bool         `json:"complete"`
	Timezone      string       `json:"timezone"`
	HostTime      string       `json:"host_time"`
}

func (m *Manager) Dashboard(id string) (Dashboard, error) {
	return m.DashboardForType(id, "player")
}

func (m *Manager) DashboardForType(id, accountType string) (Dashboard, error) {
	views := m.Views()
	d := Dashboard{Servers: views, Accounts: []Row{}, Total: "0", Complete: true, HostTime: time.Now().Format(time.RFC3339), Timezone: time.Now().Location().String()}
	total := new(big.Int)
	unique := map[string]bool{}
	found := id == ""
	selected := 0
	for _, s := range views {
		if s.accountType() != accountType {
			continue
		}
		if id != "" && s.ID != id {
			continue
		}
		if id == "" && !s.Enabled {
			continue
		}
		found = true
		selected++
		if s.Snapshot == nil || len(s.Snapshot.Issues) > 0 || s.Error != "" || s.Status == "disabled" {
			d.Complete = false
		}
		if s.Snapshot == nil {
			continue
		}
		for _, a := range s.Snapshot.Accounts {
			d.Accounts = append(d.Accounts, Row{Account: a, ServerID: s.ID, ServerName: s.Name})
			total.Add(total, big.NewInt(int64(a.Balance)))
			unique[a.identityKey()] = true
		}
	}
	if !found {
		return d, errors.New("server not found")
	}
	if selected == 0 {
		d.Complete = false
	}
	sort.Slice(d.Accounts, func(i, j int) bool {
		a, b := d.Accounts[i], d.Accounts[j]
		if a.Balance != b.Balance {
			return a.Balance > b.Balance
		}
		if a.identityKey() != b.identityKey() {
			return a.identityKey() < b.identityKey()
		}
		return a.ServerID < b.ServerID
	})
	for i := range d.Accounts {
		d.Accounts[i].Rank = i + 1
	}
	if len(d.Accounts) > 0 {
		d.Highest = d.Accounts[0].Balance
	}
	d.Total = total.String()
	d.AccountCount = len(d.Accounts)
	d.UniquePlayers = len(unique)
	// Metadata includes issues, but accounts are returned only once, above.
	for i := range d.Servers {
		if s := d.Servers[i].Snapshot; s != nil {
			copy := *s
			copy.Accounts = nil
			d.Servers[i].Snapshot = &copy
		}
	}
	return d, nil
}
