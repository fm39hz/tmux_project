package main

import (
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/junegunn/fzf/src/algo"

	"github.com/fm39hz/gotomux/internal/picker"
	"github.com/fm39hz/gotomux/internal/project"
	"github.com/fm39hz/gotomux/internal/store"
	"github.com/fm39hz/gotomux/internal/template"
	"github.com/fm39hz/gotomux/internal/tmux"
)

var version = "dev"
var errCancel = picker.ErrCancel

func init() { algo.Init("default") }

func main() {
	err := dispatch()
	if err == nil || errors.Is(err, errCancel) {
		os.Exit(0)
	}
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func dispatch() error {
	if len(os.Args) < 2 {
		return runPicker()
	}
	switch os.Args[1] {
	case "-f", "--freeze":
		return freezeCLI(os.Args)
	case "-e", "--edit":
		return editCLI(os.Args)
	case "-h", "--help":
		showHelp()
		return nil
	default:
		return fmt.Errorf("unknown flag %q (try -h)", os.Args[1])
	}
}

func showHelp() {
	fmt.Printf(`gotomux - session picker (%s)

Usage:
  gotomux              interactive picker
  gotomux -f [name]    freeze session
  gotomux -e [name]    edit preset
  gotomux -h          show help
`, version)
}

func runPicker() error {
	var (
		ctl    tmux.Connector
		st     *store.Store
		ctlErr error
		stErr  error
		root   string
	)
	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		c, e := tmux.New()
		ctl, ctlErr = c, e
	}()
	go func() {
		defer wg.Done()
		s, e := store.Open()
		st, stErr = s, e
	}()
	go func() {
		defer wg.Done()
		cwd, _ := os.Getwd()
		root = project.FindProjectRoot(cwd)
	}()
	wg.Wait()
	if ctlErr != nil {
		return fmt.Errorf("tmux: %w", ctlErr)
	}
	if stErr != nil {
		return fmt.Errorf("store: %w", stErr)
	}
	defer st.Close()
	name := project.SessionName(root)
	return picker.RunPicker(ctl, st, name, root, func(it picker.Item) error {
		return connectItem(ctl, st, it)
	})
}

func connectItem(ctl tmux.Connector, st *store.Store, it picker.Item) error {
	var err error
	switch it.Kind {
	case picker.KindCreate, picker.KindZoxide:
		err = template.ConnectProject(ctl, st, it.Name, it.Path)
	case picker.KindActive:
		err = ctl.Connect(it.Name, "")
	case picker.KindPreset:
		if st == nil {
			return fmt.Errorf("connect preset: nil store")
		}
		p, e := st.Get(it.Name)
		if e != nil {
			return e
		}
		_ = st.Touch(it.Name)
		err = ctl.ConnectPreset(p)
	default:
		return fmt.Errorf("unknown kind %v", it.Kind)
	}
	return err
}

func freezeCLI(args []string) error {
	name := ""
	if len(args) > 2 {
		name = args[2]
	}
	ctl, err := tmux.New()
	if err != nil {
		return fmt.Errorf("tmux: %w", err)
	}
	st, err := store.Open()
	if err != nil {
		return fmt.Errorf("store: %w", err)
	}
	defer st.Close()
	if name == "" {
		name = ctl.CurrentSession()
	}
	if name == "" {
		live, e := ctl.ListLive()
		if e != nil {
			return e
		}
		if len(live) == 0 {
			return fmt.Errorf("no active sessions")
		}
		items := make([]string, 0, len(live))
		for _, s := range live {
			items = append(items, s.Name)
		}
		name, err = picker.Pick(items)
		if err != nil || name == "" {
			return errCancel
		}
	}
	stop := picker.HoldInterrupt()
	_, _, err = template.FreezeRemember(ctl, st, name)
	stop()
	if err != nil {
		return err
	}
	fmt.Printf("froze %s\n", name)
	return nil
}

func editCLI(args []string) error {
	name := ""
	if len(args) > 2 {
		name = args[2]
	}
	st, err := store.Open()
	if err != nil {
		return fmt.Errorf("store: %w", err)
	}
	defer st.Close()
	ctl, ctlErr := tmux.New()
	if name == "" && ctlErr == nil && ctl != nil {
		name = ctl.CurrentSession()
	}
	if name == "" {
		return template.Edit(st, "", picker.Pick)
	}
	if _, err := st.Get(name); err != nil {
		if ctlErr != nil {
			return fmt.Errorf("preset %q not found and tmux unavailable: %v", name, ctlErr)
		}
		stop := picker.HoldInterrupt()
		_, _, err = template.FreezeRemember(ctl, st, name)
		stop()
		if err != nil {
			return err
		}
	}
	return template.Edit(st, name, picker.Pick)
}
