package builtin

import (
	"net"
	"net/http"

	"helix/internal/plugin"
	"helix/internal/store"
)

// IPPolicyEnforcer applies active allow/deny CIDR policies from the store.
// All active policies are evaluated in order; the first matching deny or missing allow blocks the request.
func IPPolicyEnforcer(st *store.Store) plugin.Plugin {
	return plugin.New("ip-policy", func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := net.ParseIP(plugin.ClientIP(r))

			for _, p := range st.ListIPPolicies() {
				if !p.Active {
					continue
				}
				matched := cidrContains(p.CIDRs, ip)
				if p.Mode == "allow" && !matched {
					plugin.WriteJSON(w, http.StatusForbidden, `{"error":"ip_not_allowed"}`)
					return
				}
				if p.Mode == "deny" && matched {
					plugin.WriteJSON(w, http.StatusForbidden, `{"error":"ip_denied"}`)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	})
}

func cidrContains(cidrs []string, ip net.IP) bool {
	if ip == nil {
		return false
	}
	for _, cidr := range cidrs {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if network.Contains(ip) {
			return true
		}
	}
	return false
}
