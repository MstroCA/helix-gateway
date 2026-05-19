package builtin

import (
	"net/http"
	"strings"

	"helix/internal/auth"
	"helix/internal/plugin"
)

func JWT(v *auth.Validator, publicPaths []string) plugin.Plugin {
	return plugin.New("jwt-auth", func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodOptions || isPublic(r.URL.Path, publicPaths) {
				next.ServeHTTP(w, r)
				return
			}

			token := extractBearer(r.Header.Get("Authorization"))
			if token == "" {
				if c, err := r.Cookie("access_token"); err == nil {
					token = c.Value
				}
			}
			if token == "" {
				plugin.WriteJSON(w, http.StatusUnauthorized, `{"error":"missing_token"}`)
				return
			}

			claims, err := v.Parse(token)
			if err != nil {
				plugin.WriteJSON(w, http.StatusUnauthorized, `{"error":"invalid_token"}`)
				return
			}

			if r.Header.Get("Authorization") == "" {
				r.Header.Set("Authorization", "Bearer "+token)
			}

			ctx := plugin.StoreClaimsInContext(r.Context(), claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
}

func isPublic(path string, publicPaths []string) bool {
	for _, p := range publicPaths {
		if strings.HasSuffix(p, "/") {
			if strings.HasPrefix(path, p) {
				return true
			}
		} else if path == p {
			return true
		}
	}
	return false
}

func extractBearer(header string) string {
	if strings.HasPrefix(header, "Bearer ") {
		return strings.TrimPrefix(header, "Bearer ")
	}
	return ""
}
