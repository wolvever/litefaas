package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/wolvever/litefaas/internal/api"
	"github.com/wolvever/litefaas/internal/config"
	"github.com/wolvever/litefaas/internal/store"
	"github.com/wolvever/litefaas/internal/version"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "litefaasd: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	listen := flag.String("listen", "127.0.0.1:8080", "HTTP listen address")
	dataDir := flag.String("data-dir", "", "directory for sqlite state (default: ~/.litefaas)")
	token := flag.String("token", os.Getenv("LITEFAAS_TOKEN"), "shared bearer token (LITEFAAS_TOKEN)")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("%s %s\n", version.Name, version.Version)
		return nil
	}

	dir := *dataDir
	if dir == "" {
		home, err := config.Dir()
		if err != nil {
			return fmt.Errorf("resolve data-dir: %w", err)
		}
		dir = home
	}
	dir, err := filepath.Abs(dir)
	if err != nil {
		return err
	}

	st, err := store.Open(dir)
	if err != nil {
		return err
	}
	defer st.Close()

	if *token == "" {
		slog.Warn("no bearer token configured; API routes are unauthenticated (set LITEFAAS_TOKEN or --token)")
	}

	srv := &http.Server{
		Addr:              *listen,
		Handler:           api.New(st, *token).Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("listening", "addr", srv.Addr, "data_dir", dir, "version", version.Version)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case <-ctx.Done():
	case err := <-errCh:
		if err != nil {
			return err
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}
