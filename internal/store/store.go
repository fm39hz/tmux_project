package store

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type Preset struct {
	Name    string
	Cwd     string // session root
	Windows []PresetWindow
}

type PresetWindow struct {
	Idx    int
	Name   string
	Cwd    string // window working dir (usually first/active pane)
	Layout string
	Panes  []PresetPane
}

type PresetPane struct {
	Idx int
	Cwd string
	Cmd string // detected tool: nvim, lazygit, ... empty = default shell
}

type Store struct {
	db *sql.DB
}

func Open() (*Store, error) {
	dir, err := DataDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	// One-shot: empty gotomux DB + legacy state → copy presets over.
	_ = migrateLegacyDB(dir)
	db, err := sql.Open("sqlite", filepath.Join(dir, "state.db"))
	if err != nil {
		return nil, err
	}
	// Single writer is enough; keep pool tiny for a short-lived CLI.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	s := &Store{db: db}
	if err := s.pragma(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// pragma: personal-tool hygiene — WAL + reasonable sync + busy wait.
func (s *Store) pragma() error {
	// modernc accepts these; ignore failures on exotic builds
	pragmas := []string{
		`PRAGMA foreign_keys = ON`,
		`PRAGMA journal_mode = WAL`,
		`PRAGMA synchronous = NORMAL`,
		`PRAGMA busy_timeout = 1000`,
		`PRAGMA temp_store = MEMORY`,
	}
	for _, p := range pragmas {
		if _, err := s.db.Exec(p); err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
	}
	return nil
}

func DataDir() (string, error) {
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "gotomux"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "gotomux"), nil
}

// migrateLegacyDB copies state.db from old product dirs once when gotomux has no sessions.
// Does not keep reading legacy paths afterward.
func migrateLegacyDB(dir string) error {
	dst := filepath.Join(dir, "state.db")
	// already has content?
	if hasSessionRows(dst) {
		return nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	roots := []string{filepath.Join(home, ".local", "share")}
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		roots = append([]string{xdg}, roots...)
	}
	var src string
	for _, root := range roots {
		for _, leg := range []string{"tmux_project", "go-tomux"} {
			p := filepath.Join(root, leg, "state.db")
			if hasSessionRows(p) {
				src = p
				break
			}
		}
		if src != "" {
			break
		}
	}
	if src == "" {
		return nil
	}
	// close any WAL on source by best-effort copy of main file only is wrong if WAL pending;
	// use sqlite backup via file copy of db+wal+shm when present.
	for _, suf := range []string{"", "-wal", "-shm"} {
		s := src + suf
		d := dst + suf
		if _, err := os.Stat(s); err != nil {
			continue
		}
		if err := copyFile(s, d); err != nil {
			return err
		}
	}
	return nil
}

func hasSessionRows(dbPath string) bool {
	st, err := os.Stat(dbPath)
	if err != nil || st.Size() == 0 {
		return false
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return false
	}
	defer db.Close()
	var n int
	err = db.QueryRow(`SELECT COUNT(*) FROM session`).Scan(&n)
	return err == nil && n > 0
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
PRAGMA foreign_keys = ON;
CREATE TABLE IF NOT EXISTS session (
  name       TEXT PRIMARY KEY,
  cwd        TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  last_used  INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS window (
  id      INTEGER PRIMARY KEY,
  session TEXT NOT NULL REFERENCES session(name) ON DELETE CASCADE,
  idx     INTEGER NOT NULL,
  name    TEXT NOT NULL DEFAULT '',
  cwd     TEXT NOT NULL DEFAULT '',
  layout  TEXT,
  UNIQUE(session, idx)
);
CREATE TABLE IF NOT EXISTS pane (
  id        INTEGER PRIMARY KEY,
  window_id INTEGER NOT NULL REFERENCES window(id) ON DELETE CASCADE,
  idx       INTEGER NOT NULL,
  cwd       TEXT,
  cmd       TEXT,
  UNIQUE(window_id, idx)
);
`)
	if err != nil {
		return err
	}
	// soft-migrate older DBs missing window.cwd
	var n int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('window') WHERE name='cwd'`).Scan(&n)
	if n == 0 {
		_, _ = s.db.Exec(`ALTER TABLE window ADD COLUMN cwd TEXT NOT NULL DEFAULT ''`)
	}
	if _, err = s.db.Exec(`
CREATE TABLE IF NOT EXISTS usage (
  name      TEXT PRIMARY KEY,
  opens     INTEGER NOT NULL DEFAULT 0,
  kills     INTEGER NOT NULL DEFAULT 0,
  last_open INTEGER NOT NULL DEFAULT 0,
  last_kill INTEGER NOT NULL DEFAULT 0
);`); err != nil {
		return err
	}
	if _, err = s.db.Exec(`
CREATE TABLE IF NOT EXISTS pair (
  a        TEXT NOT NULL,
  b        TEXT NOT NULL,
  n        INTEGER NOT NULL DEFAULT 0,
  last     INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (a, b)
);`); err != nil {
		return err
	}
	if _, err = s.db.Exec(`
CREATE TABLE IF NOT EXISTS zox_meta (
  id      INTEGER PRIMARY KEY CHECK (id = 1),
  updated INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS zox_item (
  ord     INTEGER PRIMARY KEY,
  name    TEXT NOT NULL,
  path    TEXT NOT NULL,
  title   TEXT NOT NULL DEFAULT '',
  desc    TEXT NOT NULL DEFAULT '',
  recency INTEGER NOT NULL DEFAULT 0
);`); err != nil {
		return err
	}
	if _, err = s.db.Exec(`
CREATE TABLE IF NOT EXISTS shape (
  id         TEXT PRIMARY KEY,
  key        TEXT NOT NULL UNIQUE,
  body       TEXT NOT NULL,
  created_at INTEGER NOT NULL DEFAULT 0,
  updated_at INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS sticky (
  id       INTEGER PRIMARY KEY CHECK (id = 1),
  shape_id TEXT NOT NULL DEFAULT 'default'
);`); err != nil {
		return err
	}
	// old DBs: shape without updated_at (CREATE IF NOT EXISTS does not alter)
	var shapeHasUpdated int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('shape') WHERE name='updated_at'`).Scan(&shapeHasUpdated)
	if shapeHasUpdated == 0 {
		_, _ = s.db.Exec(`ALTER TABLE shape ADD COLUMN updated_at INTEGER NOT NULL DEFAULT 0`)
		_, _ = s.db.Exec(`UPDATE shape SET updated_at = created_at WHERE updated_at = 0`)
	}
	if _, err = s.db.Exec(`
CREATE TABLE IF NOT EXISTS placement (
  shape_id TEXT NOT NULL,
  pattern  TEXT NOT NULL,
  n        INTEGER NOT NULL DEFAULT 0,
  last     INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (shape_id, pattern)
);`); err != nil {
		return err
	}
	// fork: window-level essence units learned from freeze/sticky (silent).
	// key = paneCount|split|tools — same key = same reusable mảnh.
	if _, err = s.db.Exec(`
CREATE TABLE IF NOT EXISTS fork (
  key      TEXT PRIMARY KEY,
  body     TEXT NOT NULL DEFAULT '',
  n        INTEGER NOT NULL DEFAULT 0,
  last     INTEGER NOT NULL DEFAULT 0
);`); err != nil {
		return err
	}
	_, err = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_session_last_used ON session(last_used DESC)`)
	return err
}

