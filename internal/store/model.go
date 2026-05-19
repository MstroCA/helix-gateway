package store

import "time"

type PathMode string

const (
	PathModePrefix PathMode = "prefix"
	PathModeExact  PathMode = "exact"
	PathModeRegex  PathMode = "regex"
)

type Route struct {
	ID         string             `json:"id"`
	Name       string             `json:"name"`
	Active     bool               `json:"active"`
	Match      MatchRule          `json:"match"`
	UpstreamID string             `json:"upstreamId"`
	Plugins    []PluginRef        `json:"plugins"`
	StripPath  bool               `json:"stripPath"`
	// WeightedUpstreams enables traffic splitting. When set, UpstreamID is ignored.
	WeightedUpstreams []WeightedUpstream `json:"weightedUpstreams,omitempty"`
	CreatedAt  time.Time          `json:"createdAt"`
	UpdatedAt  time.Time          `json:"updatedAt"`
}

type WeightedUpstream struct {
	UpstreamID string `json:"upstreamId"`
	Weight     int    `json:"weight"`
}

type MatchRule struct {
	Hosts    []string          `json:"hosts"`
	Path     string            `json:"path"`
	PathMode PathMode          `json:"pathMode"`
	Methods  []string          `json:"methods"`
	Headers  map[string]string `json:"headers,omitempty"`
}

type Upstream struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	URL         string    `json:"url"`
	Protocol    string    `json:"protocol,omitempty"`   // "" | "grpc"
	Endpoints   []string  `json:"endpoints,omitempty"`  // for load balancing
	LBAlgorithm string    `json:"lbAlgorithm,omitempty"` // "round-robin" | "weighted" | "least-conn"
	HealthPath  string    `json:"healthPath"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type PluginRef struct {
	Name   string         `json:"name"`
	Config map[string]any `json:"config,omitempty"`
}

type IPPolicy struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Mode      string    `json:"mode"` // "allow" | "deny"
	CIDRs     []string  `json:"cidrs"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// APIKey holds a hashed API key credential.
// The KeyHash field is excluded from API responses; only the plaintext key is
// returned once at creation time and never stored.
type APIKey struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	KeyHash   string    `json:"keyHash,omitempty"` // SHA-256 hex; omitted from list/get responses
	Active    bool      `json:"active"`
	Scopes    []string  `json:"scopes,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type snapshot struct {
	Version    int         `json:"version"`
	Routes     []*Route    `json:"routes"`
	Upstreams  []*Upstream `json:"upstreams"`
	IPPolicies []*IPPolicy `json:"ipPolicies"`
	APIKeys    []*APIKey   `json:"apiKeys,omitempty"`
}
