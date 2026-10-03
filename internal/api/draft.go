package api

import (
	"net/http"

	"github.com/wolvever/litefaas/internal/runner"
	"github.com/wolvever/litefaas/internal/store"
	"github.com/wolvever/litefaas/internal/types"
)

// deployDraft starts litefaas-<name>-draft only. It does not update the stored
// prod image and does not run release commands (those would hit shared volumes).
func (s *Server) deployDraft(w http.ResponseWriter, r *http.Request, res types.Resource, req deployRequest) {
	dr, ok := s.runner.(runner.DraftRunner)
	if !ok {
		writeJSON(w, http.StatusNotImplemented, errorBody{Error: "runner has no draft slot"})
		return
	}
	run := res
	if req.Image != "" {
		run.Image = req.Image
	}
	if req.Env != nil {
		run.Env = req.Env
	}
	if req.Memory > 0 {
		run.Memory = req.Memory
	}
	if req.Port > 0 {
		run.Port = req.Port
	}
	if req.Health != "" {
		run.Health = req.Health
	}
	if req.Volumes != nil {
		run.Volumes = req.Volumes
	}
	run.Release = nil
	secEnv := r.URL.Query().Get("env")
	if secEnv == "" {
		secEnv = "draft"
	}
	inject := queryTruthy(r.URL.Query().Get("inject_env")) || queryTruthy(r.URL.Query().Get("inject-env"))
	deployRes, err := s.withResolvedSecrets(run, secEnv, inject)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: err.Error()})
		return
	}
	deployRes.Release = nil
	out, err := dr.DeployDraft(r.Context(), deployRes)
	if err != nil {
		_, _ = s.store.AddRevisionFull(types.Revision{
			Name: res.Name, Image: run.Image, Status: "failed", Target: "draft", Snapshot: req.Snapshot,
		})
		_, _ = s.store.PruneRevisions(res.Name, store.DefaultRevisionKeep)
		writeJSON(w, http.StatusBadGateway, errorBody{Error: "draft deploy: " + err.Error()})
		return
	}
	snap := types.SanitizeSnapshot(req.Snapshot)
	if snap.Image == "" {
		snap.Image = run.Image
	}
	rev, err := s.store.AddRevisionFull(types.Revision{
		Name: res.Name, Image: run.Image, ImageID: out.ImageID, Status: "deployed", Target: "draft", Snapshot: snap,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorBody{Error: err.Error()})
		return
	}
	_, _ = s.store.PruneRevisions(res.Name, store.DefaultRevisionKeep)
	s.metrics.deploys.Add(1)
	writeJSON(w, http.StatusAccepted, deployResponse{Revision: rev, Endpoint: out.Endpoint, Container: out.Container})
}
