package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/wolvever/litefaas/internal/api"
	"github.com/wolvever/litefaas/internal/config"
	"github.com/wolvever/litefaas/internal/store"
	"github.com/wolvever/litefaas/internal/version"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "listen address")
	dataDir := flag.String("data-dir", config.DefaultDataDir(), "sqlite data directory")
	token := flag.String("token", os.Getenv("LITEFAAS_TOKEN"), "shared bearer token (LITEFAAS_TOKEN)")
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

	srv := api.New(api.Options{Store: st, Token: *token})
	log.Printf("litefaasd %s listening on http://%s data-dir=%s", version.Version, *addr, st.Dir())
	if err := http.ListenAndServe(*addr, srv); err != nil {
		log.Printf("listen: %v", err)
		os.Exit(1)
	}
}
