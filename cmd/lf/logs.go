package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/wolvever/litefaas/internal/client"
	"github.com/wolvever/litefaas/internal/config"
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
	name, err := resolveLogsName(c, rest)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := c.Logs(ctx, name, doFollow, *tail, os.Stdout); err != nil {
		if isNotDeployedErr(err) {
			return fmt.Errorf("resource %q is not deployed (no running container)", name)
		}
		return err
	}
	return nil
}

func resolveLogsName(c *client.Client, rest []string) (string, error) {
	arg := ""
	if len(rest) > 0 {
		arg = rest[0]
	}
	resources, err := resolveURLResources(c, arg, false)
	if err != nil {
		return "", err
	}
	if len(resources) == 0 {
		return "", fmt.Errorf("usage: lf logs [name|path] [-f] [--tail N]  or  lf logs --daemon [-f]")
	}
	return resources[0].Name, nil
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
