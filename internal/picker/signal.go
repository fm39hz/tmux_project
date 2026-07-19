package picker

import (
	"os"
	"os/signal"

	tea "github.com/charmbracelet/bubbletea"
)

// DrainSignals empties a signal channel so disposition stays clean after Stop.
func DrainSignals(ch <-chan os.Signal) {
	for {
		select {
		case <-ch:
		default:
			return
		}
	}
}

// HoldInterrupt discards SIGINT for a short critical section (freeze/save tx).
// Call stop() when the section ends - restores default disposition after drain.
func HoldInterrupt() (stop func()) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-ch:
				// discard - ACID section must finish
			case <-done:
				return
			}
		}
	}()
	return func() {
		signal.Stop(ch)
		close(done)
		DrainSignals(ch)
	}
}

// RunCancellable runs a Bubble Tea program while owning SIGINT as cancel.
// SIGINT -> p.Quit(); tea.ErrInterrupted is returned as (model, nil) with no extra error
// when the model already quit - callers treat cancel via their model state.
// The returned error is only a real program failure (not interrupt).
func RunCancellable(p *tea.Program) (tea.Model, error) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	sigDone := make(chan struct{})
	go func() {
		select {
		case <-sigCh:
			p.Quit()
		case <-sigDone:
		}
	}()
	final, err := p.Run()
	close(sigDone)
	signal.Stop(sigCh)
	DrainSignals(sigCh)
	if err != nil && err != tea.ErrInterrupted {
		return final, err
	}
	// interrupt/cancel -> model may be partial; caller reads Done()/name
	return final, nil
}
