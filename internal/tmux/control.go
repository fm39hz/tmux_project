package tmux

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

type ControlConn struct {
	mu   sync.Mutex
	cmd  *exec.Cmd
	in   io.WriteCloser
	out  *bufio.Reader
	readTimeout time.Duration
}

func StartControl() (*ControlConn, error) {
	cmd := exec.Command("tmux", "-C")
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("tmux -C stdin: %w", err)
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("tmux -C stdout: %w", err)
	}
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("tmux -C start: %w", err)
	}
	reader := bufio.NewReader(out)
	if err := drainHandshake(reader, 3*time.Second); err != nil {
		cmd.Process.Kill()
		return nil, fmt.Errorf("tmux -C handshake: %w", err)
	}
	return &ControlConn{
		cmd: cmd, in: in, out: reader,
		readTimeout: 5 * time.Second,
	}, nil
}

func drainHandshake(r *bufio.Reader, timeout time.Duration) error {
	done := make(chan error, 1)
	go func() {
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				done <- fmt.Errorf("read: %w", err)
				return
			}
			if strings.HasPrefix(line, "%end ") {
				done <- nil
				return
			}
		}
	}()
	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		return fmt.Errorf("timeout")
	}
}

func buildCommand(args []string) string {
	var b strings.Builder
	for i, a := range args {
		if i > 0 { b.WriteByte(' ') }
		if strings.ContainsAny(a, " '\";") {
			b.WriteByte('\'')
			b.WriteString(strings.ReplaceAll(a, "'", "'\\''"))
			b.WriteByte('\'')
		} else {
			b.WriteString(a)
		}
	}
	b.WriteByte('\n')
	return b.String()
}

func (cc *ControlConn) Send(ctx context.Context, args ...string) (string, error) {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	if cc.cmd == nil {
		return "", fmt.Errorf("tmux: not connected")
	}
	cmdLine := buildCommand(args)
	if _, err := fmt.Fprint(cc.in, cmdLine); err != nil {
		cc.closeLocked()
		return "", fmt.Errorf("tmux send: %w", err)
	}
	out, err := cc.readResponseLocked(ctx)
	if err != nil {
		cc.closeLocked()
		return "", err
	}
	return out, nil
}

func (cc *ControlConn) readResponseLocked(ctx context.Context) (string, error) {
	type res struct {
		out string
		err error
	}
	ch := make(chan res, 1)
	reader := cc.out
	go func() {
		if reader == nil {
			ch <- res{"", fmt.Errorf("tmux: read on closed connection")}
			return
		}
		var out strings.Builder
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				ch <- res{"", fmt.Errorf("tmux recv: %w", err)}
				return
			}
			line = strings.TrimSuffix(line, "\n")
			switch {
			case strings.HasPrefix(line, "%begin "):
			case strings.HasPrefix(line, "%end "):
				ch <- res{strings.TrimRight(out.String(), "\n"), nil}
				return
			case strings.HasPrefix(line, "%error "):
				ch <- res{"", fmt.Errorf("tmux: %s", strings.TrimSpace(line[7:]))}
				return
			default:
				if strings.HasPrefix(line, "%") { continue }
				out.WriteString(line)
				out.WriteByte('\n')
			}
		}
	}()
	t := cc.readTimeout
	if d, ok := ctx.Deadline(); ok {
		if rem := time.Until(d); rem < t { t = rem }
	}
	if t <= 0 { t = time.Second }
	select {
	case r := <-ch:
		return r.out, r.err
	case <-time.After(t):
		return "", fmt.Errorf("tmux: response timeout (%v)", t)
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func (cc *ControlConn) IsAlive() bool {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	if cc.cmd == nil || cc.cmd.Process == nil {
		return false
	}
	return cc.cmd.Process.Signal(syscall.Signal(0)) == nil
}

func (cc *ControlConn) Close() {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	cc.closeLocked()
}

func (cc *ControlConn) closeLocked() {
	if cc.cmd != nil && cc.cmd.Process != nil {
		cc.cmd.Process.Kill()
		cc.cmd.Wait()
	}
	if cc.in != nil { cc.in.Close() }
	cc.cmd = nil
	cc.in = nil
	cc.out = nil
}

func (cc *ControlConn) Reconnect() error {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	cc.closeLocked()
	cmd := exec.Command("tmux", "-C")
	in, err := cmd.StdinPipe()
	if err != nil { return fmt.Errorf("tmux -C stdin: %w", err) }
	out, err := cmd.StdoutPipe()
	if err != nil { return fmt.Errorf("tmux -C stdout: %w", err) }
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil { return fmt.Errorf("tmux -C start: %w", err) }
	reader := bufio.NewReader(out)
	if err := drainHandshake(reader, 3*time.Second); err != nil {
		cmd.Process.Kill()
		return fmt.Errorf("tmux -C handshake: %w", err)
	}
	cc.cmd = cmd
	cc.in = in
	cc.out = reader
	return nil
}
