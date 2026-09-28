package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/wolvever/litefaas/internal/config"
	"github.com/wolvever/litefaas/internal/token"
)

const (
	defaultUpAddr    = "127.0.0.1:8080"
	defaultUpTimeout = 15 * time.Second
	pidFileName      = "litefaasd.pid"
	logFileName      = "litefaasd.log"
)

type upOptions struct {
	Addr       string
	DataDir    string
	ConfigDir  string
	Gateway    string
	NoAuth     bool
	Token      string
	Context    string
	Litefaasd  string
	Detach     bool
	Foreground bool
	Timeout    time.Duration
	TLSCert    string
	TLSKey     string
}

type probeResult int

const (
	probeNeedStart probeResult = iota
	probeReuse
	probeForeign
)

type healthProbe struct {
	Status  string
	Version string
	RawOK   bool
}

// probeGateway GET /healthz. Connection errors → need start.
func probeGateway(client *http.Client, gateway string) (probeResult, healthProbe, error) {
	if client == nil {
		client = &http.Client{Timeout: 2 * time.Second}
	}
	url := strings.TrimRight(gateway, "/") + "/healthz"
	resp, err := client.Get(url)
	if err != nil {
		if isConnError(err) {
			return probeNeedStart, healthProbe{}, nil
		}
		return probeNeedStart, healthProbe{}, nil
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return probeForeign, healthProbe{}, fmt.Errorf("address %s is in use but /healthz returned %s", gateway, resp.Status)
	}
	var m map[string]string
	_ = json.Unmarshal(raw, &m)
	hp := healthProbe{RawOK: true, Status: m["status"], Version: m["version"]}
	// Soft litefaasd check: prefer status=ok + version field presence.
	if hp.Status != "" && hp.Status != "ok" {
		return probeForeign, hp, fmt.Errorf("address %s responds but status=%q (not litefaasd?)", gateway, hp.Status)
	}
	if hp.Version == "" && hp.Status == "" {
		// Unknown healthy body — still treat as foreign to avoid attaching to random services.
		if len(raw) > 0 && !strings.Contains(string(raw), "version") {
			return probeForeign, hp, fmt.Errorf("address %s is healthy but does not look like litefaasd", gateway)
		}
	}
	return probeReuse, hp, nil
}

func isConnError(err error) bool {
	if err == nil {
		return false
	}
	var ne net.Error
	if strings.Contains(err.Error(), "connection refused") {
		return true
	}
	if strings.Contains(err.Error(), "connect: connection refused") {
		return true
	}
	_ = ne
	return true // treat other dial/timeouts as need-start for bootstrap UX
}

// resolveLitefaasd implements flag → env → sibling of lf → PATH.
func resolveLitefaasd(flagPath, envPath, lfExecutable string, lookPath func(string) (string, error)) (string, error) {
	if flagPath = strings.TrimSpace(flagPath); flagPath != "" {
		return flagPath, nil
	}
	if envPath = strings.TrimSpace(envPath); envPath != "" {
		return envPath, nil
	}
	if lfExecutable != "" {
		sib := filepath.Join(filepath.Dir(lfExecutable), "litefaasd")
		if st, err := os.Stat(sib); err == nil && !st.IsDir() {
			return sib, nil
		}
	}
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	p, err := lookPath("litefaasd")
	if err != nil {
		return "", fmt.Errorf("litefaasd not found (tried --litefaasd, LITEFAAS_LITEFAASD, beside lf, and PATH).\nBuild from source: go build -o litefaasd ./cmd/litefaasd\nOr: go install github.com/wolvever/litefaas/cmd/litefaasd@latest")
	}
	return p, nil
}

func gatewayURL(addr, gatewayFlag, tlsCert string) string {
	if g := strings.TrimSpace(gatewayFlag); g != "" {
		return strings.TrimRight(g, "/")
	}
	scheme := "http"
	if strings.TrimSpace(tlsCert) != "" {
		scheme = "https"
	}
	host := addr
	if strings.HasPrefix(host, ":") {
		host = "127.0.0.1" + host
	}
	return fmt.Sprintf("%s://%s", scheme, host)
}

func cmdUp(args []string) error {
	fs := flag.NewFlagSet("up", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	opts := upOptions{}
	fs.StringVar(&opts.Addr, "addr", defaultUpAddr, "listen address")
	fs.StringVar(&opts.DataDir, "data-dir", "", "daemon data dir (default ~/.litefaas)")
	fs.StringVar(&opts.ConfigDir, "config-dir", "", "CLI config dir (default ~/.litefaas)")
	fs.StringVar(&opts.Gateway, "gateway", "", "gateway URL written to context")
	fs.BoolVar(&opts.NoAuth, "no-auth", false, "pass --no-auth to litefaasd (loopback demos only)")
	fs.StringVar(&opts.Token, "token", "", "explicit token for daemon")
	fs.StringVar(&opts.Context, "context", "default", "context name to create/update")
	fs.StringVar(&opts.Litefaasd, "litefaasd", "", "path to litefaasd binary")
	detach := fs.Bool("detach", true, "start daemon in background (default)")
	fs.BoolVar(&opts.Foreground, "foreground", false, "run litefaasd in foreground (Ctrl-C stops)")
	fs.DurationVar(&opts.Timeout, "timeout", defaultUpTimeout, "wait for GET /healthz")
	fs.StringVar(&opts.TLSCert, "tls-cert", "", "TLS certificate (requires --tls-key)")
	fs.StringVar(&opts.TLSKey, "tls-key", "", "TLS private key (requires --tls-cert)")
	_, err := parseMixed(fs, args)
	if err != nil {
		return err
	}
	opts.Detach = *detach
	if opts.Foreground {
		opts.Detach = false
	}
	if (opts.TLSCert == "") != (opts.TLSKey == "") {
		return fmt.Errorf("both --tls-cert and --tls-key are required together")
	}
	return runUp(opts, os.Stdout, os.Stderr)
}

func runUp(opts upOptions, stdout, stderr io.Writer) error {
	if opts.DataDir == "" {
		opts.DataDir = config.DefaultDataDir()
	}
	if opts.ConfigDir == "" {
		opts.ConfigDir = config.DefaultDir()
	}
	if opts.Timeout <= 0 {
		opts.Timeout = defaultUpTimeout
	}
	gw := gatewayURL(opts.Addr, opts.Gateway, opts.TLSCert)

	client := &http.Client{Timeout: 2 * time.Second}
	pr, _, err := probeGateway(client, gw)
	if err != nil {
		return err
	}

	reused := pr == probeReuse
	var pid int
	var bin string

	// Prepare token/context before spawn so daemon and CLI share the same file.
	if err := ensureAuthAndContext(opts, gw); err != nil {
		return err
	}

	if pr == probeNeedStart {
		exe, _ := os.Executable()
		bin, err = resolveLitefaasd(opts.Litefaasd, os.Getenv("LITEFAAS_LITEFAASD"), exe, exec.LookPath)
		if err != nil {
			return err
		}
		_ = clearStalePidfile(opts.DataDir, client, gw)

		if opts.Detach {
			pid, err = startDetached(bin, opts)
			if err != nil {
				return err
			}
			if err := waitHealthy(client, gw, opts.Timeout); err != nil {
				return fmt.Errorf("litefaasd started (pid %d) but /healthz not ready: %w\nSee log: %s", pid, err, filepath.Join(opts.DataDir, logFileName))
			}
		} else {
			printUpSummary(stdout, gw, opts, false, 0, bin)
			if opts.NoAuth && !isLoopbackAddr(opts.Addr) {
				fmt.Fprintf(stderr, "warning: --no-auth with non-loopback addr %s — anyone who can reach the API can mutate resources/secrets\n", opts.Addr)
			}
			return runForeground(bin, opts)
		}
	} else {
		pid = readPidfile(opts.DataDir)
	}
	if opts.NoAuth && !isLoopbackAddr(opts.Addr) {
		fmt.Fprintf(stderr, "warning: --no-auth with non-loopback addr %s — anyone who can reach the API can mutate resources/secrets\n", opts.Addr)
	}
	printUpSummary(stdout, gw, opts, reused, pid, bin)
	return nil
}

func isLoopbackAddr(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	ip := net.ParseIP(host)
	return host == "localhost" || (ip != nil && ip.IsLoopback())
}

func ensureAuthAndContext(opts upOptions, gw string) error {
	var ctxTok string
	if !opts.NoAuth {
		// Prefer daemon data-dir token (created by litefaasd or us).
		tokPath := token.Path(opts.DataDir)
		var tok string
		if t := strings.TrimSpace(opts.Token); t != "" {
			tok = t
			if err := token.WriteFile(tokPath, tok); err != nil {
				return err
			}
		} else {
			var err error
			tok, _, err = token.LoadOrCreate(tokPath)
			if err != nil {
				return err
			}
		}
		ctxTok = tok
		// Copy to config-dir when different.
		if filepath.Clean(opts.DataDir) != filepath.Clean(opts.ConfigDir) {
			cfgTok := token.Path(opts.ConfigDir)
			existing, err := token.ReadFile(cfgTok)
			if err != nil || existing != tok {
				if err := token.WriteFile(cfgTok, tok); err != nil {
					return err
				}
			}
		}
	}

	cfg, err := config.Load(opts.ConfigDir)
	if err != nil {
		return err
	}
	name := opts.Context
	if name == "" {
		name = "default"
	}
	cfg.Contexts[name] = config.Context{Gateway: gw, Token: ctxTok}
	cfg.Current = name
	return cfg.Save(opts.ConfigDir)
}

func daemonArgs(opts upOptions) []string {
	args := []string{
		"--addr", opts.Addr,
		"--data-dir", opts.DataDir,
	}
	if opts.NoAuth {
		args = append(args, "--no-auth")
	} else if t := strings.TrimSpace(opts.Token); t != "" {
		args = append(args, "--token", t)
	}
	if opts.TLSCert != "" {
		args = append(args, "--tls-cert", opts.TLSCert, "--tls-key", opts.TLSKey)
	}
	return args
}

func startDetached(bin string, opts upOptions) (int, error) {
	if err := os.MkdirAll(opts.DataDir, 0o755); err != nil {
		return 0, err
	}
	logPath := filepath.Join(opts.DataDir, logFileName)
	logF, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return 0, err
	}
	cmd := exec.Command(bin, daemonArgs(opts)...)
	cmd.Stdout = logF
	cmd.Stderr = logF
	cmd.Stdin = nil
	setDetachedProc(cmd)
	if err := cmd.Start(); err != nil {
		_ = logF.Close()
		return 0, fmt.Errorf("start litefaasd: %w", err)
	}
	pid := cmd.Process.Pid
	_ = logF.Close()
	// Release so parent exit doesn't wait.
	_ = cmd.Process.Release()
	if err := writePidfile(opts.DataDir, pid); err != nil {
		return pid, err
	}
	return pid, nil
}

func runForeground(bin string, opts upOptions) error {
	cmd := exec.Command(bin, daemonArgs(opts)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func waitHealthy(client *http.Client, gw string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		pr, _, err := probeGateway(client, gw)
		if err == nil && pr == probeReuse {
			return nil
		}
		if time.Now().After(deadline) {
			if err != nil {
				return err
			}
			return fmt.Errorf("timeout after %s", timeout)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func writePidfile(dataDir string, pid int) error {
	path := filepath.Join(dataDir, pidFileName)
	return os.WriteFile(path, []byte(strconv.Itoa(pid)+"\n"), 0o600)
}

func readPidfile(dataDir string) int {
	raw, err := os.ReadFile(filepath.Join(dataDir, pidFileName))
	if err != nil {
		return 0
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		return 0
	}
	return pid
}

func clearStalePidfile(dataDir string, client *http.Client, gw string) error {
	path := filepath.Join(dataDir, pidFileName)
	if _, err := os.Stat(path); err != nil {
		return nil
	}
	pr, _, _ := probeGateway(client, gw)
	if pr == probeReuse {
		return nil
	}
	return os.Remove(path)
}

func printUpSummary(w io.Writer, gw string, opts upOptions, reused bool, pid int, bin string) {
	auth := "on"
	if opts.NoAuth {
		auth = "off"
	}
	fmt.Fprintf(w, "gateway=%s\n", gw)
	fmt.Fprintf(w, "auth=%s\n", auth)
	if opts.NoAuth {
		fmt.Fprintln(w, "warning=auth=off (loopback demos only; anyone who can reach the API can mutate resources/secrets)")
	}
	fmt.Fprintf(w, "data-dir=%s\n", opts.DataDir)
	fmt.Fprintf(w, "context=%s\n", opts.Context)
	fmt.Fprintf(w, "reused=%t\n", reused)
	if pid > 0 {
		fmt.Fprintf(w, "pid=%d\n", pid)
	}
	fmt.Fprintf(w, "log=%s\n", filepath.Join(opts.DataDir, logFileName))
	if bin != "" {
		fmt.Fprintf(w, "litefaasd=%s\n", bin)
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "litefaas gateway ready: %s\n", gw)
	if !reused && pid > 0 {
		fmt.Fprintf(w, "stop: kill $(cat %s)\n", filepath.Join(opts.DataDir, pidFileName))
	}
}
