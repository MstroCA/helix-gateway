package admin

import (
	"encoding/json"
	"net/http"

	"helix/internal/audit"
	"helix/internal/store"
)

func (h *handlers) listKeys(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.store.ListAPIKeys())
}

func (h *handlers) getKey(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	k, err := h.store.GetAPIKey(id)
	if err != nil {
		storeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, k)
}

func (h *handlers) createKey(w http.ResponseWriter, r *http.Request) {
	http.MaxBytesReader(w, r.Body, 1<<20)
	var req struct {
		Name   string   `json:"name"`
		Scopes []string `json:"scopes,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errBadRequest(w, "invalid request body")
		return
	}
	if req.Name == "" {
		errBadRequest(w, "name is required")
		return
	}

	plaintext, hash, err := store.GenerateAPIKey()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "key_gen_error", "failed to generate key")
		return
	}

	k := &store.APIKey{
		Name:    req.Name,
		KeyHash: hash,
		Active:  true,
		Scopes:  req.Scopes,
	}
	created, err := h.store.CreateAPIKey(k)
	if err != nil {
		storeErr(w, err)
		return
	}

	user, _, _ := r.BasicAuth()
	h.auditLog.Record(audit.Entry{
		Action:     "create_api_key",
		Resource:   "api_key",
		ResourceID: created.ID,
		User:       user,
		RemoteAddr: r.RemoteAddr,
		Details:    map[string]string{"name": created.Name},
	})

	writeJSON(w, http.StatusCreated, map[string]any{
		"key":       created,
		"plaintextKey": plaintext,
	})
}

func (h *handlers) deleteKey(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	k, _ := h.store.GetAPIKey(id)
	if err := h.store.DeleteAPIKey(id); err != nil {
		storeErr(w, err)
		return
	}

	user, _, _ := r.BasicAuth()
	name := ""
	if k != nil {
		name = k.Name
	}
	h.auditLog.Record(audit.Entry{
		Action:     "delete_api_key",
		Resource:   "api_key",
		ResourceID: id,
		User:       user,
		RemoteAddr: r.RemoteAddr,
		Details:    map[string]string{"name": name},
	})
	w.WriteHeader(http.StatusNoContent)
}

func (h *handlers) toggleKey(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	existing, err := h.store.GetAPIKey(id)
	if err != nil {
		storeErr(w, err)
		return
	}
	updated, err := h.store.ToggleAPIKey(id, !existing.Active)
	if err != nil {
		storeErr(w, err)
		return
	}

	user, _, _ := r.BasicAuth()
	h.auditLog.Record(audit.Entry{
		Action:     "toggle_api_key",
		Resource:   "api_key",
		ResourceID: id,
		User:       user,
		RemoteAddr: r.RemoteAddr,
		Details:    map[string]bool{"active": updated.Active},
	})
	writeJSON(w, http.StatusOK, updated)
}
