package picker

import (
	"os/exec"
	"sync"
	"time"

	"github.com/fm39hz/gotomux/internal/store"
)

// Zoxide truth is `zoxide query -l`. Cache (mem + SQLite) only seeds paint.
// zoxideSource.Refresh always rebuilds full list into the search pool.

var (
	zoxItemMem   []Item
	zoxItemMemAt time.Time
	zoxItemMu    sync.Mutex
	zoxStore     *store.Store
)

func BindStore(s *store.Store) { zoxStore = s }

func loadZoxItemsSync() ([]Item, time.Duration, bool) {
	zoxItemMu.Lock()
	if len(zoxItemMem) > 0 {
		age := time.Since(zoxItemMemAt)
		items := zoxItemMem
		zoxItemMu.Unlock()
		return items, age, true
	}
	zoxItemMu.Unlock()

	if zoxStore == nil {
		return nil, 0, false
	}
	rows, updated, ok := zoxStore.LoadZox()
	if !ok {
		return nil, 0, false
	}
	items := zoxRowsToItems(rows)
	age := time.Duration(0)
	if updated > 0 {
		age = time.Since(time.Unix(updated, 0))
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

func saveZoxItems(items []Item) {
	if len(items) == 0 {
		return
	}
	if zoxStore != nil {
		_ = zoxStore.SaveZox(itemsToZoxRows(items))
	}
	zoxItemMu.Lock()
	zoxItemMem = items
	zoxItemMemAt = time.Now()
	zoxItemMu.Unlock()
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

func rebuildZoxItems() []Item {
	paths := zoxideQueryFresh()
	if len(paths) == 0 {
		return nil
	}
	// full list - low-rank paths stay searchable after merge; paint already done
	items := zoxideItems(paths, nil, nil)
	if len(items) > 0 {
		saveZoxItems(items)
	}
	return items
}

func zoxideList() []string {
	if items, _, ok := loadZoxItemsSync(); ok {
		out := make([]string, 0, len(items))
		for _, it := range items {
			if it.Path != "" {
				out = append(out, it.Path)
			}
		}
		return out
	}
	return zoxideQueryFresh()
}

func zoxRowsToItems(rows []store.ZoxRow) []Item {
	out := make([]Item, 0, len(rows))
	for _, r := range rows {
		title := r.Title
		if title == "" {
			title = "[Zoxide] " + r.Name
		}
		out = append(out, Item{
			Src: SrcZoxide, Kind: KindZoxide,
			Title: title, Desc: r.Desc, Name: r.Name, Path: r.Path, Recency: r.Recency,
		})
	}
	return out
}

func itemsToZoxRows(items []Item) []store.ZoxRow {
	out := make([]store.ZoxRow, 0, len(items))
	for _, it := range items {
		if it.Name == "" {
			continue
		}
		out = append(out, store.ZoxRow{
			Name: it.Name, Path: it.Path, Title: it.Title,
			Desc: it.Desc, Recency: it.Recency,
		})
	}
	return out
}
