package builtin

import (
	"net/http"
	"strings"

	"helix/internal/plugin"
)

type TransformConfig struct {
	SetRequestHeaders     map[string]string `json:"set_request_headers"`
	RemoveRequestHeaders  []string          `json:"remove_request_headers"`
	SetResponseHeaders    map[string]string `json:"set_response_headers"`
	RemoveResponseHeaders []string          `json:"remove_response_headers"`
	URLRewrite            *URLRewriteConfig `json:"url_rewrite,omitempty"`
}

type URLRewriteConfig struct {
	PrefixMatch string `json:"prefix_match"`
	Replacement string `json:"replacement"`
}

func Transform(cfg TransformConfig) plugin.Plugin {
	return plugin.New("transform", func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for k, v := range cfg.SetRequestHeaders {
				r.Header.Set(k, v)
			}
			for _, k := range cfg.RemoveRequestHeaders {
				r.Header.Del(k)
			}

			if cfg.URLRewrite != nil && cfg.URLRewrite.PrefixMatch != "" {
				if strings.HasPrefix(r.URL.Path, cfg.URLRewrite.PrefixMatch) {
					r = r.Clone(r.Context())
					r.URL.Path = cfg.URLRewrite.Replacement + strings.TrimPrefix(r.URL.Path, cfg.URLRewrite.PrefixMatch)
					if r.URL.Path == "" {
						r.URL.Path = "/"
					}
				}
			}

			if len(cfg.SetResponseHeaders) > 0 || len(cfg.RemoveResponseHeaders) > 0 {
				cfgPtr := cfg
			next.ServeHTTP(&transformWriter{ResponseWriter: w, cfg: &cfgPtr}, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	})
}

type transformWriter struct {
	http.ResponseWriter
	cfg         *TransformConfig
	wroteHeader bool
}

func (tw *transformWriter) WriteHeader(status int) {
	if !tw.wroteHeader {
		tw.wroteHeader = true
		for k, v := range tw.cfg.SetResponseHeaders {
			tw.Header().Set(k, v)
		}
		for _, k := range tw.cfg.RemoveResponseHeaders {
			tw.Header().Del(k)
		}
	}
	tw.ResponseWriter.WriteHeader(status)
}

func (tw *transformWriter) Write(b []byte) (int, error) {
	if !tw.wroteHeader {
		tw.WriteHeader(http.StatusOK)
	}
	return tw.ResponseWriter.Write(b)
}
