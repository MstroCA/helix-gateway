package admin

import (
	"encoding/json"
	"net/http"

	"helix/internal/store"
)

func (h *handlers) listRoutes(w http.ResponseWriter, r *http.Request) {
	routes := h.store.ListRoutes()
	writeJSON(w, http.StatusOK, map[string]any{"data": routes, "total": len(routes)})
}

func (h *handlers) createRoute(w http.ResponseWriter, r *http.Request) {
	var body store.Route
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
		errBadRequest(w, "invalid request body")
		return
	}
	created, err := h.store.CreateRoute(&body)
	if err != nil {
		storeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (h *handlers) getRoute(w http.ResponseWriter, r *http.Request) {
	route, err := h.store.GetRoute(r.PathValue("id"))
	if err != nil {
		storeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, route)
}

func (h *handlers) updateRoute(w http.ResponseWriter, r *http.Request) {
	var body store.Route
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
		errBadRequest(w, "invalid request body")
		return
	}
	updated, err := h.store.UpdateRoute(r.PathValue("id"), &body)
	if err != nil {
		storeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (h *handlers) deleteRoute(w http.ResponseWriter, r *http.Request) {
	if err := h.store.DeleteRoute(r.PathValue("id")); err != nil {
		storeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *handlers) toggleRoute(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Active bool `json:"active"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
		errBadRequest(w, "invalid request body")
		return
	}
	updated, err := h.store.ToggleRoute(r.PathValue("id"), body.Active)
	if err != nil {
		storeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}
