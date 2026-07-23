package template

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fm39hz/gotomux/internal/config"
	"github.com/fm39hz/gotomux/internal/store"
)

func configBaseDir(cfg *config.Config) string {
	if cfg != nil {
		return cfg.ResolveConfigDir()
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "gotomux")
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".config", "gotomux")
	}
	return ""
}

func configShapesDir(cfg *config.Config) string {
	base := configBaseDir(cfg)
	if base == "" {
		return ""
	}
	dir := filepath.Join(base, "shapes")
	legacy := filepath.Join(base, "layouts")
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		if st, err := os.Stat(legacy); err == nil && st.IsDir() {
			_ = os.Rename(legacy, dir)
		}
	}
	return dir
}

func shapeFilePath(id, label string) string {
	dir := configShapesDir(nil)
	if dir == "" || id == "" {
		return ""
	}
	if id == "default" {
		return filepath.Join(dir, "default.json")
	}
	lab := LabelFileSlug(label)
	if lab == "" || lab == "shape" {
		lab = "shape"
	}
	suf := id
	if strings.HasPrefix(id, "shape-") && len(id) >= 14 {
		suf = id[len(id)-8:]
	} else if len(suf) > 8 {
		suf = suf[len(suf)-8:]
	}
	return filepath.Join(dir, lab+"--"+suf+".json")
}

// writeFileAtomic writes data to path via a temp file + rename.
// Prevents partial writes if the process crashes mid-write.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func writeConfigMirror(id, body string) {
	if id == "" || body == "" {
		return
	}
	label := shapeLabelFromBody(id, body)
	path := shapeFilePath(id, label)
	if path == "" {
		return
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	_ = writeFileAtomic(path, []byte(body), 0o644)
}

func shapeLabelFromBody(id, body string) string {
	if pr, err := Parse(body); err == nil {
		pr = ToShape(pr, id)
		pr.Name = id
		return ShapeLabel(pr)
	}
	if id == "default" {
		return "default"
	}
	return "shape"
}

func reconcileConfigShapes(st store.Storer) {
	if st == nil {
		return
	}
	dir := configShapesDir(nil)
	if dir == "" {
		return
	}
	_ = os.MkdirAll(dir, 0o755)
	ids, err := st.ListShapes()
	if err != nil {
		return
	}
	keep := map[string]bool{}
	for _, id := range ids {
		body, ok := st.GetShape(id)
		if !ok {
			continue
		}
		if clean := normalizeShapeBody(id, body); clean != "" {
			if clean != body {
				pure := mustParseShape(id, clean)
				if err := st.UpsertShapeByID(id, ShapeKey(pure), clean); err != nil {
					log.Printf("upsert shape: %v", err)
				}
				body = clean
			}
		}
		label := shapeLabelFromBody(id, body)
		path := shapeFilePath(id, label)
		if path == "" {
			continue
		}
		_ = writeFileAtomic(path, []byte(body), 0o644)
		keep[filepath.Base(path)] = true
	}
	ents, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		if !keep[e.Name()] {
			_ = os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}

var syncOnce sync.Once

func syncConfigToDB(st store.Storer) {
	if st == nil {
		return
	}
	syncOnce.Do(func() {
		dir := configShapesDir(nil)
		seenFile := map[string]bool{}
		if dir != "" {
			ents, err := os.ReadDir(dir)
			if err == nil {
				for _, e := range ents {
					if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
						continue
					}
					path := filepath.Join(dir, e.Name())
					raw, err := os.ReadFile(path)
					if err != nil {
						continue
					}
					fi, err := os.Stat(path)
					if err != nil {
						continue
					}
					mtime := fi.ModTime().Unix()
					pr, err := Parse(string(raw))
					if err != nil {
						continue
					}
					id := pr.Name
					if !isShapeID(id) {
						stem := strings.TrimSuffix(e.Name(), ".json")
						if isShapeID(stem) {
							id = stem
						} else if i := strings.LastIndex(stem, "--"); i >= 0 {
							suf := stem[i+2:]
							id = "shape-" + suf
							if ids, _ := st.ListShapes(); len(ids) > 0 {
								for _, cand := range ids {
									if strings.HasSuffix(cand, suf) || cand == id {
										id = cand
										break
									}
								}
							}
						} else if stem == "default" {
							id = "default"
						} else {
							continue
						}
					}
					pure := ToShape(pr, id)
					pure.Name = id
					key := ShapeKey(pure)
					body := Format(pure)
					seenFile[id] = true

					dbBody, dbUpd, ok := st.GetShapeMeta(id)
					if !ok {
						if err := st.UpsertShapeByID(id, key, body); err != nil {
							log.Printf("upsert shape: %v", err)
						}
						continue
					}
					if mtime > dbUpd {
						if err := st.UpsertShapeByID(id, key, body); err != nil {
							log.Printf("upsert shape: %v", err)
						}
					} else if body != dbBody {
						writeConfigMirror(id, dbBody)
					}
				}
			}
		}
		_ = ensureDefault(st)
	})
}

func mirrorAfter(st store.Storer, _ string) { reconcileConfigShapes(st) }

func ensureShapesReady(st store.Storer) { syncConfigToDB(st) }
