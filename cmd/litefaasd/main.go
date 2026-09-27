package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/wolvever/litefaas/internal/api"
	"github.com/wolvever/litefaas/internal/config"
	"github.com/wolvever/litefaas/internal/runner"
	"github.com/wolvever/litefaas/internal/secret"
	"github.com/wolvever/litefaas/internal/store"
	"github.com/wolvever/litefaas/internal/token"
	"github.com/wolvever/litefaas/internal/types"
	"github.com/wolvever/litefaas/internal/version"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "listen address")
	dataDir := flag.String("data-dir", config.DefaultDataDir(), "sqlite data directory")
	tokFlag := flag.String("token", os.Getenv("LITEFAAS_TOKEN"), "shared bearer token (LITEFAAS_TOKEN)")
	noAuth := flag.Bool("no-auth", false, "disable bearer auth (open control plane)")
	idleTTL := flag.Duration("idle-ttl", types.DefaultIdleTTL, "stop idle functions (0 disables)")
	tlsCert := flag.String("tls-cert", "", "TLS certificate file (requires --tls-key)")
	tlsKey := flag.String("tls-key", "", "TLS private key file (requires --tls-cert)")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("litefaasd %s (%s)\n", version.Version, version.Commit)
		return
	}

	https, err := requireTLSPair(*tlsCert, *tlsKey)
	if err != nil {
		log.Printf("tls: %v", err)
		os.Exit(1)
	}

	st, err := store.Open(*dataDir)
	if err != nil {
		log.Printf("store: %v", err)
		os.Exit(1)
	}
	defer st.Close()

	tok, info, err := token.ResolveDaemon(*tokFlag, *dataDir, *noAuth)
	if err != nil {
		log.Printf("token: %v", err)
		os.Exit(1)
	}
	switch info.Mode {
	case "file":
		if info.Generated {
			log.Printf("generated bearer token at %s (lf reads this file, or set LITEFAAS_TOKEN)", info.Path)
		} else {
			log.Printf("auth=on token-file=%s", info.Path)
		}
	case "explicit":
		log.Printf("auth=on token=flag/env")
	default:
		log.Printf("auth=off")
	}

	sec, err := secret.Open(*dataDir)
	if err != nil {
		log.Printf("secrets: %v", err)
		os.Exit(1)
	}

	srv := api.New(api.Options{
		Store:   st,
		Secrets: sec,
		Token:   tok,
		Runner:  runner.NewDocker(),
		IdleTTL: *idleTTL,
	})
	defer srv.Close()
	idle := "off"
	if *idleTTL > 0 {
		idle = idleTTL.String()
	}
	scheme := "http"
	if https {
		scheme = "https"
	}
	log.Printf("litefaasd %s listening on %s://%s data-dir=%s idle-ttl=%s", version.Version, scheme, *addr, st.Dir(), idle)
	if https {
		err = http.ListenAndServeTLS(*addr, *tlsCert, *tlsKey, srv)
	} else {
		err = http.ListenAndServe(*addr, srv)
	}
	if err != nil {
		log.Printf("listen: %v", err)
		os.Exit(1)
	}
}

// requireTLSPair returns whether to serve HTTPS.
// Both cert and key must be set together; a single flag is a fatal misconfiguration.
func requireTLSPair(cert, key string) (https bool, err error) {
	cert = strings.TrimSpace(cert)
	key = strings.TrimSpace(key)
	if cert == "" && key == "" {
		return false, nil
	}
	if cert == "" || key == "" {
		return false, fmt.Errorf("both --tls-cert and --tls-key are required together")
	}
	return true, nil
}
