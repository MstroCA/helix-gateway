package lb

import (
	"math/rand/v2"
	"sync"
	"sync/atomic"
)

// Balancer selects an upstream endpoint URL on each call.
// isHealthy is optional; when nil every endpoint is considered healthy.
type Balancer interface {
	Next(isHealthy func(url string) bool) string
}

// WeightedEndpoint is an endpoint with a relative traffic weight.
type WeightedEndpoint struct {
	URL    string
	Weight int
}

// NewRoundRobin creates a lock-free round-robin balancer.
func NewRoundRobin(endpoints []string) Balancer {
	cp := make([]string, len(endpoints))
	copy(cp, endpoints)
	return &roundRobin{endpoints: cp}
}

type roundRobin struct {
	endpoints []string
	counter   atomic.Uint64
}

func (rr *roundRobin) Next(isHealthy func(string) bool) string {
	n := uint64(len(rr.endpoints))
	for range n {
		idx := rr.counter.Add(1) % n
		ep := rr.endpoints[idx]
		if isHealthy == nil || isHealthy(ep) {
			return ep
		}
	}
	return rr.endpoints[rr.counter.Load()%n]
}

// NewWeighted creates a weighted random balancer.
func NewWeighted(endpoints []WeightedEndpoint) Balancer {
	total := 0
	for _, e := range endpoints {
		if e.Weight > 0 {
			total += e.Weight
		}
	}
	cp := make([]WeightedEndpoint, len(endpoints))
	copy(cp, endpoints)
	return &weightedBalancer{endpoints: cp, total: total}
}

type weightedBalancer struct {
	endpoints []WeightedEndpoint
	total     int
}

func (wb *weightedBalancer) Next(isHealthy func(string) bool) string {
	eligible := wb.endpoints
	total := wb.total
	if isHealthy != nil {
		var filtered []WeightedEndpoint
		sum := 0
		for _, e := range wb.endpoints {
			if isHealthy(e.URL) {
				filtered = append(filtered, e)
				sum += e.Weight
			}
		}
		if len(filtered) > 0 {
			eligible = filtered
			total = sum
		}
	}
	if total <= 0 {
		return ""
	}
	r := rand.IntN(total)
	cum := 0
	for _, e := range eligible {
		cum += e.Weight
		if r < cum {
			return e.URL
		}
	}
	return eligible[len(eligible)-1].URL
}

// NewLeastConn creates a least-active-connections balancer.
// Callers must call Done(url) when a request to that endpoint completes.
func NewLeastConn(endpoints []string) *LeastConn {
	lc := &LeastConn{conns: make(map[string]*atomic.Int64, len(endpoints))}
	for _, ep := range endpoints {
		ep := ep
		lc.endpoints = append(lc.endpoints, ep)
		c := &atomic.Int64{}
		lc.conns[ep] = c
	}
	return lc
}

type LeastConn struct {
	mu        sync.Mutex
	endpoints []string
	conns     map[string]*atomic.Int64
}

func (lc *LeastConn) Next(isHealthy func(string) bool) string {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	var best string
	var bestCount int64 = -1
	for _, ep := range lc.endpoints {
		if isHealthy != nil && !isHealthy(ep) {
			continue
		}
		c := lc.conns[ep].Load()
		if bestCount < 0 || c < bestCount {
			best = ep
			bestCount = c
		}
	}
	if best == "" && len(lc.endpoints) > 0 {
		best = lc.endpoints[0]
	}
	if best != "" {
		lc.conns[best].Add(1)
	}
	return best
}

func (lc *LeastConn) Done(url string) {
	if c, ok := lc.conns[url]; ok {
		c.Add(-1)
	}
}
