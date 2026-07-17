package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

// Zoxide is the slow external path. Strategy for "instant" open:
//
//  1. Persist pre-built picker items to zoxide.items.json (exported DTO).
//  2. newModel loads that file synchronously — no walk, no zoxide spawn.
//  3. Background tea.Cmd refreshes when TTL expired.

const (
	zoxCacheTTL  = 60 * time.Second
	zoxPathLimit = 120
)

// zoxRow is JSON-safe (item fields are unexported).
type zoxRow struct {
	Kind    int    `json:"kind"`
	Title   string `json:"title"`
	Desc    string `json:"desc"`
	Name    string `json:"name"`
	Path    string `json:"path"`
	Recency int64  `json:"recency"`
}

type zoxItemCache struct {
	Updated int64    `json:"updated"`
	Items   []zoxRow `json:"items"`
}

var (
	zoxItemMem   []item
	zoxItemMemAt time.Time
	zoxItemMu    sync.Mutex
)

func zoxItemCachePath() string {
	dir, err := dataDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "zoxide.items.json")
}

func zoxListCachePath() string {
	dir, err := dataDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "zoxide.list")
}

func itemsToRows(items []item) []zoxRow {
	out := make([]zoxRow, 0, len(items))
	for _, it := range items {
		if it.name == "" {
			continue
		}
		out = append(out, zoxRow{
			Kind:    int(it.kind),
			Title:   it.title,
			Desc:    it.desc,
			Name:    it.name,
			Path:    it.path,
			Recency: it.recency,
		})
	}
	return out
}

func rowsToItems(rows []zoxRow) []item {
	out := make([]item, 0, len(rows))
	for _, r := range rows {
		if r.Name == "" {
			continue
		}
		k := kind(r.Kind)
		if k != kindZoxide {
			k = kindZoxide
		}
		title := r.Title
		if title == "" {
			title = "[Zoxide] " + r.Name
		}
		out = append(out, item{
			kind:    k,
			title:   title,
			desc:    r.Desc,
			name:    r.Name,
			path:    r.Path,
			recency: r.Recency,
		})
	}
	return out
}

func loadZoxItemsSync() ([]item, time.Duration, bool) {
	zoxItemMu.Lock()
	if len(zoxItemMem) > 0 {
		age := time.Since(zoxItemMemAt)
		items := zoxItemMem
		zoxItemMu.Unlock()
		return items, age, true
	}
	zoxItemMu.Unlock()

	p := zoxItemCachePath()
	if p == "" {
		return nil, 0, false
	}
	b, err := os.ReadFile(p)
	if err != nil || len(b) == 0 {
		return nil, 0, false
	}
	var c zoxItemCache
	if err := json.Unmarshal(b, &c); err != nil || len(c.Items) == 0 {
		return nil, 0, false
	}
	items := rowsToItems(c.Items)
	if len(items) == 0 {
		// corrupt / old empty-object cache — delete so we rebuild
		_ = os.Remove(p)
		return nil, 0, false
	}
	age := time.Duration(0)
	if c.Updated > 0 {
		age = time.Since(time.Unix(c.Updated, 0))
		if age < 0 {
			age = 0
		}
	}
	zoxItemMu.Lock()
	zoxItemMem = items
	zoxItemMemAt = time.Now().Add(-age)
	zoxItemMu.Unlock()
	return items, age, true
}

func saveZoxItems(items []item) {
	p := zoxItemCachePath()
	if p == "" {
		return
	}
	rows := itemsToRows(items)
	if len(rows) == 0 {
		return
	}
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	c := zoxItemCache{Updated: time.Now().Unix(), Items: rows}
	b, err := json.Marshal(c)
	if err != nil {
		return
	}
	_ = os.WriteFile(p, b, 0o644)
	parsed := rowsToItems(rows)
	zoxItemMu.Lock()
	zoxItemMem = parsed
	zoxItemMemAt = time.Now()
	zoxItemMu.Unlock()
}

func zoxItemsStale(age time.Duration, ok bool) bool {
	return !ok || age >= zoxCacheTTL
}

func capZox(paths []string) []string {
	if len(paths) > zoxPathLimit {
		return paths[:zoxPathLimit]
	}
	return paths
}

func zoxideQueryFresh() []string {
	out, err := exec.Command("zoxide", "query", "-l").Output()
	if err != nil {
		return nil
	}
	var paths []string
	for _, line := range splitLines(string(out)) {
		if line != "" {
			paths = append(paths, line)
		}
	}
	if len(paths) > 0 {
		if p := zoxListCachePath(); p != "" {
			_ = os.MkdirAll(filepath.Dir(p), 0o755)
			var b []byte
			for _, line := range paths {
				b = append(b, line...)
				b = append(b, '\n')
			}
			_ = os.WriteFile(p, b, 0o644)
		}
	}
	return paths
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			line := trimSpace(s[start:i])
			if line != "" {
				out = append(out, line)
			}
			start = i + 1
		}
	}
	if start < len(s) {
		line := trimSpace(s[start:])
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t' || s[0] == '\r') {
		s = s[1:]
	}
	for len(s) > 0 {
		c := s[len(s)-1]
		if c != ' ' && c != '\t' && c != '\r' {
			break
		}
		s = s[:len(s)-1]
	}
	return s
}

// rebuildZoxItems: spawn zoxide + projectSession — tea.Cmd / background only.
func rebuildZoxItems() []item {
	paths := zoxideQueryFresh()
	if len(paths) == 0 {
		return nil
	}
	items := zoxideItems(capZox(paths), map[string]bool{}, map[string]bool{})
	if len(items) > 0 {
		saveZoxItems(items)
	}
	return items
}

func zoxideList() []string {
	if items, _, ok := loadZoxItemsSync(); ok {
		out := make([]string, 0, len(items))
		for _, it := range items {
			if it.path != "" {
				out = append(out, it.path)
			}
		}
		return out
	}
	return zoxideQueryFresh()
}

func zoxideListCachedOnly() []string {
	if items, _, ok := loadZoxItemsSync(); ok {
		out := make([]string, 0, len(items))
		for _, it := range items {
			if it.path != "" {
				out = append(out, it.path)
			}
		}
		return out
	}
	return nil
}
