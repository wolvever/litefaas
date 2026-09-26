package api

import (
	"net/http"
	"sync/atomic"
	"time"
)

type counters struct {
	requests atomic.Int64
	invokes  atomic.Int64
	deploys  atomic.Int64
	errors   atomic.Int64
}

type snapshot struct {
	Started       time.Time `json:"started"`
	UptimeSeconds int64     `json:"uptime_seconds"`
	Requests      int64     `json:"requests"`
	Invokes       int64     `json:"invokes"`
	Deploys       int64     `json:"deploys"`
	Errors        int64     `json:"errors"`
	Resources     int       `json:"resources"`
	Auth          bool      `json:"auth"`
	IdleTTL       string    `json:"idle_ttl,omitempty"`
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Flush() {
	if f, ok := s.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

type flushWriter struct {
	w http.ResponseWriter
	f http.Flusher
}

func (f flushWriter) Write(p []byte) (int, error) {
	n, err := f.w.Write(p)
	f.f.Flush()
	return n, err
}
