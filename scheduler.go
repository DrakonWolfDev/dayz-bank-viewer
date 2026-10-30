package main

import (
	"context"
	"time"
)

// scheduleDue deduplicates wall-clock minutes, including a repeated DST hour.
// A skipped DST minute is skipped; downtime does not cause catch-up scans.
func scheduleDue(now time.Time, times []string, fired map[string]bool) bool {
	date := now.Format("2006-01-02")
	for key := range fired {
		if len(key) < 10 || key[:10] != date {
			delete(fired, key)
		}
	}
	minute := now.Format("15:04")
	for _, t := range times {
		key := date + " " + t
		if t == minute && !fired[key] {
			fired[key] = true
			return true
		}
	}
	return false
}
func (m *Manager) Schedule(ctx context.Context) {
	fired := map[string]bool{}
	for {
		c := m.Config()
		if scheduleDue(time.Now(), c.RefreshTimes, fired) {
			m.Refresh("")
		}
		now := time.Now()
		next := now.Truncate(time.Minute).Add(time.Minute)
		timer := time.NewTimer(time.Until(next))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}
