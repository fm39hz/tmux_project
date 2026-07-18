package template

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fm39hz/gotomux/internal/project"
	"github.com/fm39hz/gotomux/internal/store"
	"github.com/fm39hz/gotomux/internal/tmux"
)

// Templates live under $XDG_DATA_HOME/gotomux/templates/
//
//	default.json   — builtin shape (auto-seeded)
//	<name>.json    — derived from a preset via ctrl-t
//	active         — sticky template name (one line); empty/missing = default
//
// Create/Zoxide enter: live → named preset → active template @ cwd.

func builtinDefaultTemplate() *store.Preset {
	return &store.Preset{
		Name: "default",
		Windows: []store.PresetWindow{
			{Name: "editor", Panes: []store.PresetPane{{Cmd: "nvim"}}},
			{Name: "shell", Panes: []store.PresetPane{{}}},
		},
	}
}

func templatesDir() (string, error) {
	dir, err := store.DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "templates"), nil
}

func templateFile(name string) (string, error) {
	dir, err := templatesDir()
	if err != nil {
		return "", err
	}
	if name == "" {
		name = "default"
	}
	return filepath.Join(dir, name+".json"), nil
}

func activeNamePath() (string, error) {
	dir, err := templatesDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "active"), nil
}

func ReadActiveName() string {
	path, err := activeNamePath()
	if err != nil {
		return "default"
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "default"
	}
	name := strings.TrimSpace(string(b))
	if name == "" || !project.ValidSessionName(name) && name != "default" {
		// allow default always; other names must be safe filenames
		if name != "default" && !project.ValidSessionName(name) {
			return "default"
		}
	}
	if name == "" {
		return "default"
	}
	return name
}

func writeActiveName(name string) error {
	if name == "" {
		name = "default"
	}
	path, err := activeNamePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(name+"\n"), 0o644)
}

func saveTemplate(p *store.Preset) error {
	if p == nil || p.Name == "" {
		return fmt.Errorf("template needs name")
	}
	path, err := templateFile(p.Name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	// formatPreset keeps layout named-only; strip session cwd for template file
	cp := *p
	cp.Cwd = ""
	return os.WriteFile(path, []byte(Format(&cp)), 0o644)
}

func loadTemplateFile(name string) (*store.Preset, error) {
	if name == "" || name == "default" {
		return loadDefaultTemplate()
	}
	path, err := templateFile(name)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(string(raw))
}

// loadDefaultTemplate reads templates/default.json, or seeds builtin.
func loadDefaultTemplate() (*store.Preset, error) {
	path, err := templateFile("default")
	if err != nil {
		return builtinDefaultTemplate(), nil
	}
	raw, err := os.ReadFile(path)
	if err == nil {
		p, err := Parse(string(raw))
		if err != nil {
			return nil, fmt.Errorf("template %s: %w", path, err)
		}
		return p, nil
	}
	if !os.IsNotExist(err) {
		return nil, err
	}
	p := builtinDefaultTemplate()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return p, nil
	}
	_ = os.WriteFile(path, []byte(Format(p)), 0o644)
	return p, nil
}

// loadActiveTemplate resolves sticky name → template shape.
func LoadActive() (*store.Preset, string, error) {
	name := ReadActiveName()
	p, err := loadTemplateFile(name)
	if err != nil {
		// missing file → fall back default, clear sticky
		_ = writeActiveName("default")
		p, err2 := loadDefaultTemplate()
		return p, "default", err2
	}
	return p, name, nil
}

// relativizeCwd: abs under root → rel; empty/outside → "" (= $ROOT at bake).
func relativizeCwd(root, cwd string) string {
	if cwd == "" || root == "" {
		return ""
	}
	if !filepath.IsAbs(cwd) {
		return filepath.Clean(cwd)
	}
	rel, err := filepath.Rel(root, cwd)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return ""
	}
	if rel == "." {
		return ""
	}
	return rel
}

// presetToTemplate drops abs roots; paths become relative to preset.Cwd.
func presetToTemplate(p *store.Preset) *store.Preset {
	if p == nil {
		return builtinDefaultTemplate()
	}
	root := p.Cwd
	out := &store.Preset{Name: p.Name}
	if out.Name == "" {
		out.Name = "custom"
	}
	for i, w := range p.Windows {
		pw := store.PresetWindow{
			Idx:    i,
			Name:   w.Name,
			Cwd:    relativizeCwd(root, w.Cwd),
			Layout: tmux.LayoutForStore(w.Layout, len(w.Panes)),
		}
		if len(w.Panes) == 0 {
			pw.Panes = []store.PresetPane{{}}
		} else {
			for j, pn := range w.Panes {
				pw.Panes = append(pw.Panes, store.PresetPane{
					Idx: j,
					Cwd: relativizeCwd(root, pn.Cwd),
					Cmd: pn.Cmd,
				})
			}
		}
		out.Windows = append(out.Windows, pw)
	}
	if len(out.Windows) == 0 {
		return builtinDefaultTemplate()
	}
	return out
}

// setActiveFromPreset writes templates/<name>.json + sticky active.
func SetActiveFromPreset(p *store.Preset) (string, error) {
	t := presetToTemplate(p)
	if err := saveTemplate(t); err != nil {
		return "", err
	}
	if err := writeActiveName(t.Name); err != nil {
		return "", err
	}
	return t.Name, nil
}

func ResetActive() error {
	return writeActiveName("default")
}

// applyTemplate stamps a template onto a project root.
func Apply(tmpl *store.Preset, name, root string) *store.Preset {
	if root == "" {
		root, _ = os.Getwd()
	}
	p := &store.Preset{Name: name, Cwd: root}
	if tmpl == nil || len(tmpl.Windows) == 0 {
		tmpl = builtinDefaultTemplate()
	}
	for i, w := range tmpl.Windows {
		wcwd := resolveCwd(root, w.Cwd)
		pw := store.PresetWindow{
			Idx:    i,
			Name:   w.Name,
			Cwd:    wcwd,
			Layout: w.Layout,
		}
		if len(w.Panes) == 0 {
			pw.Panes = []store.PresetPane{{Cwd: wcwd}}
		} else {
			for j, pn := range w.Panes {
				cwd := pn.Cwd
				if cwd == "" {
					cwd = w.Cwd
				}
				pw.Panes = append(pw.Panes, store.PresetPane{
					Idx: j,
					Cwd: resolveCwd(root, cwd),
					Cmd: pn.Cmd,
				})
			}
		}
		p.Windows = append(p.Windows, pw)
	}
	return p
}

func resolveCwd(root, cwd string) string {
	if cwd == "" {
		return root
	}
	if filepath.IsAbs(cwd) {
		return cwd
	}
	return filepath.Join(root, cwd)
}

// connectProject: Create / Zoxide enter — zero prompts.
//
//	live? → attach
//	preset with same name? → bake that preset
//	else → active sticky template @ cwd
func ConnectProject(ctl *tmux.Ctl, st *store.Store, name, cwd string) error {
	if ctl.Has(name) {
		return ctl.Connect(name, "")
	}
	if p, err := st.Get(name); err == nil {
		_ = st.Touch(name)
		return ctl.ConnectPreset(p)
	}
	tmpl, _, err := LoadActive()
	if err != nil {
		return err
	}
	return ctl.ConnectPreset(Apply(tmpl, name, cwd))
}
