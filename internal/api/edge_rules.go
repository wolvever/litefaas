package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/wolvever/litefaas/internal/proxy"
	"github.com/wolvever/litefaas/internal/store"
)

// projectIDRE is one resource name, or several joined by '+'.
var projectIDRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}(\+[A-Za-z0-9][A-Za-z0-9._-]{0,63})*$`)

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
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "invalid body"})
		return
	}
	trim := bytes.TrimSpace(raw)
	if len(trim) == 0 {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "invalid json: empty body"})
		return
	}
	switch trim[0] {
	case '[':
		// Legacy single-project replace of the whole table.
		var in []proxy.EdgeRule
		if err := json.Unmarshal(trim, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, errorBody{Error: "invalid json: " + err.Error()})
			return
		}
		specs, err := normalizeEdgeRules(in)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, errorBody{Error: err.Error()})
			return
		}
		if err := s.store.SetEdgeRules(specs); err != nil {
			writeJSON(w, http.StatusInternalServerError, errorBody{Error: err.Error()})
			return
		}
	case '{':
		var in struct {
			Project string           `json:"project"`
			Rules   []proxy.EdgeRule `json:"rules"`
		}
		if err := json.Unmarshal(trim, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, errorBody{Error: "invalid json: " + err.Error()})
			return
		}
		project := strings.TrimSpace(in.Project)
		if !projectIDRE.MatchString(project) || len(project) > 512 {
			writeJSON(w, http.StatusBadRequest, errorBody{Error: "project id is required (resource names joined by +)"})
			return
		}
		specs, err := normalizeEdgeRules(in.Rules)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, errorBody{Error: err.Error()})
			return
		}
		if err := s.store.SetProjectEdgeRules(project, specs); err != nil {
			writeJSON(w, http.StatusInternalServerError, errorBody{Error: err.Error()})
			return
		}
	default:
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "invalid json: want array or {project,rules}"})
		return
	}
	s.handleEdgeRules(w, r)
}

func normalizeEdgeRules(in []proxy.EdgeRule) ([]store.EdgeRuleSpec, error) {
	if in == nil {
		in = []proxy.EdgeRule{}
	}
	specs := make([]store.EdgeRuleSpec, 0, len(in))
	for i, rule := range in {
		from := strings.TrimSpace(rule.From)
		if from == "" {
			return nil, fmt.Errorf("edge_rules[%d]: from is required", i)
		}
		if !strings.HasPrefix(from, "/") {
			return nil, fmt.Errorf("edge_rules[%d]: from must start with /", i)
		}
		if rule.Status != 0 {
			switch rule.Status {
			case 200, 301, 302, 303, 307, 308:
			default:
				return nil, fmt.Errorf("edge_rules[%d]: unsupported status %d", i, rule.Status)
			}
			if strings.TrimSpace(rule.To) == "" {
				return nil, fmt.Errorf("edge_rules[%d]: to is required for status %d", i, rule.Status)
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
	return specs, nil
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
			Project: sp.Project,
		})
	}
	return out, nil
}
