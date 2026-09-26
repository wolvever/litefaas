// Package metrics holds process-local counters for GET /v1/metrics.
package metrics

import (
	"sync/atomic"
	"time"
)

type Metrics struct {
	started time.Time
	invokes atomic.Int64
	deploys atomic.Int64
	edge    atomic.Int64
	errors  atomic.Int64
}

func New() *Metrics {
	return &Metrics{started: time.Now()}
}

func (m *Metrics) IncInvokes() {
	if m != nil {
		m.invokes.Add(1)
	}
}

func (m *Metrics) IncDeploys() {
	if m != nil {
		m.deploys.Add(1)
	}
}

func (m *Metrics) IncEdge() {
	if m != nil {
		m.edge.Add(1)
	}
}

func (m *Metrics) IncErrors() {
	if m != nil {
		m.errors.Add(1)
	}
}

type Snapshot struct {
	UptimeSeconds int64 `json:"uptime_seconds"`
	Invokes       int64 `json:"invokes"`
	Deploys       int64 `json:"deploys"`
	EdgeRequests  int64 `json:"edge_requests"`
	Errors        int64 `json:"errors"`
	Resources     int   `json:"resources"`
}

func (m *Metrics) Snapshot(resources int) Snapshot {
	if m == nil {
		return Snapshot{Resources: resources}
	}
	return Snapshot{
		UptimeSeconds: int64(time.Since(m.started).Seconds()),
		Invokes:       m.invokes.Load(),
		Deploys:       m.deploys.Load(),
		EdgeRequests:  m.edge.Load(),
		Errors:        m.errors.Load(),
		Resources:     resources,
	}
}
