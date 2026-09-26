package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/wolvever/litefaas/internal/api"
	"github.com/wolvever/litefaas/internal/config"
	"github.com/wolvever/litefaas/internal/metrics"
	"github.com/wolvever/litefaas/internal/runner"
	"github.com/wolvever/litefaas/internal/store"
	"github.com/wolvever/litefaas/internal/token"
	"github.com/wolvever/litefaas/internal/version"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "listen address")
	dataDir := flag.String("data-dir", config.DefaultDataDir(), "sqlite data directory")
	tok := flag.String("token", os.Getenv("LITEFAAS_TOKEN"), "shared bearer token (LITEFAAS_TOKEN)")
	tokenFile := flag.String("token-file", "", "token file (default <data-dir>/token)")
	insecure := flag.Bool("insecure", false, "disable bearer auth (local demo)")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("litefaasd %s (%s)\n", version.Version, version.Commit)
		return
	}

	st, err := store.Open(*dataDir)
	if err != nil {
		log.Printf("store: %v", err)
		os.Exit(1)
	}
	defer st.Close()

	tf := *tokenFile
	if tf == "" {
		tf = token.Path(*dataDir)
	}
	auth, created, err := token.Resolve(*tok, tf, *insecure)
	if err != nil {
		log.Printf("token: %v", err)
		os.Exit(1)
	}
	if *insecure {
		log.Printf("auth disabled (--insecure)")
	} else if created {
		log.Printf("generated bearer token at %s (export LITEFAAS_TOKEN=$(cat %s))", tf, tf)
	} else if auth != "" {
		log.Printf("bearer auth enabled")
	}

	srv := api.New(api.Options{
		Store:   st,
		Token:   auth,
		Runner:  runner.NewDocker(),
		Metrics: metrics.New(),
	})
	log.Printf("litefaasd %s listening on http://%s data-dir=%s", version.Version, *addr, st.Dir())
	if err := http.ListenAndServe(*addr, srv); err != nil {
		log.Printf("listen: %v", err)
		os.Exit(1)
	}
}
