package picker

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/lipgloss"

	"github.com/fm39hz/gotomux/internal/store"
	"github.com/fm39hz/gotomux/internal/template"
	"github.com/fm39hz/gotomux/internal/tmux"
)

type Action int

const (
	ActionNone Action = iota
	ActionConnect
	ActionQuit
)

type Result struct {
	Action Action
	Item   Item
	Err    error
}

// viewModel — UI state, tách biệt khỏi business logic.
type viewModel struct {
	items    []Item
	cursor   int
	selID    ID
	query    string
	status   string
	done     Result
	width    int
	height   int
	maxShow  int
	help     bool
	started  time.Time
	editPath string
	editOld  string
}

func (v *viewModel) scrollOff() int {
	ms := v.maxShow
	if ms <= 0 {
		ms = 12
	}
	half := ms / 2
	s := v.cursor - half
	if s < 0 {
		s = 0
	}
	if s+ms > len(v.items) {
		s = len(v.items) - ms
	}
	if s < 0 {
		s = 0
	}
	return s
}

func (v *viewModel) visible() []Item {
	start := v.scrollOff()
	end := start + v.maxShow
	if end > len(v.items) {
		end = len(v.items)
	}
	return v.items[start:end]
}

// viewModel

type model struct {
	sources    []Source
	bySrc      map[Source][]Item
	ctl        tmux.Connector
	store      *store.Store
	cache      sourceCache
	env        Context
	tmpl       string
	createName string
	createCwd  string
	ui         viewModel
}

// ID now method on Item { return it.Name + "\x00" + it.Path }

var (
	styleCursor = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	// weight: Active strongest -> Preset -> Create -> Zoxide dimmest
	styleActive = lipgloss.NewStyle().Foreground(lipgloss.Color("15")) // bright white
	stylePreset = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))  // normal
	styleCreate = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))  // cyan - action
	styleZoxide = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))  // gray
	styleDim    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	styleStatus = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	styleHeader = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)

func (m model) Done() Result { return m.ui.done }

func styleFor(k Kind) lipgloss.Style {
	switch k {
	case KindActive:
		return styleActive
	case KindPreset:
		return stylePreset
	case KindCreate:
		return styleCreate
	case KindZoxide:
		return styleZoxide
	default:
		return stylePreset
	}
}

func NewModelFromDaemon(ctl tmux.Connector, st *store.Store, createName, createCwd string, sessions []tmux.LiveSession, presets []store.PresetMeta, env Context) model {
	var cache sourceCache
	cache.zoxSt = st
	cache.zoxMu = &sync.Mutex{}
	cache.tmuxSnap = sessions
	cache.tmuxOK = true
	cache.presetM = presets
	cache.presetOK = true
	srcs := defaultSources(ctl, st, createName, createCwd, &cache)
	bySrc := snapshotAll(srcs)
	applyRankMeta(bySrc, st, env)
	enrichAllSync(bySrc)
	m := model{
		sources:    srcs,
		bySrc:      bySrc,
		cache:      cache,
		ctl:        ctl,
		store:      st,
		tmpl:       template.StickyLabel(st),
		env:        env,
		createName: createName,
		createCwd:  createCwd,
		ui: viewModel{maxShow: 12, started: time.Now()},
	}
	m.refilter()
	return m
}

func NewModel(ctl tmux.Connector, store *store.Store, createName, createCwd string) model {
	var cache sourceCache
	cache.zoxSt = store
	cache.zoxMu = &sync.Mutex{}
	srcs := defaultSources(ctl, store, createName, createCwd, &cache)
	bySrc := snapshotAll(srcs)
	env := newContext(ctl, store)
	applyRankMeta(bySrc, store, env)
	enrichAllSync(bySrc)
	m := model{
		sources:    srcs,
		bySrc:      bySrc,
		cache:      cache,
		ctl:        ctl,
		store:      store,
		tmpl:       template.StickyLabel(store),
		env:        env,
		createName: createName,
		createCwd:  createCwd,
		ui: viewModel{maxShow: 12, started: time.Now()},
	}
	m.refilter()
	return m
}



func (m *model) pool() []Item {
	return flattenSources(m.sources, m.bySrc, strings.TrimSpace(m.ui.query))
}

func (m *model) mergeSource(src Source, items []Item) {
	if m.bySrc == nil {
		m.bySrc = map[Source][]Item{}
	}
	slot := map[Source][]Item{src: items}
	applyRankMeta(slot, m.store, m.env)
	m.bySrc[src] = slot[src]
}

func (m *model) refilter() {
	// Preserve scroll offset so visual position doesn't jump on kill/delete.
	q := strings.ToLower(strings.TrimSpace(m.ui.query))
	m.ui.items = rankItems(q, m.pool())

	if m.ui.cursor >= len(m.ui.items) {
		m.ui.cursor = len(m.ui.items) - 1
	}
	if m.ui.cursor < 0 && len(m.ui.items) > 0 {
		m.ui.cursor = 0
	}
	if len(m.ui.items) > 0 {
		m.ui.selID = m.ui.items[m.ui.cursor].ID()
	} else {
		m.ui.selID = ""
	}

	for i := range m.ui.items {
		setGitBranch(&m.ui.items[i])
	}
}

func (m *model) refilterFromQuery() {
	m.refilter()
	m.ui.cursor = 0
	if len(m.ui.items) > 0 {
		m.ui.selID = m.ui.items[0].ID()
	} else {
		m.ui.selID = ""
	}
}

func (m *model) totalCount() int {
	return countSources(m.bySrc)
}

func (m model) Init() tea.Cmd {
	var cmds []tea.Cmd
	cmds = append(cmds, refreshCmds(m.sources)...)
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case sourceMsg:
		if len(msg.items) == 0 {
			return m, nil
		}
		m.mergeSource(msg.src, msg.items)
		m.refilter()
		return m, nil

	case tea.WindowSizeMsg:
		m.ui.width, m.ui.height = msg.Width, msg.Height
		// inline mode: keep list short like fzf --height
		if m.ui.maxShow <= 0 {
			m.ui.maxShow = 12
		}
		return m, nil

	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c":
			m.ui.done = Result{Action: ActionQuit}
			return m, tea.Quit

		case "esc":
			// display-popup + bind -n M-* : releasing Alt often injects ESC into the
			// new pane and would quit instantly. Ignore brief false ESC after open.
			if time.Since(m.ui.started) < 500*time.Millisecond {
				return m, nil
			}
			m.ui.done = Result{Action: ActionQuit}
			return m, tea.Quit

		case "?":
			m.ui.help = !m.ui.help
			return m, nil

		case "ctrl+t": // sticky <- shape from selection; Create/Zox use it
			if len(m.ui.items) > 0 {
				it := m.ui.items[m.ui.cursor]
				var p *store.Preset
				var err error
				switch it.Kind {
				case KindPreset:
					p, err = m.store.Get(it.Name)
				case KindActive:
					p, err = m.ctl.Freeze(it.Name)
				default:
					if err := template.ResetActive(m.store); err != nil {
						m.ui.status = err.Error()
					} else {
						m.tmpl = "default"
						m.ui.status = "sticky: default"
					}
					return m, nil
				}
				if err != nil {
					m.ui.status = err.Error()
					return m, nil
				}
				id, created, err := template.StickFrom(m.store, p)
				if err != nil {
					m.ui.status = err.Error()
					return m, nil
				}
				m.tmpl = template.StickyLabel(m.store)
				if m.tmpl == "" || m.tmpl == id {
					m.tmpl = template.ShapeLabel(template.ToShape(p, id))
				}
				if created {
					m.ui.status = "sticky <- " + m.tmpl + "  (new)"
				} else {
					m.ui.status = "sticky <- " + m.tmpl
				}
				return m, nil
			}
			if err := template.ResetActive(m.store); err != nil {
				m.ui.status = err.Error()
			} else {
				m.tmpl = "default"
				m.ui.status = "sticky: default"
			}
			return m, nil

		case "enter":
			if len(m.ui.items) > 0 && m.ui.cursor < len(m.ui.items) {
				m.ui.done = Result{Action: ActionConnect, Item: m.ui.items[m.ui.cursor]}
				m.ui.query = ""
				m.ui.items = m.ui.items[:0]
				return m, tea.Quit
			}

		case "ctrl+n", "down":
			if len(m.ui.items) > 0 {
				m.ui.cursor = (m.ui.cursor + 1) % len(m.ui.items)
				m.ui.selID = m.ui.items[m.ui.cursor].ID()
			}
			return m, nil

		case "ctrl+p", "up":
			if len(m.ui.items) > 0 {
				m.ui.cursor--
				if m.ui.cursor < 0 {
					m.ui.cursor = len(m.ui.items) - 1
				}
				m.ui.selID = m.ui.items[m.ui.cursor].ID()
			}
			return m, nil

		case "ctrl+u":
			m.ui.query = ""
			m.refilterFromQuery()
			return m, nil

		case "ctrl+w":
			m.ui.query = trimLastWord(m.ui.query)
			m.refilterFromQuery()
			return m, nil

		case "backspace":
			if len(m.ui.query) > 0 {
				// drop last rune
				r := []rune(m.ui.query)
				m.ui.query = string(r[:len(r)-1])
				m.refilterFromQuery()
			}
			return m, nil

		case "ctrl+x": // kill active
			if len(m.ui.items) > 0 {
				it := m.ui.items[m.ui.cursor]
				if it.Kind == KindActive {
					if err := m.ctl.Kill(it.Name); err != nil {
						m.ui.status = err.Error()
					} else {
						if m.store != nil {
							_ = m.store.RecordKill(it.Name)
						}
						m.ui.status = "killed " + it.Name
						m.cache.invalidate()
						m.reload()
					}
				}
			}
			return m, nil

		case "ctrl+f": // freeze instance+shape; sticky stays intentional (^t)
			if len(m.ui.items) > 0 {
				it := m.ui.items[m.ui.cursor]
				name := it.Name
				if it.Kind == KindActive || (it.Kind == KindPreset && m.ctl.Has(name)) {
					stop := HoldInterrupt()
					sid, created, err := template.FreezeRemember(m.ctl, m.store, name)
					stop()
					if err != nil {
						m.ui.status = err.Error()
						return m, nil
					}
					if created {
						m.ui.status = "froze " + name + " | shape " + sid
					} else if sid != "" {
						m.ui.status = "froze " + name + " | shape " + sid + " (exists)"
					} else {
						m.ui.status = "froze " + name
					}
					m.cache.invalidate()
					m.reload()
				} else if it.Kind == KindPreset {
					m.ui.status = "session not running - attach first"
				}
			}
			return m, nil

		case "ctrl+e": // edit preset (Active: freeze to preset first)
			if len(m.ui.items) == 0 {
				return m, nil
			}
			it := m.ui.items[m.ui.cursor]
			switch it.Kind {
			case KindActive:
				stop := HoldInterrupt()
				_, _, err := template.FreezeRemember(m.ctl, m.store, it.Name)
				stop()
				if err != nil {
					m.ui.status = err.Error()
					return m, nil
				}
				cmd, err := m.beginEdit(it.Name)
				if err != nil {
					m.ui.status = err.Error()
					return m, nil
				}
				ClearInline(m.FrameLines())
				return m, cmd
			case KindPreset:
				cmd, err := m.beginEdit(it.Name)
				if err != nil {
					m.ui.status = err.Error()
					return m, nil
				}
				ClearInline(m.FrameLines())
				return m, cmd
			default:
				m.ui.status = "edit: pick Active or Preset"
			}
			return m, nil

		case "ctrl+d": // delete preset
			if len(m.ui.items) > 0 {
				it := m.ui.items[m.ui.cursor]
				if it.Kind == KindPreset {
					if err := m.store.Delete(it.Name); err != nil {
						m.ui.status = err.Error()
					} else {
						m.ui.status = "deleted " + it.Name
						m.cache.invalidate()
						m.reload()
					}
				}
			}
			return m, nil

		default:
			// unmapped ctrl/alt chords: ignore (don't leak into query)
			if isModifierChord(msg) {
				return m, nil
			}
			// plain printable -> filter
			if text := msg.Key().Text; text != "" {
				for _, r := range text {
					if unicode.IsPrint(r) {
						m.ui.query += string(r)
					}
				}
				m.refilterFromQuery()
			}
		}

	case editDoneMsg:
		// editor left junk below; wipe so repaint is single frame
		ClearInline(m.FrameLines())
		if msg.err != nil {
			m.ui.status = msg.err.Error()
		} else {
			m.ui.status = "saved " + msg.name
			m.cache.invalidate()
			m.reload()
		}
		return m, nil
	}
	return m, nil
}

func (m *model) reload() {
	savedScroll := m.ui.scrollOff()
	m.sources = defaultSources(m.ctl, m.store, m.createName, m.createCwd, &m.cache)
	m.bySrc = snapshotAll(m.sources)
	m.env = newContext(m.ctl, m.store)
	applyRankMeta(m.bySrc, m.store, m.env)
	enrichAllSync(m.bySrc)
	m.refilter()
	// Actions (kill/freeze/delete/edit) change list length → preserve scroll.
	if savedScroll > 0 && savedScroll != m.ui.scrollOff() {
		half := m.ui.maxShow / 2
		c := savedScroll + half
		if c >= len(m.ui.items) {
			c = len(m.ui.items) - 1
		}
		if c >= 0 {
			m.ui.cursor = c
			m.ui.selID = m.ui.items[c].ID()
		}
	}
}

// FrameLines is fixed height of View - wipe residual inline UI after quit.
func (m model) FrameLines() int {
	maxShow := m.ui.maxShow
	if maxShow <= 0 {
		maxShow = 12
	}
	// prompt line + header + list + status
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
	dir, err := store.DataDir()
	if err != nil {
		return nil, err
	}
	f, err := os.CreateTemp(dir, "edit-*.json")
	if err != nil {
		return nil, err
	}
	path := f.Name()
	if _, err := f.WriteString(template.Format(p)); err != nil {
		f.Close()
		os.Remove(path)
		return nil, err
	}
	f.Close()
	m.ui.editPath = path
	m.ui.editOld = name

	c := editorCmd(path)
	if c.Stdin == nil {
		c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	}
	st := m.store
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
		np, err := template.Parse(string(raw))
		if err != nil {
			return editDoneMsg{err: fmt.Errorf("parse: %w", err)}
		}
		stop := HoldInterrupt()
		err = template.CommitEdit(st, old, np)
		stop()
		if err != nil {
			return editDoneMsg{err: err}
		}
		return editDoneMsg{name: np.Name}
	}), nil
}

// editorCmd opens path in $EDITOR (default nvim).
func editorCmd(path string) *exec.Cmd {
	ed := os.Getenv("EDITOR")
	if ed == "" {
		ed = os.Getenv("VISUAL")
	}
	if ed == "" {
		ed = "nvim"
	}
	if fields := strings.Fields(ed); len(fields) > 1 {
		return exec.Command(fields[0], append(fields[1:], path)...)
	}
	return exec.Command(ed, path)
}

func trimLastWord(s string) string {
	s = strings.TrimRightFunc(s, unicode.IsSpace)
	i := strings.LastIndexFunc(s, unicode.IsSpace)
	if i < 0 {
		return ""
	}
	return s[:i+1]
}

func (m model) View() tea.View {
	var b strings.Builder

	// First line: continuation prompt + query (giống gõ ở shell prompt).
	b.WriteString(styleDim.Render(iconPrompt()))
	b.WriteString(m.ui.query)
	b.WriteByte('\n')
	// Second line: header với count + meta + keys.
	meta := fmt.Sprintf("  %d/%d", len(m.ui.items), m.totalCount())
	if m.ui.help {
		meta += "  ^n/p | enter | ^t sticky | ^x kill | ^f freeze | ^e edit | ^d del | ^u/^w | esc"
	} else if m.tmpl != "" && m.tmpl != "default" {
		meta += formatStickyMeta(m.tmpl) + "  enter | esc | ?"
	} else {
		meta += "  enter | esc | ?"
	}
	b.WriteString(styleHeader.Render(meta))
	b.WriteByte('\n')

	maxShow := m.ui.maxShow
	if maxShow <= 0 {
		maxShow = 12
	}

	shown := 0
	if len(m.ui.items) == 0 {
		b.WriteString(styleDim.Render("  (no match)"))
		b.WriteByte('\n')
		shown = 1
	} else {
		half := maxShow / 2
		start := m.ui.cursor - half
		if start < 0 {
			start = 0
		}
		if start+maxShow > len(m.ui.items) {
			start = len(m.ui.items) - maxShow
		}
		if start < 0 {
			start = 0
		}
		end := start + maxShow
		if end > len(m.ui.items) {
			end = len(m.ui.items)
		}

		for i := start; i < end; i++ {
			it := m.ui.items[i]
			line := it.Title
			if it.GitBranch != "" {
				line += " (" + it.GitBranch + ")"
			}
			if it.Desc != "" {
				titleW := lipgloss.Width(line)
				if titleW < 44 {
					line += strings.Repeat(" ", 44-titleW)
				} else {
					line = truncateRunes(line, 42)
					line += "  "
				}
				line += styleDim.Render(it.Desc)
			}
			if m.ui.width > 4 {
				line = truncateRunes(line, m.ui.width-2)
			}
			if it.ID() == m.ui.selID {
				b.WriteString(styleCursor.Render(iconCursor() + line))
			} else {
				b.WriteString(styleFor(it.Kind).Render("  " + line))
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

	// status always occupies 1 line - fixed frame height for clearInline
	if m.ui.status != "" {
		b.WriteString(styleStatus.Render(m.ui.status))
	}
	b.WriteByte('\n')
	return tea.NewView(b.String())
}


