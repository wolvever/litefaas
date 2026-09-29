package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/wolvever/litefaas/internal/proxy"
	"github.com/wolvever/litefaas/internal/store"
)

func (s *Server) handleEdgeRules(w http.ResponseWriter, r *http.Request) {
	rules, err := s.loadEdgeRules()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorBody{Error: err.Error()})
		return
	}
	if rules == nil {
		rules = []proxy.EdgeRule{}
	}
	writeJSON(w, http.StatusOK, rules)
}

func (s *Server) handlePutEdgeRules(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorBody{Error: "store not configured"})
		return
	}
	var in []proxy.EdgeRule
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "invalid json: " + err.Error()})
		return
	}
	if in == nil {
		in = []proxy.EdgeRule{}
	}
	specs := make([]store.EdgeRuleSpec, 0, len(in))
	for i, rule := range in {
		from := strings.TrimSpace(rule.From)
		if from == "" {
			writeJSON(w, http.StatusBadRequest, errorBody{Error: fmt.Sprintf("edge_rules[%d]: from is required", i)})
			return
		}
		if !strings.HasPrefix(from, "/") {
			writeJSON(w, http.StatusBadRequest, errorBody{Error: fmt.Sprintf("edge_rules[%d]: from must start with /", i)})
			return
		}
		if rule.Status != 0 {
			switch rule.Status {
			case 200, 301, 302, 303, 307, 308:
			default:
				writeJSON(w, http.StatusBadRequest, errorBody{Error: fmt.Sprintf("edge_rules[%d]: unsupported status %d", i, rule.Status)})
				return
			}
			if strings.TrimSpace(rule.To) == "" {
				writeJSON(w, http.StatusBadRequest, errorBody{Error: fmt.Sprintf("edge_rules[%d]: to is required for status %d", i, rule.Status)})
				return
			}
		}
		specs = append(specs, store.EdgeRuleSpec{
			From:    from,
			To:      rule.To,
			Status:  rule.Status,
			Headers: rule.Headers,
			Force:   rule.Force,
			Source:  rule.Source,
		})
	}
	if err := s.store.SetEdgeRules(specs); err != nil {
		writeJSON(w, http.StatusInternalServerError, errorBody{Error: err.Error()})
		return
	}
	s.handleEdgeRules(w, r)
}

func (s *Server) handleClearEdgeRules(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorBody{Error: "store not configured"})
		return
	}
	if err := s.store.ClearEdgeRules(); err != nil {
		writeJSON(w, http.StatusInternalServerError, errorBody{Error: err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) loadEdgeRules() ([]proxy.EdgeRule, error) {
	if s.store == nil {
		return nil, nil
	}
	specs, ok, err := s.store.GetEdgeRules()
	if err != nil || !ok {
		return nil, err
	}
	out := make([]proxy.EdgeRule, 0, len(specs))
	for _, sp := range specs {
		out = append(out, proxy.EdgeRule{
			From:    sp.From,
			To:      sp.To,
			Status:  sp.Status,
			Headers: sp.Headers,
			Force:   sp.Force,
			Source:  sp.Source,
		})
	}
	return out, nil
}
