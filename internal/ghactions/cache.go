package ghactions

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// entry caches one endpoint's ETag and body so a 304 (which does not count
// against the GitHub rate limit) can be served without re-fetching.
type entry struct {
	ETag string `json:"etag"`
	Body string `json:"body"`
}

type accountCache struct {
	ReposPath    string           `json:"reposPath,omitempty"`
	ReposFetched int64            `json:"reposFetched,omitempty"`
	Repos        []string         `json:"repos,omitempty"`
	Entries      map[string]entry `json:"entries,omitempty"`
	Last         *Snapshot        `json:"last,omitempty"`
}

type cacheFile struct {
	Version    int                     `json:"version"`
	Login      string                  `json:"login,omitempty"`
	PauseUntil int64                   `json:"pauseUntil,omitempty"`
	Remaining  int                     `json:"remaining,omitempty"`
	Accounts   map[string]accountCache `json:"accounts,omitempty"`
}

// CacheStore persists ETags, the repo list, and the rate-limit pause between the
// helper's one-shot invocations. The shell runs the binary fresh on every poll,
// so without this the cache would not survive.
type CacheStore struct {
	path string
	data cacheFile
	mu   sync.Mutex
}

// DefaultCacheDir returns $XDG_STATE_HOME/omarchy/gh-actions-watch, falling back
// to $HOME/.local/state/omarchy/gh-actions-watch.
func DefaultCacheDir() string {
	base := os.Getenv("XDG_STATE_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		base = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(base, "omarchy", "gh-actions-watch")
}

// LoadCache reads the cache file at dir, or starts an empty one. A missing or
// corrupt file is not fatal; polling simply continues without cached ETags.
func LoadCache(dir string) *CacheStore {
	c := &CacheStore{data: cacheFile{Version: 1, Remaining: -1, Accounts: map[string]accountCache{}}}
	if dir == "" {
		return c
	}
	c.path = filepath.Join(dir, "cache.json")
	raw, err := os.ReadFile(c.path)
	if err != nil {
		return c
	}
	var f cacheFile
	if err := json.Unmarshal(raw, &f); err != nil {
		return c
	}
	if f.Accounts == nil {
		f.Accounts = map[string]accountCache{}
	}
	if f.Remaining == 0 {
		f.Remaining = -1
	}
	c.data = f
	return c
}

func (c *CacheStore) Save() {
	if c == nil || c.path == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(c.path), 0o755); err != nil {
		return
	}
	raw, err := json.Marshal(c.data)
	if err != nil {
		return
	}
	tmp := c.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return
	}
	_ = os.Rename(tmp, c.path)
}

func (c *CacheStore) account(name string) accountCache {
	a, ok := c.data.Accounts[name]
	if !ok || a.Entries == nil {
		a.Entries = map[string]entry{}
	}
	return a
}

func (c *CacheStore) setAccount(name string, a accountCache) {
	c.data.Accounts[name] = a
}

func (c *CacheStore) paused(now time.Time) bool {
	return c.data.PauseUntil > now.Unix()
}

func (c *CacheStore) pauseUntil() time.Time {
	if c.data.PauseUntil <= 0 {
		return time.Time{}
	}
	return time.Unix(c.data.PauseUntil, 0)
}
