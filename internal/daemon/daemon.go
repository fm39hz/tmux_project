package daemon

import (
	"context"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fm39hz/gotomux/internal/config"
	"github.com/fm39hz/gotomux/internal/store"
	"github.com/fm39hz/gotomux/internal/template"
	"github.com/fm39hz/gotomux/internal/tmux"
)

type Daemon struct {
	cc           *tmux.ControlConn
	ctl          *tmux.Ctl
	st           *store.Store
	stMu         sync.Mutex
	cfg          *config.Config
	lastSeen     map[string]int64
	lastSeenMu   sync.Mutex
	stateVersion atomic.Int64

	cachedSessions []tmux.LiveSession
	cachedPresets  []store.PresetMeta
	cachedPairs    map[string]int64
	cachedUsage    map[string]store.Usage
	cachedSticky   string
	cacheMu        sync.RWMutex

	stopCh  chan struct{}
	sockPath string
	wg       sync.WaitGroup

	storeErrs atomic.Int64
	startedAt time.Time
}

func New(cfg *config.Config) (*Daemon, error) {
	// Step 1: setup tmux server with correct options BEFORE -C connects.
	// exit-empty off ensures server stays alive; .gotomuxd prevents auto-creation.
	exec.Command("tmux", "start-server").Run()
	exec.Command("tmux", "set-option", "-g", "exit-empty", "off").Run()
	exec.Command("tmux", "new-session", "-d", "-s", ".gotomuxd").Run()

	// Step 2: connect control client to the now-stable server.
	cc, err := tmux.StartControl()
	if err != nil {
		return nil, err
	}
	ctl, err := tmux.New()
	if err != nil {
		cc.Close()
		return nil, err
	}
	stDir := cfg.ResolveDataDir()
	if err := os.MkdirAll(stDir, 0o755); err != nil {
		cc.Close()
		return nil, err
	}
	st, err := store.OpenWithConfig(cfg)
	if err != nil {
		cc.Close()
		return nil, err
	}

	ipcDir := os.Getenv("XDG_DATA_HOME")
	if ipcDir == "" {
		home, _ := os.UserHomeDir()
		ipcDir = filepath.Join(home, ".local", "share")
	}
	ipcSock := filepath.Join(ipcDir, "gotomux", "gotomux.sock")

	d := &Daemon{
		cc: cc, ctl: ctl, st: st, cfg: cfg,
		lastSeen: map[string]int64{}, sockPath: ipcSock,
		stopCh: make(chan struct{}), startedAt: time.Now(),
	}
	d.syncZoxide()
	d.syncNow()
	d.wg.Add(1)
	go d.pollLoop()
	return d, nil
}

func (d *Daemon) Close() {
	close(d.stopCh)
	d.wg.Wait()
	if d.st != nil {
		d.st.Close()
	}
	if d.cc != nil {
		d.cc.Close()
	}
	exec.Command("tmux", "kill-session", "-t", ".gotomuxd").Run()
}

func (d *Daemon) ensureDB() {
	d.stMu.Lock()
	defer d.stMu.Unlock()
	if d.st == nil { return }
	if err := d.st.Ping(); err != nil {
		d.storeErrs.Add(1)
		log.Printf("[store] [ERROR] ping: %v — reopening", err)
		d.st.Close()
		if st, err := store.OpenWithConfig(d.cfg); err == nil {
			d.st = st
		} else {
			d.st = nil
		}
	}
}

func (d *Daemon) ensureSocket() {
	if _, err := os.Stat(d.sockPath); err != nil {
		log.Printf("[ipc] [WARN] socket %s missing", d.sockPath)
	}
}

var listArgs = []string{"list-sessions", "-F", tmux.ListSessFmt, ";", "list-panes", "-s", "-F", tmux.ListPanesFmt}

func (d *Daemon) listLive() []tmux.LiveSession {
	raw, err := d.cc.Send(context.Background(), listArgs...)
	if err != nil {
		log.Printf("[tmux] [ERR] list: %v", err)
		return nil
	}
	sessions := tmux.ParseLiveOutput(raw)
	filtered := sessions[:0]
	for _, s := range sessions {
		if !strings.HasPrefix(s.Name, ".") {
			filtered = append(filtered, s)
		}
	}
	return filtered
}

func (d *Daemon) syncNow() {
	sessions := d.listLive()
	if sessions == nil {
		return
	}
	changed := false

	d.lastSeenMu.Lock()
	for _, s := range sessions {
		if prev, ok := d.lastSeen[s.Name]; !ok || s.LastAttached > prev {
			d.recordTelemetry(s.Name, sessions)
			changed = true
		}
		d.lastSeen[s.Name] = s.LastAttached
	}
	for name := range d.lastSeen {
		keep := false
		for _, s := range sessions {
			if s.Name == name { keep = true; break }
		}
		if !keep {
			delete(d.lastSeen, name)
			changed = true
		}
	}
	d.lastSeenMu.Unlock()

	d.cacheMu.Lock()
	d.cachedSessions = sessions
	d.stMu.Lock()
	if d.st != nil {
		d.cachedUsage, _ = d.st.AllUsage()
		if pm, err := d.st.ListMeta(); err == nil {
			d.cachedPresets = pm
		}
		d.cachedSticky = template.StickyLabel(d.st)
	} else {
		d.cachedUsage = nil
		d.cachedPresets = nil
		d.cachedSticky = ""
	}
	d.cachedPairs = nil
	d.stMu.Unlock()
	d.cacheMu.Unlock()

	if changed {
		d.stateVersion.Add(1)
	}
}

func (d *Daemon) recordTelemetry(name string, all []tmux.LiveSession) {
	d.stMu.Lock()
	st := d.st
	d.stMu.Unlock()
	if st == nil { return }
	st.RecordOpen(name)
	others := make([]string, 0, len(all))
	for _, s := range all {
		if s.Name != name { others = append(others, s.Name) }
	}
	if len(others) > 0 {
		st.RecordPairsWithLive(name, others)
	}
}

func (d *Daemon) syncZoxide() {
	d.stMu.Lock()
	st := d.st
	d.stMu.Unlock()
	if st == nil { return }
	out, err := exec.Command("zoxide", "query", "-l").Output()
	if err != nil { return }
	now := time.Now().Unix()
	var rows []store.ZoxRow
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" { continue }
		name := filepath.Base(line)
		if name == "" || name == "." || name == "/" { continue }
		rows = append(rows, store.ZoxRow{Name: name, Path: line, Title: "[Zoxide] " + name, Recency: now})
	}
	if len(rows) > 0 {
		st.SaveZox(rows)
	}
}

func (d *Daemon) pollLoop() {
	defer d.wg.Done()
	interval := 10 * time.Second
	if d.cfg != nil { interval = d.cfg.PollInterval }
	for {
		select {
		case <-d.stopCh:
			return
		case <-time.After(interval):
			if !d.cc.IsAlive() {
				log.Printf("[tmux] [WARN] -C died, reconnecting")
				if cc, err := tmux.StartControl(); err != nil {
					log.Printf("[tmux] [ERROR] reconnect: %v", err)
				} else {
					d.cc.Close()
					d.cc = cc
				}
			}
			d.ensureDB()
			d.ensureSocket()
			d.syncNow()
		}
	}
}
