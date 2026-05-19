package store

import (
	"crypto/sha256"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNotFound = errors.New("not found")
	ErrInUse    = errors.New("resource in use")
	ErrConflict = errors.New("name already exists")
)

type Store struct {
	mu         sync.RWMutex
	routes     map[string]*Route
	upstreams  map[string]*Upstream
	policies   map[string]*IPPolicy
	apiKeys    map[string]*APIKey   // id → key
	hashIndex  map[string]*APIKey   // SHA-256 hex → key
	filePath   string
	listeners  []func()
}

func New(filePath string) (*Store, error) {
	s := &Store{
		routes:    make(map[string]*Route),
		upstreams: make(map[string]*Upstream),
		policies:  make(map[string]*IPPolicy),
		apiKeys:   make(map[string]*APIKey),
		hashIndex: make(map[string]*APIKey),
		filePath:  filePath,
	}
	if err := s.load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("store: load %q: %w", filePath, err)
	}
	return s, nil
}

func (s *Store) OnChange(fn func()) {
	s.mu.Lock()
	s.listeners = append(s.listeners, fn)
	s.mu.Unlock()
}

// ── Routes ────────────────────────────────────────────────────────────────────

func (s *Store) ListRoutes() []*Route {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Route, 0, len(s.routes))
	for _, r := range s.routes {
		cp := *r
		out = append(out, &cp)
	}
	return out
}

func (s *Store) GetRoute(id string) (*Route, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.routes[id]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *r
	return &cp, nil
}

func (s *Store) CreateRoute(r *Route) (*Route, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, existing := range s.routes {
		if existing.Name == r.Name {
			return nil, ErrConflict
		}
	}
	if r.UpstreamID != "" {
		if _, ok := s.upstreams[r.UpstreamID]; !ok {
			return nil, fmt.Errorf("upstream %q: %w", r.UpstreamID, ErrNotFound)
		}
	}

	r.ID = uuid.NewString()
	r.CreatedAt = time.Now().UTC()
	r.UpdatedAt = r.CreatedAt
	cp := *r
	s.routes[r.ID] = &cp

	if err := s.persist(); err != nil {
		delete(s.routes, r.ID)
		return nil, err
	}
	s.notify()
	return &cp, nil
}

func (s *Store) UpdateRoute(id string, patch *Route) (*Route, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.routes[id]
	if !ok {
		return nil, ErrNotFound
	}
	if patch.UpstreamID != "" {
		if _, ok := s.upstreams[patch.UpstreamID]; !ok {
			return nil, fmt.Errorf("upstream %q: %w", patch.UpstreamID, ErrNotFound)
		}
	}
	for rid, r := range s.routes {
		if r.Name == patch.Name && rid != id {
			return nil, ErrConflict
		}
	}

	patch.ID = existing.ID
	patch.CreatedAt = existing.CreatedAt
	patch.UpdatedAt = time.Now().UTC()
	cp := *patch
	s.routes[id] = &cp

	if err := s.persist(); err != nil {
		s.routes[id] = existing
		return nil, err
	}
	s.notify()
	return &cp, nil
}

func (s *Store) DeleteRoute(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.routes[id]; !ok {
		return ErrNotFound
	}
	backup := s.routes[id]
	delete(s.routes, id)
	if err := s.persist(); err != nil {
		s.routes[id] = backup
		return err
	}
	s.notify()
	return nil
}

func (s *Store) ToggleRoute(id string, active bool) (*Route, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.routes[id]
	if !ok {
		return nil, ErrNotFound
	}
	r.Active = active
	r.UpdatedAt = time.Now().UTC()
	if err := s.persist(); err != nil {
		return nil, err
	}
	s.notify()
	cp := *r
	return &cp, nil
}

// ── Upstreams ─────────────────────────────────────────────────────────────────

func (s *Store) ListUpstreams() []*Upstream {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Upstream, 0, len(s.upstreams))
	for _, u := range s.upstreams {
		cp := *u
		out = append(out, &cp)
	}
	return out
}

func (s *Store) GetUpstream(id string) (*Upstream, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.upstreams[id]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *u
	return &cp, nil
}

func (s *Store) CreateUpstream(u *Upstream) (*Upstream, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.upstreams {
		if existing.Name == u.Name {
			return nil, ErrConflict
		}
	}
	u.ID = uuid.NewString()
	u.CreatedAt = time.Now().UTC()
	u.UpdatedAt = u.CreatedAt
	cp := *u
	s.upstreams[u.ID] = &cp
	if err := s.persist(); err != nil {
		delete(s.upstreams, u.ID)
		return nil, err
	}
	s.notify()
	return &cp, nil
}

func (s *Store) UpdateUpstream(id string, patch *Upstream) (*Upstream, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	existing, ok := s.upstreams[id]
	if !ok {
		return nil, ErrNotFound
	}
	for uid, u := range s.upstreams {
		if u.Name == patch.Name && uid != id {
			return nil, ErrConflict
		}
	}
	patch.ID = existing.ID
	patch.CreatedAt = existing.CreatedAt
	patch.UpdatedAt = time.Now().UTC()
	cp := *patch
	s.upstreams[id] = &cp
	if err := s.persist(); err != nil {
		s.upstreams[id] = existing
		return nil, err
	}
	s.notify()
	return &cp, nil
}

func (s *Store) DeleteUpstream(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.upstreams[id]; !ok {
		return ErrNotFound
	}
	for _, r := range s.routes {
		if r.UpstreamID == id {
			return ErrInUse
		}
	}
	backup := s.upstreams[id]
	delete(s.upstreams, id)
	if err := s.persist(); err != nil {
		s.upstreams[id] = backup
		return err
	}
	s.notify()
	return nil
}

// ── IP Policies ───────────────────────────────────────────────────────────────

func (s *Store) ListIPPolicies() []*IPPolicy {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*IPPolicy, 0, len(s.policies))
	for _, p := range s.policies {
		cp := *p
		out = append(out, &cp)
	}
	return out
}

func (s *Store) GetIPPolicy(id string) (*IPPolicy, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.policies[id]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *p
	return &cp, nil
}

func (s *Store) CreateIPPolicy(p *IPPolicy) (*IPPolicy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.policies {
		if existing.Name == p.Name {
			return nil, ErrConflict
		}
	}
	p.ID = uuid.NewString()
	p.CreatedAt = time.Now().UTC()
	p.UpdatedAt = p.CreatedAt
	cp := *p
	s.policies[p.ID] = &cp
	if err := s.persist(); err != nil {
		delete(s.policies, p.ID)
		return nil, err
	}
	s.notify()
	return &cp, nil
}

func (s *Store) UpdateIPPolicy(id string, patch *IPPolicy) (*IPPolicy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	existing, ok := s.policies[id]
	if !ok {
		return nil, ErrNotFound
	}
	for pid, p := range s.policies {
		if p.Name == patch.Name && pid != id {
			return nil, ErrConflict
		}
	}
	patch.ID = existing.ID
	patch.CreatedAt = existing.CreatedAt
	patch.UpdatedAt = time.Now().UTC()
	cp := *patch
	s.policies[id] = &cp
	if err := s.persist(); err != nil {
		s.policies[id] = existing
		return nil, err
	}
	s.notify()
	return &cp, nil
}

func (s *Store) DeleteIPPolicy(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.policies[id]; !ok {
		return ErrNotFound
	}
	backup := s.policies[id]
	delete(s.policies, id)
	if err := s.persist(); err != nil {
		s.policies[id] = backup
		return err
	}
	s.notify()
	return nil
}

func (s *Store) ToggleIPPolicy(id string, active bool) (*IPPolicy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.policies[id]
	if !ok {
		return nil, ErrNotFound
	}
	p.Active = active
	p.UpdatedAt = time.Now().UTC()
	if err := s.persist(); err != nil {
		return nil, err
	}
	s.notify()
	cp := *p
	return &cp, nil
}

// ── API Keys ──────────────────────────────────────────────────────────────────

// GenerateAPIKey creates a cryptographically random key, returns (plaintext, hash).
func GenerateAPIKey() (plaintext, hash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return
	}
	plaintext = base64.RawURLEncoding.EncodeToString(b)
	h := sha256.Sum256([]byte(plaintext))
	hash = hex.EncodeToString(h[:])
	return
}

func hashKey(plaintext string) string {
	h := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(h[:])
}

func (s *Store) ListAPIKeys() []*APIKey {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*APIKey, 0, len(s.apiKeys))
	for _, k := range s.apiKeys {
		cp := *k
		cp.KeyHash = "" // never expose hash
		out = append(out, &cp)
	}
	return out
}

func (s *Store) GetAPIKey(id string) (*APIKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	k, ok := s.apiKeys[id]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *k
	cp.KeyHash = ""
	return &cp, nil
}

// CreateAPIKey stores a new API key. The plaintext must be hashed by the caller;
// this function expects the hash. Returns the stored key (without hash).
func (s *Store) CreateAPIKey(k *APIKey) (*APIKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.apiKeys {
		if existing.Name == k.Name {
			return nil, ErrConflict
		}
	}
	if k.KeyHash == "" {
		return nil, fmt.Errorf("key hash is required")
	}
	k.ID = uuid.NewString()
	k.CreatedAt = time.Now().UTC()
	k.UpdatedAt = k.CreatedAt
	cp := *k
	s.apiKeys[k.ID] = &cp
	s.hashIndex[k.KeyHash] = &cp
	if err := s.persist(); err != nil {
		delete(s.apiKeys, k.ID)
		delete(s.hashIndex, k.KeyHash)
		return nil, err
	}
	cp.KeyHash = ""
	return &cp, nil
}

func (s *Store) DeleteAPIKey(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	k, ok := s.apiKeys[id]
	if !ok {
		return ErrNotFound
	}
	delete(s.hashIndex, k.KeyHash)
	delete(s.apiKeys, id)
	if err := s.persist(); err != nil {
		s.apiKeys[id] = k
		s.hashIndex[k.KeyHash] = k
		return err
	}
	return nil
}

func (s *Store) ToggleAPIKey(id string, active bool) (*APIKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	k, ok := s.apiKeys[id]
	if !ok {
		return nil, ErrNotFound
	}
	k.Active = active
	k.UpdatedAt = time.Now().UTC()
	if err := s.persist(); err != nil {
		return nil, err
	}
	cp := *k
	cp.KeyHash = ""
	return &cp, nil
}

// ValidateAPIKey checks whether a plaintext key is active and returns the key metadata.
func (s *Store) ValidateAPIKey(plaintext string) (*APIKey, bool) {
	h := hashKey(plaintext)
	s.mu.RLock()
	k, ok := s.hashIndex[h]
	s.mu.RUnlock()
	if !ok || !k.Active {
		return nil, false
	}
	cp := *k
	cp.KeyHash = ""
	return &cp, true
}

// ── Persistence ───────────────────────────────────────────────────────────────

func (s *Store) Export() snapshot {
	routes := make([]*Route, 0, len(s.routes))
	for _, r := range s.routes {
		cp := *r
		routes = append(routes, &cp)
	}
	upstreams := make([]*Upstream, 0, len(s.upstreams))
	for _, u := range s.upstreams {
		cp := *u
		upstreams = append(upstreams, &cp)
	}
	policies := make([]*IPPolicy, 0, len(s.policies))
	for _, p := range s.policies {
		cp := *p
		policies = append(policies, &cp)
	}
	apiKeys := make([]*APIKey, 0, len(s.apiKeys))
	for _, k := range s.apiKeys {
		cp := *k
		apiKeys = append(apiKeys, &cp)
	}
	return snapshot{Version: 1, Routes: routes, Upstreams: upstreams, IPPolicies: policies, APIKeys: apiKeys}
}

func (s *Store) persist() error {
	if s.filePath == "" {
		return nil
	}
	snap := s.Export()
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	tmp := s.filePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return os.Rename(tmp, s.filePath)
}

func (s *Store) load() error {
	if s.filePath == "" {
		return nil
	}
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return err
	}
	var snap snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}
	for _, r := range snap.Routes {
		s.routes[r.ID] = r
	}
	for _, u := range snap.Upstreams {
		s.upstreams[u.ID] = u
	}
	for _, p := range snap.IPPolicies {
		s.policies[p.ID] = p
	}
	for _, k := range snap.APIKeys {
		s.apiKeys[k.ID] = k
		if k.KeyHash != "" {
			s.hashIndex[k.KeyHash] = k
		}
	}
	return nil
}

func (s *Store) notify() {
	for _, fn := range s.listeners {
		go fn()
	}
}
