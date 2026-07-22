package store

import "github.com/fm39hz/gotomux/internal/model"

func SessionToModel(p *Preset) *model.Session {
	if p == nil {
		return nil
	}
	s := &model.Session{Name: p.Name, Cwd: p.Cwd}
	for _, w := range p.Windows {
		mw := model.Window{Idx: w.Idx, Name: w.Name, Cwd: w.Cwd, Layout: w.Layout}
		for _, pn := range w.Panes {
			mw.Panes = append(mw.Panes, model.Pane{Idx: pn.Idx, Cwd: pn.Cwd, Cmd: pn.Cmd})
		}
		s.Windows = append(s.Windows, mw)
	}
	return s
}

func ModelToSession(s *model.Session) *Preset {
	if s == nil {
		return nil
	}
	p := &Preset{Name: s.Name, Cwd: s.Cwd}
	for _, w := range s.Windows {
		pw := PresetWindow{Idx: w.Idx, Name: w.Name, Cwd: w.Cwd, Layout: w.Layout}
		for _, pn := range w.Panes {
			pw.Panes = append(pw.Panes, PresetPane{Idx: pn.Idx, Cwd: pn.Cwd, Cmd: pn.Cmd})
		}
		p.Windows = append(p.Windows, pw)
	}
	return p
}

func usageToModel(u Usage) model.Usage {
	return model.Usage{Name: u.Name, Opens: u.Opens, Kills: u.Kills, LastOpen: u.LastOpen, LastKill: u.LastKill}
}

func usageFromModel(u model.Usage) Usage {
	return Usage{Name: u.Name, Opens: u.Opens, Kills: u.Kills, LastOpen: u.LastOpen, LastKill: u.LastKill}
}
