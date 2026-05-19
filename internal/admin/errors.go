package admin

import (
	"errors"
	"net/http"

	"helix/internal/store"
)

func storeErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
	case errors.Is(err, store.ErrConflict):
		writeError(w, http.StatusConflict, "CONFLICT", err.Error())
	case errors.Is(err, store.ErrInUse):
		writeError(w, http.StatusConflict, "IN_USE", err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "internal error")
	}
}

func errBadRequest(w http.ResponseWriter, msg string) {
	writeError(w, http.StatusBadRequest, "BAD_REQUEST", msg)
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]string{"code": code, "message": msg},
	})
}
