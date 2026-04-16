package tui

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/nelsong6/fzt/core"
	frontend "github.com/nelsong6/fzt-frontend"
)

const syncInterval = 20 * 60 // 20 minutes in seconds

// initSyncCheck reads .last-sync-check, sets SyncNextCheck on state,
// and spawns a background goroutine if a check is due.
//
// The actual "fetch from API and compare timestamps" logic lives in
// frontend.CheckBookmarkStaleness. This file owns only the renderer-side
// concerns: the timer, UI state mutation (SyncIcon), and posting events
// back into the tcell event loop.
func initSyncCheck(s *core.State, cfg Config, postEvent func()) {
	if cfg.ConfigDir == "" {
		return
	}

	lastCheckFile := filepath.Join(cfg.ConfigDir, ".last-sync-check")
	lastCheck := int64(0)
	if data, err := os.ReadFile(lastCheckFile); err == nil {
		lastCheck, _ = strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	}

	now := time.Now().Unix()
	s.SyncNextCheck = lastCheck + syncInterval

	if now >= s.SyncNextCheck {
		secret := s.JWTSecret
		go func() {
			stale := frontend.CheckBookmarkStaleness(cfg.ConfigDir, secret)
			if stale {
				s.SyncIcon = "⟳"
			}
			nowStr := strconv.FormatInt(time.Now().Unix(), 10)
			os.WriteFile(lastCheckFile, []byte(nowStr), 0644)
			s.SyncNextCheck = time.Now().Unix() + syncInterval
			postEvent()
		}()
	}
}
