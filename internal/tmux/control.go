package tmux

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// ControlConn wraps tmux -C for persistent listing.
type ControlConn struct {
	mu     sync.Mutex
	w      io.WriteCloser
	r      io.ReadCloser
	br     *bufio.Reader
	cmd    *exec.Cmd
	broken bool
}

func StartControl() (*ControlConn, error) {
	cmd := exec.Command("tmux", "-C")
	w, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	r, err := cmd.StdoutPipe()
	if err != nil {
		w.Close()
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		w.Close()
		r.Close()
		return nil, fmt.Errorf("tmux -C: %w", err)
	}
	return &ControlConn{w: w, r: r, br: bufio.NewReader(r), cmd: cmd}, nil
}

// Send writes a command and reads the response.
// Runs in a goroutine with a 5-second timeout so a hung tmux -C never
// blocks the daemon permanently. On timeout, the reader pipe is closed
// to unblock the goroutine; the connection is marked broken and caller
// must Reconnect.
func (cc *ControlConn) Send(ctx context.Context, args ...string) (string, error) {
	type resT struct {
		out string
		err error
	}
	ch := make(chan resT, 1)

	go func() {
		cc.mu.Lock()
		defer cc.mu.Unlock()

		if cc.broken {
			ch <- resT{"", fmt.Errorf("tmux -C: broken pipe")}
			return
		}

		var cmdLine strings.Builder
		for i, a := range args {
			if i > 0 {
				cmdLine.WriteByte(' ')
			}
			if strings.ContainsAny(a, " '\";") {
				cmdLine.WriteByte('\'')
				cmdLine.WriteString(strings.ReplaceAll(a, "'", "'\\''"))
				cmdLine.WriteByte('\'')
			} else {
				cmdLine.WriteString(a)
			}
		}
		cmdLine.WriteByte('\n')

		if _, err := io.WriteString(cc.w, cmdLine.String()); err != nil {
			cc.broken = true
			ch <- resT{"", fmt.Errorf("tmux -C send: %w", err)}
			return
		}

		var out strings.Builder
		for {
			line, err := cc.br.ReadString('\n')
			if err != nil {
				cc.broken = true
				ch <- resT{"", fmt.Errorf("tmux -C recv: %w", err)}
				return
			}
			line = strings.TrimSuffix(line, "\n")

			switch {
			case strings.HasPrefix(line, "%begin "):
			case strings.HasPrefix(line, "%end "):
				ch <- resT{strings.TrimRight(out.String(), "\n"), nil}
				return
			case strings.HasPrefix(line, "%error "):
				ch <- resT{"", fmt.Errorf("tmux -C: %s", strings.TrimSpace(line[7:]))}
				return
			default:
				if strings.HasPrefix(line, "%") {
					continue
				}
				out.WriteString(line)
				out.WriteByte('\n')
			}
		}
	}()

	timeout := 5 * time.Second
	if dl, ok := ctx.Deadline(); ok {
		if d := time.Until(dl); d < timeout {
			timeout = d
		}
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case r := <-ch:
		return r.out, r.err
	case <-timer.C:
		cc.mu.Lock()
		cc.broken = true
		cc.r.Close()
		cc.mu.Unlock()
		<-ch
		return "", fmt.Errorf("tmux -C: timeout after %v", timeout)
	}
}

func (cc *ControlConn) Close() {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	cc.broken = true
	cc.r.Close()
	cc.w.Close()
	if cc.cmd != nil && cc.cmd.Process != nil {
		cc.cmd.Wait()
	}
}

// Reconnect starts a fresh tmux server and control connection.
func (cc *ControlConn) Reconnect() error {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	cc.r.Close()
	cc.w.Close()
	if cc.cmd != nil && cc.cmd.Process != nil {
		cc.cmd.Wait()
	}

	exec.Command("tmux", "start-server").Run()
	exec.Command("tmux", "set-option", "-g", "exit-empty", "off").Run()

	cmd := exec.Command("tmux", "-C")
	w, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	r, err := cmd.StdoutPipe()
	if err != nil {
		w.Close()
		return err
	}
	if err := cmd.Start(); err != nil {
		w.Close()
		r.Close()
		return fmt.Errorf("reconnect tmux -C: %w", err)
	}
	cc.w = w
	cc.r = r
	cc.br = bufio.NewReader(r)
	cc.cmd = cmd
	cc.broken = false
	return nil
}
