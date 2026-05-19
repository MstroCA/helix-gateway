package builtin

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"helix/internal/plugin"
)

var (
	sqlPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(UNION\s+SELECT|SELECT\s+.*\s+FROM|INSERT\s+INTO|UPDATE\s+.*\s+SET|DELETE\s+FROM|DROP\s+TABLE|DROP\s+DATABASE|ALTER\s+TABLE|EXEC\s*\(|EXECUTE\s*\(|CAST\s*\(|CONVERT\s*\(|WAITFOR\s+DELAY)\b`),
		regexp.MustCompile(`(?i)(--\s|;--|\/\*.*\*\/)`),
		regexp.MustCompile(`'[^']*'[^']*=\s*'[^']*'`),
		regexp.MustCompile(`\b(OR|AND)\s+[\d]+=[\d]+`),
	}

	xssPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)<\s*script[^>]*>`),
		regexp.MustCompile(`(?i)javascript\s*:`),
		regexp.MustCompile(
			`(?i)\bon(?:abort|animation(?:cancel|end|iteration|start)|auxclick|before(?:copy|cut|paste|unload)|blur|cancel|canplay|canplaythrough|change|click|close|contextmenu|copy|cuechange|cut|dblclick|drag|dragend|dragenter|dragleave|dragover|dragstart|drop|durationchange|emptied|ended|error|focus|fullscreenchange|fullscreenerror|hashchange|input|invalid|keydown|keypress|keyup|load(?:start|end)?|lostpointercapture|message|mousedown|mouseenter|mouseleave|mousemove|mouseout|mouseover|mouseup|mousewheel|offline|online|pagehide|pageshow|paste|pause|play|playing|pointer(?:cancel|down|enter|leave|move|out|over|rawupdate|up)|popstate|progress|ratechange|reset|resize|scroll|securitypolicyviolation|seeked|seeking|select|slotchange|stalled|submit|suspend|timeupdate|toggle|touch(?:cancel|end|move|start)|transition(?:cancel|end|run|start)|unload|volumechange|waiting|wheel)\s*=`,
		),
		regexp.MustCompile(`(?i)(vbscript|data:text\/html)`),
	}

	pathTraversalPattern = regexp.MustCompile(`(\.\.[/\\]){2,}`)
)

func SecurityHeaders() plugin.Plugin {
	return plugin.New("security-headers", func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("X-Frame-Options", "DENY")
			h.Set("X-XSS-Protection", "1; mode=block")
			h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
			h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			h.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
			next.ServeHTTP(w, r)
		})
	})
}

func RequestSecurity() plugin.Plugin {
	return plugin.New("request-security", func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}
			if pathTraversalPattern.MatchString(r.URL.Path) {
				plugin.WriteJSON(w, http.StatusBadRequest, `{"error":"invalid_path"}`)
				return
			}
			raw := r.URL.RawQuery
			if raw != "" {
				decoded, err := url.QueryUnescape(raw)
				if err != nil {
					plugin.WriteJSON(w, http.StatusBadRequest, `{"error":"invalid_query"}`)
					return
				}
				decoded = strings.ToLower(decoded)
				for _, p := range sqlPatterns {
					if p.MatchString(decoded) {
						plugin.WriteJSON(w, http.StatusBadRequest, `{"error":"invalid_request"}`)
						return
					}
				}
				for _, p := range xssPatterns {
					if p.MatchString(decoded) {
						plugin.WriteJSON(w, http.StatusBadRequest, `{"error":"invalid_request"}`)
						return
					}
				}
			}
			next.ServeHTTP(w, r)
		})
	})
}
