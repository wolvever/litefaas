package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/wolvever/litefaas/internal/builder"
	"github.com/wolvever/litefaas/internal/client"
	"github.com/wolvever/litefaas/internal/manifest"
)

// stringList is a repeatable flag.Value for --ignore.
type stringList []string

func (s *stringList) String() string {
	if s == nil {
		return ""
	}
	return strings.Join(*s, ",")
}

func (s *stringList) Set(v string) error {
	*s = append(*s, v)
	return nil
}

type watchOptions struct {
	Path       string
	StackID    string
	Debounce   time.Duration
	Ignore     []string
	Gateway    string
	Token      string
	ConfigDir  string
	NoUp       bool
	NoInitial  bool
	Once       bool
	// Up subset (used unless --no-up)
	Addr      string
	DataDir   string
	NoAuth    bool
	Context   string
	Litefaasd string
	Timeout   time.Duration
}

func cmdWatch(args []string) error {
	fs := flag.NewFlagSet("watch", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	opts := watchOptions{}
	fs.StringVar(&opts.StackID, "stack", "", "stack pack id (overrides detection)")
	fs.DurationVar(&opts.Debounce, "debounce", 500*time.Millisecond, "quiet period before rebuild")
	var ignore stringList
	fs.Var(&ignore, "ignore", "extra ignore path segment or glob (repeatable)")
	fs.StringVar(&opts.Gateway, "gateway", "", "litefaasd URL (overrides context)")
	fs.StringVar(&opts.Token, "token", "", "bearer token")
	fs.StringVar(&opts.ConfigDir, "config-dir", "", "CLI config directory")
	fs.BoolVar(&opts.NoUp, "no-up", false, "do not call runUp; require existing healthy gateway")
	fs.BoolVar(&opts.NoInitial, "no-initial", false, "skip the first build+deploy; only rebuild on change")
	fs.BoolVar(&opts.Once, "once", false, "run one build+deploy then exit (no watch loop)")
	// up subset
	fs.StringVar(&opts.Addr, "addr", defaultUpAddr, "listen address (passed to lf up)")
	fs.StringVar(&opts.DataDir, "data-dir", "", "daemon data dir (passed to lf up)")
	fs.BoolVar(&opts.NoAuth, "no-auth", false, "pass --no-auth to litefaasd when ensuring daemon")
	fs.StringVar(&opts.Context, "context", "default", "context name (passed to lf up)")
	fs.StringVar(&opts.Litefaasd, "litefaasd", "", "path to litefaasd binary")
	fs.DurationVar(&opts.Timeout, "timeout", defaultUpTimeout, "wait for GET /healthz when ensuring daemon")

	rest, err := parseMixed(fs, args)
	if err != nil {
		return err
	}
	opts.Ignore = []string(ignore)
	opts.Path = "."
	if len(rest) > 0 {
		opts.Path = rest[0]
	}
	if opts.Debounce <= 0 {
		opts.Debounce = 500 * time.Millisecond
	}
	return runWatch(opts, os.Stdout, os.Stderr)
}

func runWatch(opts watchOptions, stdout, stderr io.Writer) error {
	root, err := filepath.Abs(opts.Path)
	if err != nil {
		return err
	}
	st, err := os.Stat(root)
	if err != nil {
		return err
	}
	if !st.IsDir() {
		return fmt.Errorf("watch path is not a directory: %s", root)
	}

	if !opts.NoUp {
		upOpts := upOptions{
			Addr:      opts.Addr,
			DataDir:   opts.DataDir,
			ConfigDir: opts.ConfigDir,
			Gateway:   opts.Gateway,
			NoAuth:    opts.NoAuth,
			Token:     opts.Token,
			Context:   opts.Context,
			Litefaasd: opts.Litefaasd,
			Detach:    true, // watch owns the foreground TTY
			Timeout:   opts.Timeout,
		}
		if err := runUp(upOpts, stdout, stderr); err != nil {
			return err
		}
	}

	c, err := resolveClient(opts.Gateway, opts.Token, opts.ConfigDir)
	if err != nil {
		return err
	}
	if opts.NoUp {
		if _, err := c.Healthz(); err != nil {
			return fmt.Errorf("gateway unreachable (--no-up): %w", err)
		}
	}

	doCycle := func(ctx context.Context) error {
		return runBuildDeploy(ctx, root, opts.StackID, c, stdout, stderr)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if opts.Once {
		if err := doCycle(ctx); err != nil {
			return err
		}
		return nil
	}

	if !opts.NoInitial {
		if err := doCycle(ctx); err != nil {
			fmt.Fprintf(stderr, "initial build/deploy failed: %v\n", err)
			// keep watching (design: rebuild errors keep watching; initial also continues)
		}
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	dirs, err := collectWatchDirs(root, opts.Ignore)
	if err != nil {
		return err
	}
	for _, d := range dirs {
		if err := watcher.Add(d); err != nil {
			fmt.Fprintf(stderr, "watch add %s: %v\n", d, err)
		}
	}

	fmt.Fprintf(stderr, "watching %s (debounce=%s)\n", root, opts.Debounce)

	var (
		mu       sync.Mutex
		dirty    bool
		pending  bool
		running  bool
		timer    *time.Timer
		rebuildN int
	)

	startRebuild := func() {
		mu.Lock()
		if running {
			pending = true
			mu.Unlock()
			return
		}
		running = true
		dirty = false
		mu.Unlock()

		go func() {
			for {
				rebuildN++
				fmt.Fprintf(stderr, "rebuild #%d…\n", rebuildN)
				err := doCycle(ctx)
				if err != nil {
					fmt.Fprintf(stderr, "rebuild failed: %v\n", err)
				}
				mu.Lock()
				if pending && ctx.Err() == nil {
					pending = false
					dirty = false
					mu.Unlock()
					continue // coalesce burst into one more pass
				}
				running = false
				mu.Unlock()
				fmt.Fprintf(stderr, "watching %s (debounce=%s)\n", root, opts.Debounce)
				return
			}
		}()
	}

	schedule := func() {
		mu.Lock()
		defer mu.Unlock()
		if running {
			// During rebuild: coalesce to one post-rebuild pass; do not reset timer storm.
			pending = true
			return
		}
		dirty = true
		if timer != nil {
			timer.Stop()
		}
		timer = time.AfterFunc(opts.Debounce, func() {
			mu.Lock()
			if !dirty {
				mu.Unlock()
				return
			}
			dirty = false
			mu.Unlock()
			startRebuild()
		})
	}

	for {
		select {
		case <-ctx.Done():
			mu.Lock()
			if timer != nil {
				timer.Stop()
			}
			mu.Unlock()
			return nil
		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			fmt.Fprintf(stderr, "watch error: %v\n", err)
		case ev, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			rel, relErr := filepath.Rel(root, ev.Name)
			if relErr != nil {
				continue
			}
			// New directories: add watch if not ignored.
			if ev.Has(fsnotify.Create) {
				if info, statErr := os.Stat(ev.Name); statErr == nil && info.IsDir() {
					if !isIgnoredDirName(info.Name(), opts.Ignore) && ShouldRebuild(rel, opts.Ignore) {
						_ = watcher.Add(ev.Name)
					}
				}
			}
			if !ev.Has(fsnotify.Create) && !ev.Has(fsnotify.Write) &&
				!ev.Has(fsnotify.Remove) && !ev.Has(fsnotify.Rename) &&
				!ev.Has(fsnotify.Chmod) {
				continue
			}
			if !ShouldRebuild(rel, opts.Ignore) {
				continue
			}
			schedule()
		}
	}
}

// runBuildDeploy builds then deploys in-process (shared by lf watch).
func runBuildDeploy(ctx context.Context, dir, stackID string, c *client.Client, stdout, stderr io.Writer) error {
	fmt.Fprintf(stderr, "building %s\n", dir)
	results, err := builder.BuildStackWith(ctx, dir, stackID, stdout, stderr)
	if err != nil {
		return err
	}
	for _, res := range results {
		if res.Detected && res.Stack != "" {
			fmt.Fprintf(stderr, "detected stack=%s\n", res.Stack)
			if res.Hints != "" {
				fmt.Fprintf(stderr, "stack hints (not started by litefaas): %s\n", res.Hints)
			}
		} else if res.Stack != "" && stackID != "" {
			fmt.Fprintf(stderr, "stack=%s\n", res.Stack)
		}
		fmt.Fprintf(stdout, "built %s\n", res.Image)
	}

	resolved, err := manifest.ResolveDetect(dir, stackID)
	if err != nil {
		return err
	}
	if resolved.Multi != nil {
		n := 0
		for i := range resolved.Multi.Services {
			if err := deployResource(c, resolved.Multi.Services[i].Resource()); err != nil {
				return err
			}
			n++
		}
		fmt.Fprintf(stdout, "deployed %d services → gateway %s\n", n, c.Gateway)
		return nil
	}
	if resolved.Detected && resolved.Pack != nil {
		fmt.Fprintf(stdout, "detected stack=%s\n", resolved.Pack.ID)
		if h := resolved.Pack.FormatHints(); h != "" {
			fmt.Fprintf(stdout, "stack hints (not started by litefaas): %s\n", h)
		}
	}
	return deployResource(c, resolved.Manifest.Resource())
}

// Debouncer collapses events that arrive within Window into a single fire.
// Used by unit tests; the watch loop uses time.AfterFunc equivalently.
type Debouncer struct {
	Window time.Duration
	C      chan struct{} // receives one signal per coalesced fire

	mu    sync.Mutex
	timer *time.Timer
}

func NewDebouncer(window time.Duration) *Debouncer {
	return &Debouncer{
		Window: window,
		C:      make(chan struct{}, 1),
	}
}

func (d *Debouncer) Hit() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.timer != nil {
		d.timer.Stop()
	}
	d.timer = time.AfterFunc(d.Window, func() {
		select {
		case d.C <- struct{}{}:
		default:
		}
	})
}

func (d *Debouncer) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.timer != nil {
		d.timer.Stop()
		d.timer = nil
	}
}
