package daemon

import (
	"log"
	"os/exec"
	"sync"
	"time"

	"github.com/fm39hz/gotomux/internal/store"
	"github.com/fm39hz/gotomux/internal/tmux"
)

// Daemon: warm infra + background telemetry. No IPC.
type Daemon struct {
	mu       sync.Mutex
	ctl      *tmux.Ctl
	st       *store.Store
	lastSeen map[string]int64
}

func New() (*Daemon, error) {
	exec.Command("tmux", "start-server").Run()
	exec.Command("tmux", "set-option", "-g", "exit-empty", "off").Run()

	var (
		ctl    *tmux.Ctl
		st     *store.Store
		ctlErr error
		stErr  error
	)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		ctl, ctlErr = tmux.New()
	}()
	go func() {
		defer wg.Done()
		st, stErr = store.Open()
	}()
	wg.Wait()
	if ctlErr != nil {
		return nil, ctlErr
	}
	if stErr != nil {
		return nil, stErr
	}
	d := &Daemon{ctl: ctl, st: st, lastSeen: map[string]int64{}}
	d.syncNow()
	go d.pollLoop()
	return d, nil
}

func (d *Daemon) Close() {
	if d.st != nil {
		d.st.Close()
	}
}

// syncNow polls tmux + SQLite, records telemetry for session switches.
func (d *Daemon) syncNow() {
	sessions, err := d.ctl.ListLive()
	if err != nil {
		return
	}
	d.mu.Lock()
	seen := d.lastSeen
	d.mu.Unlock()

	for _, s := range sessions {
		prev, ok := seen[s.Name]
		if !ok || s.LastAttached > prev {
			d.recordTelemetry(s.Name, sessions)
		}
	}

	d.mu.Lock()
	for _, s := range sessions {
		d.lastSeen[s.Name] = s.LastAttached
	}
	for name := range d.lastSeen {
		found := false
		for _, s := range sessions {
			if s.Name == name {
				found = true
				break
			}
		}
		if !found {
			delete(d.lastSeen, name)
		}
	}
	d.mu.Unlock()
}

func (d *Daemon) recordTelemetry(name string, all []tmux.LiveSession) {
	if d.st == nil {
		return
	}
	log.Printf("telemetry: %s", name)
	d.st.RecordOpen(name)
	others := make([]string, 0, len(all))
	for _, s := range all {
		if s.Name != name {
			others = append(others, s.Name)
		}
	}
	d.st.RecordPairsWithLive(name, others)
}

func (d *Daemon) pollLoop() {
	for {
		time.Sleep(2 * time.Second)
		d.syncNow()
	}
}
