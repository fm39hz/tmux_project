package main

import (
	"fmt"
	"path/filepath"
	"sort"
)

type kind int

const (
	kindCreate kind = iota
	kindActive
	kindPreset
	kindZoxide
)

type item struct {
	kind    kind
	title   string
	desc    string
	name    string
	path    string
	windows int
	// recency: higher = more recent / more frequent.
	// Preset: last_used unix; Zoxide: inverted list index (zoxide score order);
	// Active/Create: 0 (kind already prefers them).
	recency int64
}

const zoxCap = 40 // unfiltered list shows top-N zoxide only

// collectBase: Create → Active → Presets(last_used). No zoxide.
func collectBase(ctl *TmuxCtl, store *Store, create item) []item {
	seenName := map[string]bool{}
	var items []item

	live, _ := ctl.ListLive()
	liveNames := map[string]bool{}
	for _, s := range live {
		liveNames[s.Name] = true
	}

	if create.name != "" && !liveNames[create.name] {
		seenName[create.name] = true
		items = append(items, create)
	}

	for _, s := range live {
		seenName[s.Name] = true
		items = append(items, item{
			kind:    kindActive,
			title:   fmt.Sprintf("[Active] %s", s.Name),
			desc:    fmt.Sprintf("%d windows", s.Windows),
			name:    s.Name,
			path:    s.Path,
			windows: s.Windows,
		})
	}

	if meta, err := store.ListMeta(); err == nil {
		for _, m := range meta {
			if seenName[m.Name] {
				continue
			}
			seenName[m.Name] = true
			items = append(items, item{
				kind:    kindPreset,
				title:   fmt.Sprintf("[Preset] %s", m.Name),
				desc:    "saved layout",
				name:    m.Name,
				path:    m.Cwd,
				recency: m.LastUsed,
			})
		}
	}
	return items
}

func normPath(p string) string {
	if p == "" {
		return ""
	}
	return filepath.Clean(p)
}

func occupancy(items []item) (names, paths map[string]bool) {
	names = map[string]bool{}
	paths = map[string]bool{}
	for _, it := range items {
		names[it.name] = true
		if p := normPath(it.path); p != "" {
			paths[p] = true
		}
	}
	return names, paths
}

// zoxideItems: skip if session name OR path already covered.
// recency: earlier in zoxide list (higher frecency) → larger recency.
func zoxideItems(zpaths []string, names, paths map[string]bool) []item {
	var out []item
	n := len(zpaths)
	for i, p := range zpaths {
		np := normPath(p)
		base := sessionName(p)
		if base == "" {
			continue
		}
		if names[base] || (np != "" && paths[np]) {
			continue
		}
		names[base] = true
		if np != "" {
			paths[np] = true
		}
		out = append(out, item{
			kind:    kindZoxide,
			title:   fmt.Sprintf("[Zoxide] %s", base),
			desc:    p,
			name:    base,
			path:    p,
			recency: int64(n - i), // first path = highest
		})
	}
	return out
}

// rankItems sorts pool by rankKey.
func rankItems(q string, pool []item) []item {
	type scored struct {
		it item
		k  rankKey
	}
	hits := make([]scored, 0, len(pool))
	for i, it := range pool {
		k, ok := rankOf(q, it, i)
		if !ok {
			continue
		}
		hits = append(hits, scored{it, k})
	}
	sort.SliceStable(hits, func(a, b int) bool {
		return hits[a].k.less(hits[b].k)
	})
	out := make([]item, len(hits))
	for i, h := range hits {
		out[i] = h.it
	}
	return out
}
