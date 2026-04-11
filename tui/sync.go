package tui

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/nelsong6/fzt/core"
	terminal "github.com/nelsong6/fzt-terminal"
)

const syncInterval = 20 * 60 // 20 minutes in seconds

// initSyncCheck reads .last-sync-check, sets SyncNextCheck on state,
// and spawns a background goroutine if a check is due.
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
			stale := checkBookmarkStaleness(cfg.ConfigDir, secret)
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

// checkBookmarkStaleness checks if the local bookmark cache is older than
// what the API has.
func checkBookmarkStaleness(configDir string, secret string) bool {
	if secret == "" {
		var err error
		secret, err = terminal.ReadJWTSecret()
		if err != nil {
			return false
		}
	}

	_, claims, err := terminal.LoadIdentityClaims(configDir)
	if err != nil {
		return false
	}

	token := terminal.MintJWT(secret, claims)
	_, _, updatedAt, err := terminal.FetchMenu(token)
	if err != nil {
		return false
	}

	if updatedAt == "" {
		return false
	}

	serverTime, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return false
	}

	cacheFile := filepath.Join(configDir, "menu-cache.yaml")
	info, err := os.Stat(cacheFile)
	if err != nil {
		return true // no cache = stale
	}

	return serverTime.After(info.ModTime())
}
