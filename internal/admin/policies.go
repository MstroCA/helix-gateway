package admin

import (
	"encoding/json"
	"net/http"

	"helix/internal/store"
)

func (h *handlers) listPolicies(w http.ResponseWriter, r *http.Request) {
	policies := h.store.ListIPPolicies()
	writeJSON(w, http.StatusOK, map[string]any{"data": policies, "total": len(policies)})
}

func (h *handlers) createPolicy(w http.ResponseWriter, r *http.Request) {
	var body store.IPPolicy
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
		errBadRequest(w, "invalid request body")
		return
	}
	created, err := h.store.CreateIPPolicy(&body)
	if err != nil {
		storeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (h *handlers) getPolicy(w http.ResponseWriter, r *http.Request) {
	p, err := h.store.GetIPPolicy(r.PathValue("id"))
	if err != nil {
		storeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (h *handlers) updatePolicy(w http.ResponseWriter, r *http.Request) {
	var body store.IPPolicy
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
		errBadRequest(w, "invalid request body")
		return
	}
	updated, err := h.store.UpdateIPPolicy(r.PathValue("id"), &body)
	if err != nil {
		storeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (h *handlers) deletePolicy(w http.ResponseWriter, r *http.Request) {
	if err := h.store.DeleteIPPolicy(r.PathValue("id")); err != nil {
		storeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *handlers) togglePolicy(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Active bool `json:"active"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
		errBadRequest(w, "invalid request body")
		return
	}
	updated, err := h.store.ToggleIPPolicy(r.PathValue("id"), body.Active)
	if err != nil {
		storeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}
