package admin

import (
	"encoding/json"
	"net/http"
	"time"

	"helix/internal/store"
)

func (h *handlers) listUpstreams(w http.ResponseWriter, r *http.Request) {
	upstreams := h.store.ListUpstreams()
	writeJSON(w, http.StatusOK, map[string]any{"data": upstreams, "total": len(upstreams)})
}

func (h *handlers) createUpstream(w http.ResponseWriter, r *http.Request) {
	var body store.Upstream
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
		errBadRequest(w, "invalid request body")
		return
	}
	created, err := h.store.CreateUpstream(&body)
	if err != nil {
		storeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (h *handlers) getUpstream(w http.ResponseWriter, r *http.Request) {
	u, err := h.store.GetUpstream(r.PathValue("id"))
	if err != nil {
		storeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, u)
}

func (h *handlers) updateUpstream(w http.ResponseWriter, r *http.Request) {
	var body store.Upstream
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
		errBadRequest(w, "invalid request body")
		return
	}
	updated, err := h.store.UpdateUpstream(r.PathValue("id"), &body)
	if err != nil {
		storeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (h *handlers) deleteUpstream(w http.ResponseWriter, r *http.Request) {
	if err := h.store.DeleteUpstream(r.PathValue("id")); err != nil {
		storeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *handlers) upstreamHealth(w http.ResponseWriter, r *http.Request) {
	u, err := h.store.GetUpstream(r.PathValue("id"))
	if err != nil {
		storeErr(w, err)
		return
	}

	healthPath := u.HealthPath
	if healthPath == "" {
		healthPath = "/health"
	}

	client := &http.Client{Timeout: 5 * time.Second}
	start := time.Now()
	resp, err := client.Get(u.URL + healthPath)
	latency := time.Since(start).Milliseconds()

	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"upstream_id": u.ID,
			"status":      "unhealthy",
			"error":       err.Error(),
			"latency_ms":  latency,
		})
		return
	}
	defer resp.Body.Close()

	status := "healthy"
	if resp.StatusCode >= 400 {
		status = "unhealthy"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"upstream_id": u.ID,
		"status":      status,
		"code":        resp.StatusCode,
		"latency_ms":  latency,
	})
}
