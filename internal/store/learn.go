package store

import (
	"strings"
	"time"
)

// RecordPlacement bumps learned pane-slot pattern for a shape (silent).
func (s *Store) RecordPlacement(shapeID, pattern string) error {
	shapeID, pattern = strings.TrimSpace(shapeID), strings.TrimSpace(pattern)
	if shapeID == "" || pattern == "" {
		return nil
	}
	now := time.Now().Unix()
	_, err := s.db.Exec(`
INSERT INTO placement(shape_id, pattern, n, last) VALUES(?, ?, 1, ?)
ON CONFLICT(shape_id, pattern) DO UPDATE SET
  n = n + 1,
  last = excluded.last
`, shapeID, pattern, now)
	return err
}

// BestPlacement returns highest-count pattern for shape (tie -> newest last).
// ok=false if none or weak (n < 1 - any observation counts for v1).
func (s *Store) BestPlacement(shapeID string) (pattern string, ok bool) {
	shapeID = strings.TrimSpace(shapeID)
	if shapeID == "" {
		return "", false
	}
	err := s.db.QueryRow(`
SELECT pattern FROM placement
WHERE shape_id = ?
ORDER BY n DESC, last DESC
LIMIT 1
`, shapeID).Scan(&pattern)
	if err != nil || pattern == "" {
		return "", false
	}
	return pattern, true
}

// RecordFork bumps a window-level essence unit (silent fork learning).
// key fingerprints topology+tools of one window; body is product JSON for that window.
func (s *Store) RecordFork(key, body string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil
	}
	now := time.Now().Unix()
	_, err := s.db.Exec(`
INSERT INTO fork(key, body, n, last) VALUES(?, ?, 1, ?)
ON CONFLICT(key) DO UPDATE SET
  n = n + 1,
  last = excluded.last,
  body = CASE WHEN excluded.body != '' THEN excluded.body ELSE body END
`, key, body, now)
	return err
}

// ForkHits returns how often a window key has been observed (0 if unknown).
func (s *Store) ForkHits(key string) int64 {
	key = strings.TrimSpace(key)
	if key == "" {
		return 0
	}
	var n int64
	_ = s.db.QueryRow(`SELECT n FROM fork WHERE key = ?`, key).Scan(&n)
	return n
}
