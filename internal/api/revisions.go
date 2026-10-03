package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/wolvever/litefaas/internal/store"
	"github.com/wolvever/litefaas/internal/types"
)

func (s *Server) recordRevision(name, image, imageID, status string, snap types.RevisionSnapshot) (types.Revision, error) {
	snap = types.SanitizeSnapshot(snap)
	if snap.Image == "" {
		snap.Image = image
	}
	rev, err := s.store.AddRevisionFull(types.Revision{
		Name:     name,
		Image:    image,
		ImageID:  imageID,
		Status:   status,
		Target:   "prod",
		Snapshot: snap,
	})
	if err != nil {
		return types.Revision{}, err
	}
	_, _ = s.store.PruneRevisions(name, store.DefaultRevisionKeep)
	return rev, nil
}

func (s *Server) handleListRevisions(w http.ResponseWriter, r *http.Request) {
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
	revs, err := s.store.ListRevisions(name)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorBody{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, revs)
}

func (s *Server) handlePinRevision(w http.ResponseWriter, r *http.Request) {
	s.setPin(w, r, true)
}

func (s *Server) handleUnpinRevision(w http.ResponseWriter, r *http.Request) {
	s.setPin(w, r, false)
}

func (s *Server) setPin(w http.ResponseWriter, r *http.Request, pinned bool) {
	if s.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorBody{Error: "store not configured"})
		return
	}
	name := r.PathValue("name")
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "revision id must be a positive integer"})
		return
	}
	rev, err := s.store.SetRevisionPinned(name, id, pinned)
	if errors.Is(err, store.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, errorBody{Error: "revision not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorBody{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, rev)
}

type rollbackRequest struct {
	ID int64 `json:"id"`
}

func (s *Server) handleRollback(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorBody{Error: "store not configured"})
		return
	}
	if s.runner == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorBody{Error: "runner not configured"})
		return
	}
	name := r.PathValue("name")
	res, err := s.store.Get(name)
	if errors.Is(err, store.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, errorBody{Error: "not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorBody{Error: err.Error()})
		return
	}
	var req rollbackRequest
	if r.Body != nil {
		dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
		if err := dec.Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			writeJSON(w, http.StatusBadRequest, errorBody{Error: "invalid json: " + err.Error()})
			return
		}
	}
	revs, err := s.store.ListRevisions(name)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorBody{Error: err.Error()})
		return
	}
	target, err := selectRollbackRevision(revs, res.Image, req.ID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: err.Error()})
		return
	}
	// Image swap only. Does not restore env, volumes, or undo migrations.
	res.Image = target.Image
	if _, err := s.store.Update(res); err != nil {
		writeJSON(w, http.StatusInternalServerError, errorBody{Error: err.Error()})
		return
	}
	secEnv := r.URL.Query().Get("env")
	inject := queryTruthy(r.URL.Query().Get("inject_env")) || queryTruthy(r.URL.Query().Get("inject-env"))
	deployRes, err := s.withResolvedSecrets(res, secEnv, inject)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: err.Error()})
		return
	}
	// Rollbacks skip release commands (they must not re-run migrations).
	deployRes.Release = nil
	out, err := s.runner.Deploy(r.Context(), deployRes)
	if err != nil {
		_, _ = s.recordRevision(name, res.Image, "", "failed", target.Snapshot)
		writeJSON(w, http.StatusBadGateway, errorBody{Error: "rollback: " + err.Error()})
		return
	}
	rev, err := s.recordRevision(name, res.Image, out.ImageID, "rolled_back", target.Snapshot)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorBody{Error: err.Error()})
		return
	}
	s.metrics.deploys.Add(1)
	s.touch(name)
	writeJSON(w, http.StatusAccepted, deployResponse{Revision: rev, Endpoint: out.Endpoint, Container: out.Container})
}

func selectRollbackRevision(revs []types.Revision, currentImage string, id int64) (types.Revision, error) {
	if id > 0 {
		for _, rev := range revs {
			if rev.ID != id {
				continue
			}
			if rev.Target != "" && rev.Target != "prod" {
				return types.Revision{}, fmt.Errorf("revision %d is a %s slot, not a prod rollback target", id, rev.Target)
			}
			if !rollbackable(rev.Status) {
				return types.Revision{}, fmt.Errorf("revision %d status %q cannot be rolled back", id, rev.Status)
			}
			if rev.Image == "" {
				return types.Revision{}, fmt.Errorf("revision %d has no image", id)
			}
			return rev, nil
		}
		return types.Revision{}, fmt.Errorf("revision %d not found", id)
	}
	var ok []types.Revision
	for _, rev := range revs {
		if rev.Target != "" && rev.Target != "prod" {
			continue
		}
		if rollbackable(rev.Status) && rev.Image != "" {
			ok = append(ok, rev)
		}
	}
	for i := len(ok) - 1; i >= 0; i-- {
		if ok[i].Image != currentImage {
			return ok[i], nil
		}
	}
	if len(ok) >= 2 {
		return ok[len(ok)-2], nil
	}
	return types.Revision{}, errors.New("no previous revision to roll back to")
}

func rollbackable(status string) bool {
	return status == "deployed" || status == "rolled_back"
}
