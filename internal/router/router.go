package router

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"golang.org/x/net/http2"

	"helix/internal/health"
	"helix/internal/lb"
	helixmetrics "helix/internal/metrics"
	"helix/internal/plugin"
	"helix/internal/store"
)

type Router struct {
	table    atomic.Pointer[routeTable]
	fallback http.Handler
	st       *store.Store
	reg      *plugin.Registry
	tracker  *helixmetrics.Tracker
	checker  *health.Checker
	reloadMu sync.Mutex
}

type routeTable struct {
	exactEntries  []entry
	prefixEntries []entry
	regexEntries  []entry
}

type entry struct {
	route   *store.Route
	handler http.Handler
	re      *regexp.Regexp
}

type responseRecorder struct {
	http.ResponseWriter
	status int
}

func (r *responseRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func New(st *store.Store, reg *plugin.Registry, tracker *helixmetrics.Tracker, checker *health.Checker, fallback http.Handler) *Router {
	r := &Router{st: st, reg: reg, tracker: tracker, checker: checker, fallback: fallback}
	r.reload()
	st.OnChange(r.reload)
	return r
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if h := r.table.Load().match(req); h != nil {
		h.ServeHTTP(w, req)
		return
	}
	r.fallback.ServeHTTP(w, req)
}

func (rt *routeTable) match(req *http.Request) http.Handler {
	host := stripPort(req.Host)
	path := req.URL.Path
	method := req.Method

	for _, e := range rt.exactEntries {
		if matchEntry(e, host, path, method, req) {
			return e.handler
		}
	}
	for _, e := range rt.prefixEntries {
		if matchEntry(e, host, path, method, req) {
			return e.handler
		}
	}
	for _, e := range rt.regexEntries {
		if matchEntry(e, host, path, method, req) {
			return e.handler
		}
	}
	return nil
}

func matchEntry(e entry, host, path, method string, req *http.Request) bool {
	m := e.route.Match
	if !matchHosts(m.Hosts, host) || !matchMethod(m.Methods, method) || !matchHeaders(m.Headers, req) {
		return false
	}
	switch m.PathMode {
	case store.PathModeExact:
		return path == m.Path
	case store.PathModePrefix:
		return strings.HasPrefix(path, m.Path)
	case store.PathModeRegex:
		return e.re != nil && e.re.MatchString(path)
	}
	return false
}

func matchHosts(hosts []string, host string) bool {
	if len(hosts) == 0 {
		return true
	}
	for _, h := range hosts {
		if h == host || h == "*" {
			return true
		}
	}
	return false
}

func matchMethod(methods []string, method string) bool {
	if len(methods) == 0 {
		return true
	}
	for _, m := range methods {
		if strings.EqualFold(m, method) {
			return true
		}
	}
	return false
}

func matchHeaders(headers map[string]string, req *http.Request) bool {
	for k, v := range headers {
		if req.Header.Get(k) != v {
			return false
		}
	}
	return true
}

func (r *Router) reload() {
	r.reloadMu.Lock()
	defer r.reloadMu.Unlock()

	upstreams := r.buildUpstreamMap()
	routes := r.st.ListRoutes()

	table := &routeTable{}
	for _, route := range routes {
		if !route.Active {
			continue
		}

		var upH http.Handler
		if len(route.WeightedUpstreams) > 0 {
			upH = r.buildTrafficSplitHandler(route.WeightedUpstreams, upstreams)
		} else {
			var ok bool
			upH, ok = upstreams[route.UpstreamID]
			if !ok {
				slog.Warn("router: route references unknown upstream, skipping",
					"route", route.Name, "upstream_id", route.UpstreamID)
				continue
			}
		}

		h := r.buildRouteHandler(route, upH)
		e := entry{route: route, handler: h}

		switch route.Match.PathMode {
		case store.PathModeRegex:
			re, err := regexp.Compile(route.Match.Path)
			if err != nil {
				slog.Warn("router: invalid regex in route, skipping", "route", route.Name, "err", err)
				continue
			}
			e.re = re
			table.regexEntries = append(table.regexEntries, e)
		case store.PathModeExact:
			table.exactEntries = append(table.exactEntries, e)
		default:
			table.prefixEntries = append(table.prefixEntries, e)
		}
	}

	sort.Slice(table.prefixEntries, func(i, j int) bool {
		return len(table.prefixEntries[i].route.Match.Path) > len(table.prefixEntries[j].route.Match.Path)
	})

	r.table.Store(table)
	slog.Info("router reloaded",
		"exact", len(table.exactEntries),
		"prefix", len(table.prefixEntries),
		"regex", len(table.regexEntries),
	)
}

func (r *Router) buildUpstreamMap() map[string]http.Handler {
	upstreams := r.st.ListUpstreams()
	m := make(map[string]http.Handler, len(upstreams))
	for _, u := range upstreams {
		var (
			h   http.Handler
			err error
		)
		if len(u.Endpoints) > 1 {
			h, err = r.buildLBHandler(u)
		} else {
			h, err = buildUpstreamProxy(u, u.URL)
		}
		if err != nil {
			slog.Warn("router: failed to build upstream proxy", "upstream", u.Name, "err", err)
			continue
		}
		m[u.ID] = h
	}
	return m
}

// buildLBHandler creates a load-balancing handler for an upstream with multiple endpoints.
func (r *Router) buildLBHandler(u *store.Upstream) (http.Handler, error) {
	proxies := make(map[string]http.Handler, len(u.Endpoints))
	for _, ep := range u.Endpoints {
		proxy, err := buildUpstreamProxy(u, ep)
		if err != nil {
			return nil, fmt.Errorf("endpoint %s: %w", ep, err)
		}
		proxies[ep] = proxy
	}

	var balancer lb.Balancer
	switch u.LBAlgorithm {
	case "weighted":
		var weps []lb.WeightedEndpoint
		for i, ep := range u.Endpoints {
			w := 100 / len(u.Endpoints)
			if i == 0 {
				w += 100 % len(u.Endpoints)
			}
			weps = append(weps, lb.WeightedEndpoint{URL: ep, Weight: w})
		}
		balancer = lb.NewWeighted(weps)
	case "least-conn":
		balancer = lb.NewLeastConn(u.Endpoints)
	default:
		balancer = lb.NewRoundRobin(u.Endpoints)
	}

	checker := r.checker
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		var isHealthy func(string) bool
		if checker != nil {
			isHealthy = checker.IsHealthy
		}
		ep := balancer.Next(isHealthy)
		if ep == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":"no_healthy_upstream"}`))
			return
		}
		proxy, ok := proxies[ep]
		if !ok {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		if lc, ok := balancer.(*lb.LeastConn); ok {
			defer lc.Done(ep)
		}
		proxy.ServeHTTP(w, req)
	}), nil
}

// buildTrafficSplitHandler creates a weighted random traffic split across multiple upstreams.
func (r *Router) buildTrafficSplitHandler(wus []store.WeightedUpstream, upstreams map[string]http.Handler) http.Handler {
	type wh struct {
		upstreamID string
		handler    http.Handler
		weight     int
	}
	var handlers []wh
	total := 0
	for _, wu := range wus {
		h, ok := upstreams[wu.UpstreamID]
		if !ok {
			slog.Warn("traffic split: upstream not found, skipping", "upstream_id", wu.UpstreamID)
			continue
		}
		w := wu.Weight
		if w <= 0 {
			w = 1
		}
		handlers = append(handlers, wh{upstreamID: wu.UpstreamID, handler: h, weight: w})
		total += w
	}
	if len(handlers) == 0 {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"error":"no_upstream_configured"}`))
		})
	}

	checker := r.checker
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		eligible := handlers
		eligibleTotal := total
		if checker != nil {
			var filtered []wh
			sum := 0
			for _, h := range handlers {
				up := r.st.ListUpstreams()
				upstreamURL := ""
				for _, u := range up {
					if u.ID == h.upstreamID {
						upstreamURL = u.URL
						break
					}
				}
				if upstreamURL == "" || checker.IsHealthy(upstreamURL) {
					filtered = append(filtered, h)
					sum += h.weight
				}
			}
			if len(filtered) > 0 {
				eligible = filtered
				eligibleTotal = sum
			}
		}

		rnd := rand.IntN(eligibleTotal)
		cum := 0
		for _, h := range eligible {
			cum += h.weight
			if rnd < cum {
				h.handler.ServeHTTP(w, req)
				return
			}
		}
		eligible[len(eligible)-1].handler.ServeHTTP(w, req)
	})
}

func (r *Router) buildRouteHandler(route *store.Route, upstream http.Handler) http.Handler {
	var base http.Handler
	if route.StripPath && route.Match.PathMode == store.PathModePrefix {
		prefix := route.Match.Path
		base = http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			trimmed := strings.TrimPrefix(req.URL.Path, prefix)
			if trimmed == "" {
				trimmed = "/"
			}
			req2 := req.Clone(req.Context())
			req2.URL.Path = trimmed
			if req.URL.RawPath != "" {
				req2.URL.RawPath = strings.TrimPrefix(req.URL.RawPath, prefix)
			}
			upstream.ServeHTTP(w, req2)
		})
	} else {
		base = upstream
	}

	if len(route.Plugins) > 0 {
		plugins := make([]plugin.Plugin, 0, len(route.Plugins))
		for _, ref := range route.Plugins {
			p, err := r.reg.Build(ref.Name, ref.Config)
			if err != nil {
				slog.Warn("router: plugin not registered, skipping", "route", route.Name, "plugin", ref.Name)
				continue
			}
			plugins = append(plugins, p)
		}
		if len(plugins) > 0 {
			base = plugin.Chain(base, plugins...)
		}
	}

	if r.tracker == nil {
		return base
	}

	routeID := route.ID
	routeName := route.Name
	inner := base
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		start := time.Now()
		rec := &responseRecorder{ResponseWriter: w, status: http.StatusOK}
		inner.ServeHTTP(rec, req)
		r.tracker.Track(routeID, routeName, rec.status, time.Since(start))
	})
}

func buildUpstreamProxy(u *store.Upstream, targetURL string) (http.Handler, error) {
	target, err := url.Parse(targetURL)
	if err != nil {
		return nil, fmt.Errorf("invalid upstream URL: %w", err)
	}

	rp := httputil.NewSingleHostReverseProxy(target)

	if u.Protocol == "grpc" {
		rp.Transport = &http2.Transport{
			AllowHTTP: true,
			DialTLSContext: func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, network, addr)
			},
		}
	} else {
		rp.Transport = &http.Transport{
			MaxIdleConns:        200,
			MaxIdleConnsPerHost: 50,
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
		}
		rp.ModifyResponse = func(resp *http.Response) error {
			resp.Header.Del("Access-Control-Allow-Origin")
			resp.Header.Del("Access-Control-Allow-Credentials")
			resp.Header.Del("Access-Control-Allow-Methods")
			resp.Header.Del("Access-Control-Allow-Headers")
			resp.Header.Del("Access-Control-Max-Age")
			resp.Header.Del("Vary")
			return nil
		}
	}

	rawURL := targetURL
	rp.Director = func(req *http.Request) {
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.Host = target.Host
		otel.GetTextMapPropagator().Inject(req.Context(), propagation.HeaderCarrier(req.Header))
		req.Header.Del("X-Forwarded-For")
		req.Header.Set("X-Forwarded-For", plugin.ClientIP(req))
		req.Header.Set("X-Forwarded-Proto", forwardedProto(req))
		if claims := plugin.ClaimsFrom(req.Context()); claims != nil {
			req.Header.Set("X-Gateway-User-Id", claims.UserID())
			req.Header.Set("X-Gateway-User-Role", claims.Role)
		}
	}

	rp.ErrorHandler = func(w http.ResponseWriter, req *http.Request, err error) {
		slog.Error("upstream error", "url", rawURL, "path", req.URL.Path, "err", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error":"upstream_unavailable"}`))
	}
	return rp, nil
}

func stripPort(host string) string {
	if i := strings.LastIndex(host, ":"); i != -1 {
		return host[:i]
	}
	return host
}

func forwardedProto(r *http.Request) string {
	if p := r.Header.Get("X-Forwarded-Proto"); p != "" {
		return p
	}
	if r.TLS != nil {
		return "https"
	}
	return "http"
}
