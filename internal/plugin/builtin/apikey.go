package builtin

import (
	"net/http"

	"helix/internal/plugin"
	"helix/internal/store"
)

// APIKeyAuth validates requests using the X-API-Key (or custom) header.
// Keys are validated against the store's API key registry using SHA-256 hashes.
func APIKeyAuth(st *store.Store, headerName string) plugin.Plugin {
	if headerName == "" {
		headerName = "X-API-Key"
	}
	return plugin.New("api-key-auth", func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get(headerName)
			if key == "" {
				plugin.WriteJSON(w, http.StatusUnauthorized, `{"error":"api_key_required"}`)
				return
			}
			apiKey, ok := st.ValidateAPIKey(key)
			if !ok {
				plugin.WriteJSON(w, http.StatusUnauthorized, `{"error":"api_key_invalid"}`)
				return
			}
			r.Header.Set("X-Gateway-API-Key-Name", apiKey.Name)
			next.ServeHTTP(w, r)
		})
	})
}
