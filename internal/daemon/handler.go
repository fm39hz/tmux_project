package daemon

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"log"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/fm39hz/gotomux/internal/store"
	"github.com/fm39hz/gotomux/internal/tmux"
)

type Request struct {
	Cmd     string `json:"cmd"`
	Name    string `json:"name,omitempty"`
	Version int64  `json:"version,omitempty"`
}

type Response struct {
	OK      bool                `json:"ok"`
	Version int64               `json:"version,omitempty"`
	Error   string              `json:"error,omitempty"`
	Sessions []tmux.LiveSession `json:"sessions,omitempty"`
	Presets []store.PresetMeta  `json:"presets,omitempty"`
	Pairs   map[string]int64    `json:"pairs,omitempty"`
	Usage   map[string]store.Usage `json:"usage,omitempty"`
	StickyLabel string          `json:"sticky_label,omitempty"`
	StoreErrs int64             `json:"store_errs,omitempty"`
	Uptime    int64             `json:"uptime,omitempty"`
}

func listenWithGuard(sock string) (net.Listener, error) {
	l, err := net.Listen("unix", sock)
	if err == nil { return l, nil }
	if conn, dialErr := net.DialTimeout("unix", sock, 100*time.Millisecond); dialErr == nil {
		conn.Close()
		return nil, fmt.Errorf("daemon already running on %s", sock)
	}
	os.Remove(sock)
	l, err = net.Listen("unix", sock)
	if err != nil { return nil, fmt.Errorf("listen %s: %w", sock, err) }
	return l, nil
}

func acquireLock(sockPath string) (func(), error) {
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" { runtimeDir = os.TempDir() }
	_ = os.MkdirAll(runtimeDir, 0o750)
	h := fnv.New64a()
	_, _ = h.Write([]byte("gotomux-" + sockPath))
	lockPath := filepath.Join(runtimeDir, fmt.Sprintf("gotomux-%x.lock", h.Sum64()))
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil { return nil, fmt.Errorf("open lock: %w", err) }
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("daemon already running (lock %s)", lockPath)
	}
	return func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
		_ = os.Remove(lockPath)
	}, nil
}

func ServeIPC(d *Daemon) error {
	dir := os.Getenv("XDG_DATA_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".local", "share")
	}
	sock := filepath.Join(dir, "gotomux", "gotomux.sock")
	unlock, err := acquireLock(sock)
	if err != nil { return err }
	defer unlock()
	l, err := listenWithGuard(sock)
	if err != nil { return err }
	defer os.Remove(sock)
	for {
		conn, err := l.Accept()
		if err != nil { return err }
		go d.handleConn(conn)
	}
}

func (d *Daemon) handleConn(conn net.Conn) {
	defer conn.Close()
	var req Request
	if err := json.NewDecoder(conn).Decode(&req); err != nil {
		log.Printf("[ipc] decode: %v", err)
		json.NewEncoder(conn).Encode(Response{OK: false, Error: "bad request"})
		return
	}
	enc := json.NewEncoder(conn)
	switch req.Cmd {
	case "list":
		resp := d.buildListResponse()
		resp.Version = d.stateVersion.Load()
		enc.Encode(resp)
	default:
		enc.Encode(Response{OK: false, Error: "unknown cmd"})
	}
}

func (d *Daemon) buildListResponse() Response {
	d.cacheMu.RLock()
	sessions := d.cachedSessions
	presets := d.cachedPresets
	pairs := d.cachedPairs
	usage := d.cachedUsage
	sticky := d.cachedSticky
	d.cacheMu.RUnlock()
	return Response{OK: true, Sessions: sessions, Presets: presets,
		Pairs: pairs, Usage: usage, StickyLabel: sticky}
}
