package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
}

type action int

const (
	actionNone action = iota
	actionConnect
	actionQuit
)

type result struct {
	action action
	item   item
	err    error
}

type model struct {
	all      []item
	view     []item
	cursor   int
	query    string
	ctl      *TmuxCtl
	store    *Store
	create   item
	status   string
	done     result
	width    int
	height   int
	maxShow  int
	editPath string // temp file while $EDITOR open
	editOld  string // preset name before edit (rename detect)
}

var (
	stylePrompt = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)
	styleCursor = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	styleNormal = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	styleDim    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	styleStatus = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	styleHeader = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)

func newModel(ctl *TmuxCtl, store *Store, createName, createCwd string) model {
	create := item{
		kind:  kindCreate,
		title: fmt.Sprintf("[Create] %s", createName),
		desc:  createCwd,
		name:  createName,
		path:  createCwd,
	}
	// skip zoxide here — Init loads it async so TUI paints immediately
	all := collectItems(ctl, store, create, false)
	m := model{
		all:     all,
		ctl:     ctl,
		store:   store,
		create:  create,
		maxShow: 12,
	}
	m.refilter()
	return m
}

func collectItems(ctl *TmuxCtl, store *Store, create item, withZoxide bool) []item {
	seen := map[string]bool{}
	var items []item

	items = append(items, create)
	seen[create.name] = true

	// live + sqlite in parallel — both cheap, still shaves a bit
	type liveRes struct {
		ss  []LiveSession
		err error
	}
	type nameRes struct {
		ns  []string
		err error
	}
	liveCh := make(chan liveRes, 1)
	nameCh := make(chan nameRes, 1)
	go func() {
		ss, err := ctl.ListLive()
		liveCh <- liveRes{ss, err}
	}()
	go func() {
		ns, err := store.ListNames()
		nameCh <- nameRes{ns, err}
	}()
	lr := <-liveCh
	nr := <-nameCh

	if lr.err == nil {
		for _, s := range lr.ss {
			seen[s.Name] = true
			items = append(items, item{
				kind:    kindActive,
				title:   fmt.Sprintf("[Active] %s", s.Name),
				desc:    fmt.Sprintf("%d windows", s.Windows),
				name:    s.Name,
				windows: s.Windows,
			})
		}
	}
	if nr.err == nil {
		for _, n := range nr.ns {
			if seen[n] {
				continue
			}
			seen[n] = true
			items = append(items, item{
				kind:  kindPreset,
				title: fmt.Sprintf("[Preset] %s", n),
				desc:  "saved layout",
				name:  n,
			})
		}
	}

	if withZoxide {
		for _, p := range zoxideList() {
			base := sessionName(p)
			if seen[base] {
				continue
			}
			seen[base] = true
			items = append(items, item{
				kind:  kindZoxide,
				title: fmt.Sprintf("[Zoxide] %s", base),
				desc:  p,
				name:  base,
				path:  p,
			})
		}
	}
	return items
}

type zoxideMsg []string

func loadZoxideCmd() tea.Msg {
	return zoxideMsg(zoxideList())
}

func (m *model) mergeZoxide(paths []string) {
	seen := map[string]bool{}
	for _, it := range m.all {
		seen[it.name] = true
	}
	for _, p := range paths {
		base := sessionName(p)
		if seen[base] {
			continue
		}
		seen[base] = true
		m.all = append(m.all, item{
			kind:  kindZoxide,
			title: fmt.Sprintf("[Zoxide] %s", base),
			desc:  p,
			name:  base,
			path:  p,
		})
	}
}

func (m *model) refilter() {
	q := strings.ToLower(strings.TrimSpace(m.query))
	if q == "" {
		m.view = append([]item(nil), m.all...)
	} else {
		m.view = m.view[:0]
		for _, it := range m.all {
			hay := strings.ToLower(it.title + " " + it.name + " " + it.path + " " + it.desc)
			if fuzzyMatch(q, hay) {
				m.view = append(m.view, it)
			}
		}
	}
	if m.cursor >= len(m.view) {
		m.cursor = len(m.view) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// subsequence match like fzf default (case-insensitive already applied)
func fuzzyMatch(query, text string) bool {
	if query == "" {
		return true
	}
	ti := 0
	for qi := 0; qi < len(query); qi++ {
		found := false
		for ; ti < len(text); ti++ {
			if text[ti] == query[qi] {
				ti++
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func (m model) Init() tea.Cmd { return loadZoxideCmd }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case zoxideMsg:
		m.mergeZoxide(msg)
		m.refilter()
		return m, nil

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		// inline mode: keep list short like fzf --height
		if m.maxShow <= 0 {
			m.maxShow = 12
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.done = result{action: actionQuit}
			return m, tea.Quit

		case "enter":
			if len(m.view) > 0 && m.cursor < len(m.view) {
				m.done = result{action: actionConnect, item: m.view[m.cursor]}
				return m, tea.Quit
			}

		case "ctrl+n", "down":
			if len(m.view) > 0 {
				m.cursor = (m.cursor + 1) % len(m.view)
			}
			return m, nil

		case "ctrl+p", "up":
			if len(m.view) > 0 {
				m.cursor--
				if m.cursor < 0 {
					m.cursor = len(m.view) - 1
				}
			}
			return m, nil

		case "ctrl+u":
			m.query = ""
			m.refilter()
			return m, nil

		case "ctrl+w":
			m.query = trimLastWord(m.query)
			m.refilter()
			return m, nil

		case "backspace":
			if len(m.query) > 0 {
				// drop last rune
				r := []rune(m.query)
				m.query = string(r[:len(r)-1])
				m.refilter()
			}
			return m, nil

		case "ctrl+x": // kill active
			if len(m.view) > 0 {
				it := m.view[m.cursor]
				if it.kind == kindActive {
					if err := m.ctl.Kill(it.name); err != nil {
						m.status = err.Error()
					} else {
						m.status = "killed " + it.name
						m.reload()
					}
				}
			}
			return m, nil

		case "ctrl+f": // freeze active
			if len(m.view) > 0 {
				it := m.view[m.cursor]
				if it.kind == kindActive {
					p, err := m.ctl.Freeze(it.name)
					if err != nil {
						m.status = err.Error()
					} else if err := m.store.Save(p); err != nil {
						m.status = err.Error()
					} else {
						m.status = "froze " + it.name
						m.reload()
					}
				}
			}
			return m, nil

		case "ctrl+e": // edit preset in $EDITOR
			if len(m.view) > 0 {
				it := m.view[m.cursor]
				if it.kind == kindPreset {
					cmd, err := m.beginEdit(it.name)
					if err != nil {
						m.status = err.Error()
						return m, nil
					}
					return m, cmd
				}
			}
			return m, nil

		case "ctrl+d": // delete preset
			if len(m.view) > 0 {
				it := m.view[m.cursor]
				if it.kind == kindPreset {
					if err := m.store.Delete(it.name); err != nil {
						m.status = err.Error()
					} else {
						m.status = "deleted " + it.name
						m.reload()
					}
				}
			}
			return m, nil

		default:
			// printable → append to query (combobox always typing)
			if msg.Type == tea.KeyRunes {
				for _, r := range msg.Runes {
					if unicode.IsPrint(r) {
						m.query += string(r)
					}
				}
				m.refilter()
			}
		}

	case editDoneMsg:
		if msg.err != nil {
			m.status = msg.err.Error()
		} else {
			m.status = "saved " + msg.name
			m.reload()
		}
		return m, nil
	}
	return m, nil
}

func (m *model) reload() {
	m.all = collectItems(m.ctl, m.store, m.create, true)
	m.refilter()
}

// frameLines is fixed height of View — used to wipe residual UI after quit.
func (m model) frameLines() int {
	maxShow := m.maxShow
	if maxShow <= 0 {
		maxShow = 12
	}
	// prompt + header + list + status
	return maxShow + 3
}

type editDoneMsg struct {
	err  error
	name string
}

func (m *model) beginEdit(name string) (tea.Cmd, error) {
	p, err := m.store.Get(name)
	if err != nil {
		return nil, err
	}
	dir, err := dataDir()
	if err != nil {
		return nil, err
	}
	f, err := os.CreateTemp(dir, "edit-*.tp")
	if err != nil {
		return nil, err
	}
	path := f.Name()
	if _, err := f.WriteString(formatPreset(p)); err != nil {
		f.Close()
		os.Remove(path)
		return nil, err
	}
	f.Close()
	m.editPath = path
	m.editOld = name

	ed := os.Getenv("EDITOR")
	if ed == "" {
		ed = os.Getenv("VISUAL")
	}
	if ed == "" {
		ed = "nvim"
	}
	var c *exec.Cmd
	if fields := strings.Fields(ed); len(fields) > 1 {
		c = exec.Command(fields[0], append(fields[1:], path)...)
	} else {
		c = exec.Command(ed, path)
	}
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr

	store := m.store
	old := name
	return tea.ExecProcess(c, func(err error) tea.Msg {
		defer os.Remove(path)
		if err != nil {
			return editDoneMsg{err: fmt.Errorf("editor: %w", err)}
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return editDoneMsg{err: err}
		}
		np, err := parsePreset(string(raw))
		if err != nil {
			return editDoneMsg{err: fmt.Errorf("parse: %w", err)}
		}
		if err := store.Save(np); err != nil {
			return editDoneMsg{err: err}
		}
		if np.Name != old {
			_ = store.Delete(old)
		}
		return editDoneMsg{name: np.Name}
	}), nil
}

func trimLastWord(s string) string {
	s = strings.TrimRightFunc(s, unicode.IsSpace)
	i := strings.LastIndexFunc(s, unicode.IsSpace)
	if i < 0 {
		return ""
	}
	return s[:i+1]
}

func (m model) View() string {
	var b strings.Builder

	// prompt line — fzf style
	b.WriteString(stylePrompt.Render("❯ "))
	b.WriteString(m.query)
	b.WriteString(styleDim.Render("█"))
	b.WriteByte('\n')
	b.WriteString(styleHeader.Render(fmt.Sprintf("  %d/%d  ^n/p · enter · ^x kill · ^f freeze · ^e edit · ^d del · esc", len(m.view), len(m.all))))
	b.WriteByte('\n')

	maxShow := m.maxShow
	if maxShow <= 0 {
		maxShow = 12
	}

	shown := 0
	if len(m.view) == 0 {
		b.WriteString(styleDim.Render("  (no match)"))
		b.WriteByte('\n')
		shown = 1
	} else {
		start := 0
		if m.cursor >= maxShow {
			start = m.cursor - maxShow + 1
		}
		end := start + maxShow
		if end > len(m.view) {
			end = len(m.view)
		}
		for i := start; i < end; i++ {
			it := m.view[i]
			line := it.title
			if it.desc != "" {
				line = line + "  " + it.desc
			}
			if m.width > 4 && len(line) > m.width-2 {
				line = line[:m.width-5] + "…"
			}
			if i == m.cursor {
				b.WriteString(styleCursor.Render("▸ " + line))
			} else {
				b.WriteString(styleNormal.Render("  " + line))
			}
			b.WriteByte('\n')
			shown++
		}
	}
	// pad to fixed height so filter shrink doesn't leave ghost lines
	for shown < maxShow {
		b.WriteByte('\n')
		shown++
	}

	// status always occupies 1 line — fixed frame height for clearInline
	if m.status != "" {
		b.WriteString(styleStatus.Render(m.status))
	}
	b.WriteByte('\n')
	return b.String()
}

// clearInline erases n lines of residual bubbletea inline UI (fzf-style).
// Bubble Tea stop() only clears the current line — the rest stays in scrollback.
func clearInline(n int) {
	if n <= 0 {
		return
	}
	var b strings.Builder
	// cursor is at start of last rendered line after stop(); go up n-1 then erase n
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteString("\x1b[1A") // up
		}
		b.WriteString("\x1b[2K") // erase line
	}
	b.WriteByte('\r')
	fmt.Fprint(os.Stdout, b.String())
}
