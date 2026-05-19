package tls

import (
	"crypto/tls"
	"net/http"

	"golang.org/x/crypto/acme/autocert"
)

type Manager struct {
	ac *autocert.Manager
}

// New creates a TLS manager using ACME (Let's Encrypt) for the given domains.
// cacheDir is the directory where certificates are persisted; defaults to /var/cache/helix/acme.
func New(domains []string, cacheDir string) *Manager {
	if cacheDir == "" {
		cacheDir = "/var/cache/helix/acme"
	}
	return &Manager{
		ac: &autocert.Manager{
			Prompt:     autocert.AcceptTOS,
			HostPolicy: autocert.HostWhitelist(domains...),
			Cache:      autocert.DirCache(cacheDir),
		},
	}
}

// TLSConfig returns a *tls.Config that automatically obtains and renews certificates.
func (m *Manager) TLSConfig() *tls.Config {
	return m.ac.TLSConfig()
}

// HTTPChallengeHandler returns an http.Handler for port 80 that handles ACME
// HTTP-01 challenges and redirects all other traffic to HTTPS.
func (m *Manager) HTTPChallengeHandler() http.Handler {
	return m.ac.HTTPHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		target := "https://" + r.Host + r.URL.RequestURI()
		http.Redirect(w, r, target, http.StatusMovedPermanently)
	}))
}
