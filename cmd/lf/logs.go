package main

import (
	"bufio"
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/wolvever/litefaas/internal/client"
	"github.com/wolvever/litefaas/internal/config"
	"github.com/wolvever/litefaas/internal/types"
)

func cmdLogs(args []string) error {
	fs := flag.NewFlagSet("logs", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	follow := fs.Bool("f", false, "follow log output")
	followLong := fs.Bool("follow", false, "follow log output")
	tail := fs.Int("tail", 100, "lines from the end of the logs")
	daemon := fs.Bool("daemon", false, "tail local litefaasd.log instead of container logs")
	dataDir := fs.String("data-dir", "", "daemon data dir for --daemon (default ~/.litefaas)")
	gw := fs.String("gateway", "", "litefaasd URL")
	tok := fs.String("token", "", "bearer token")
	dir := fs.String("config-dir", "", "CLI config directory")
	rest, err := parseMixed(fs, args)
	if err != nil {
		return err
	}
	doFollow := *follow || *followLong
	if *daemon {
		dd := *dataDir
		if dd == "" {
			dd = config.DefaultDataDir()
		}
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		return tailDaemonLog(ctx, dd, *tail, doFollow, os.Stdout)
	}
	c, err := resolveClient(*gw, *tok, *dir)
	if err != nil {
		return err
	}
	resources, err := resolveLogsResources(c, rest)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	return streamLogs(ctx, c, resources, doFollow, *tail, os.Stdout, os.Stderr)
}

func resolveLogsResources(c *client.Client, rest []string) ([]types.Resource, error) {
	arg := ""
	if len(rest) > 0 {
		arg = rest[0]
	}
	// Multi-service projects aggregate by default (unlike lf url, which needs --all).
	resources, err := resolveURLResources(c, arg, true)
	if err != nil {
		return nil, err
	}
	if len(resources) == 0 {
		return nil, fmt.Errorf("usage: lf logs [name|path] [-f] [--tail N]  or  lf logs --daemon [-f]")
	}
	return resources, nil
}

func streamLogs(ctx context.Context, c *client.Client, resources []types.Resource, follow bool, tail int, w, errW io.Writer) error {
	if len(resources) == 1 {
		err := c.Logs(ctx, resources[0].Name, follow, tail, w)
		if err != nil && isNotDeployedErr(err) {
			return fmt.Errorf("resource %q is not deployed (no running container)", resources[0].Name)
		}
		return err
	}
	return streamLogsAggregate(ctx, c, resources, follow, tail, w, errW)
}

func streamLogsAggregate(ctx context.Context, c *client.Client, resources []types.Resource, follow bool, tail int, w, errW io.Writer) error {
	var mu sync.Mutex
	var wg sync.WaitGroup
	errCh := make(chan error, len(resources))
	var (
		okMu    sync.Mutex
		okCount int
	)
	for _, res := range resources {
		name := res.Name
		wg.Add(1)
		go func() {
			defer wg.Done()
			pw := &prefixWriter{mu: &mu, w: w, prefix: "[" + name + "] "}
			err := c.Logs(ctx, name, follow, tail, pw)
			_ = pw.Flush()
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				if isNotDeployedErr(err) {
					mu.Lock()
					fmt.Fprintf(errW, "warning: resource %q is not deployed (skipping)\n", name)
					mu.Unlock()
					return
				}
				errCh <- fmt.Errorf("%s: %w", name, err)
				return
			}
			okMu.Lock()
			okCount++
			okMu.Unlock()
		}()
	}
	wg.Wait()
	close(errCh)
	var first error
	for e := range errCh {
		if first == nil {
			first = e
		}
	}
	okMu.Lock()
	nOK := okCount
	okMu.Unlock()
	if nOK == 0 && first != nil {
		return first
	}
	if nOK == 0 {
		return fmt.Errorf("no deployed services to show logs for")
	}
	// Partial failures: warn already printed for not-deployed; return first hard error if any stream died oddly while others OK — prefer success.
	_ = first
	return nil
}

// prefixWriter prefixes each complete line under mu.
type prefixWriter struct {
	mu     *sync.Mutex
	w      io.Writer
	prefix string
	buf    []byte
}

func (p *prefixWriter) Write(b []byte) (int, error) {
	p.buf = append(p.buf, b...)
	for {
		i := bytes.IndexByte(p.buf, '\n')
		if i < 0 {
			break
		}
		line := append([]byte(nil), p.buf[:i+1]...)
		p.buf = p.buf[i+1:]
		if err := p.writePrefixed(line); err != nil {
			return len(b), err
		}
	}
	return len(b), nil
}

func (p *prefixWriter) Flush() error {
	if len(p.buf) == 0 {
		return nil
	}
	line := append([]byte(nil), p.buf...)
	if !bytes.HasSuffix(line, []byte("\n")) {
		line = append(line, '\n')
	}
	p.buf = nil
	return p.writePrefixed(line)
}

func (p *prefixWriter) writePrefixed(line []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, err := io.WriteString(p.w, p.prefix); err != nil {
		return err
	}
	_, err := p.w.Write(line)
	return err
}

func isNotDeployedErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "not deployed") || strings.Contains(msg, "\"error\":\"not deployed\"")
}

func daemonLogPath(dataDir string) string {
	return filepath.Join(dataDir, "litefaasd.log")
}

// tailDaemonLog prints the last tail lines of litefaasd.log and optionally follows.
func tailDaemonLog(ctx context.Context, dataDir string, tail int, follow bool, w io.Writer) error {
	path := daemonLogPath(dataDir)
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("daemon log not found at %s (run lf up?)", path)
		}
		return err
	}
	if tail <= 0 {
		tail = 100
	}
	offset, err := writeLastLines(path, tail, w)
	if err != nil {
		return err
	}
	if !follow {
		return nil
	}
	return followFile(ctx, path, offset, w)
}

func writeLastLines(path string, n int, w io.Writer) (int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return 0, err
	}
	size := st.Size()
	lines, err := readLastNLines(f, size, n)
	if err != nil {
		return size, err
	}
	for _, line := range lines {
		if _, err := io.WriteString(w, line); err != nil {
			return size, err
		}
		if !strings.HasSuffix(line, "\n") {
			if _, err := io.WriteString(w, "\n"); err != nil {
				return size, err
			}
		}
	}
	return size, nil
}

func readLastNLines(f *os.File, size int64, n int) ([]string, error) {
	if n <= 0 || size == 0 {
		return nil, nil
	}
	// Read whole file when small; otherwise scan from a bounded window.
	const maxWindow = 1 << 20 // 1 MiB
	start := int64(0)
	if size > maxWindow {
		start = size - maxWindow
	}
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return nil, err
	}
	sc := bufio.NewScanner(f)
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 1<<20)
	var all []string
	for sc.Scan() {
		all = append(all, sc.Text())
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(all) > n {
		all = all[len(all)-n:]
	}
	return all, nil
}

func followFile(ctx context.Context, path string, offset int64, w io.Writer) error {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			f, err := os.Open(path)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return err
			}
			st, err := f.Stat()
			if err != nil {
				_ = f.Close()
				return err
			}
			if st.Size() < offset {
				// truncated
				offset = 0
			}
			if st.Size() == offset {
				_ = f.Close()
				continue
			}
			if _, err := f.Seek(offset, io.SeekStart); err != nil {
				_ = f.Close()
				return err
			}
			n, err := io.Copy(w, f)
			offset += n
			_ = f.Close()
			if err != nil {
				return err
			}
		}
	}
}
