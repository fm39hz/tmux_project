package picker

import (
	"os"
	"os/signal"
)

// HoldInterrupt discards SIGINT for a short critical section (freeze/save tx).
// Call stop() when the section ends — restores default disposition after drain.
func HoldInterrupt() (stop func()) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-ch:
				// discard — ACID section must finish
			case <-done:
				return
			}
		}
	}()
	return func() {
		signal.Stop(ch)
		close(done)
		for {
			select {
			case <-ch:
			default:
				return
			}
		}
	}
}
