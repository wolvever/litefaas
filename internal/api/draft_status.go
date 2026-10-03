package api

import (
	"errors"
	"net/http"

	"github.com/wolvever/litefaas/internal/runner"
	"github.com/wolvever/litefaas/internal/store"
)

// handleDraftStatus reports whether litefaas-<name>-draft is up.
// It only inspects the draft runner. It does not deploy or cold-start prod.
func (s *Server) handleDraftStatus(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorBody{Error: "store not configured"})
		return
	}
	name := r.PathValue("name")
	if _, err := s.store.Get(name); errors.Is(err, store.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, errorBody{Error: "not found"})
		return
	} else if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorBody{Error: err.Error()})
		return
	}
	body := draftStatusBody{
		Name:      name,
		Container: runner.DraftContainerName(name),
	}
	dr, ok := s.runner.(runner.DraftRunner)
	if !ok || dr == nil {
		writeJSON(w, http.StatusOK, body)
		return
	}
	if _, err := dr.EndpointDraft(r.Context(), name); err == nil {
		body.Up = true
		writeJSON(w, http.StatusOK, body)
		return
	} else if errors.Is(err, runner.ErrNotDeployed) {
		writeJSON(w, http.StatusOK, body)
		return
	} else {
		writeJSON(w, http.StatusBadGateway, errorBody{Error: err.Error()})
	}
}

type draftStatusBody struct {
	Name      string `json:"name"`
	Up        bool   `json:"up"`
	Container string `json:"container"`
}
