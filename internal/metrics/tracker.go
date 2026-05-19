package metrics

import (
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

type RouteSnapshot struct {
	RouteID      string  `json:"route_id"`
	RouteName    string  `json:"route_name"`
	Requests     int64   `json:"requests"`
	Errors       int64   `json:"errors"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
	ErrorRate    float64 `json:"error_rate"`
	P99LatencyMs int64   `json:"p99_latency_ms"`
}

type Snapshot struct {
	Routes    []RouteSnapshot `json:"routes"`
	TotalReqs int64           `json:"total_requests"`
	TotalErrs int64           `json:"total_errors"`
}

type counter struct {
	name      string
	requests  atomic.Int64
	errors    atomic.Int64
	latSum    atomic.Int64
	mu        sync.Mutex
	latWindow []int64
}

const windowSize = 1000

func (c *counter) recordLatency(ms int64) {
	c.mu.Lock()
	if len(c.latWindow) >= windowSize {
		c.latWindow = c.latWindow[1:]
	}
	c.latWindow = append(c.latWindow, ms)
	c.mu.Unlock()
}

func (c *counter) p99() int64 {
	c.mu.Lock()
	if len(c.latWindow) == 0 {
		c.mu.Unlock()
		return 0
	}
	cp := make([]int64, len(c.latWindow))
	copy(cp, c.latWindow)
	c.mu.Unlock()

	sort.Slice(cp, func(i, j int) bool { return cp[i] < cp[j] })
	idx := int(float64(len(cp)) * 0.99)
	if idx >= len(cp) {
		idx = len(cp) - 1
	}
	return cp[idx]
}

type Tracker struct {
	mu     sync.RWMutex
	routes map[string]*counter
}

func NewTracker() *Tracker {
	return &Tracker{routes: make(map[string]*counter)}
}

func (t *Tracker) Track(routeID, routeName string, statusCode int, latency time.Duration) {
	c := t.getOrCreate(routeID, routeName)
	c.requests.Add(1)
	ms := latency.Milliseconds()
	c.latSum.Add(ms)
	c.recordLatency(ms)
	if statusCode >= 400 {
		c.errors.Add(1)
	}
}

func (t *Tracker) Snapshot() Snapshot {
	t.mu.RLock()
	defer t.mu.RUnlock()

	routes := make([]RouteSnapshot, 0, len(t.routes))
	var totalReqs, totalErrs int64

	for id, c := range t.routes {
		reqs := c.requests.Load()
		errs := c.errors.Load()
		latSum := c.latSum.Load()
		totalReqs += reqs
		totalErrs += errs

		var avgLat, errRate float64
		if reqs > 0 {
			avgLat = float64(latSum) / float64(reqs)
			errRate = float64(errs) / float64(reqs) * 100
		}
		routes = append(routes, RouteSnapshot{
			RouteID:      id,
			RouteName:    c.name,
			Requests:     reqs,
			Errors:       errs,
			AvgLatencyMs: avgLat,
			ErrorRate:    errRate,
			P99LatencyMs: c.p99(),
		})
	}

	sort.Slice(routes, func(i, j int) bool { return routes[i].RouteName < routes[j].RouteName })
	return Snapshot{Routes: routes, TotalReqs: totalReqs, TotalErrs: totalErrs}
}

func (t *Tracker) getOrCreate(routeID, routeName string) *counter {
	t.mu.RLock()
	if c, ok := t.routes[routeID]; ok {
		t.mu.RUnlock()
		return c
	}
	t.mu.RUnlock()

	t.mu.Lock()
	defer t.mu.Unlock()
	if c, ok := t.routes[routeID]; ok {
		return c
	}
	c := &counter{name: routeName}
	t.routes[routeID] = c
	return c
}
