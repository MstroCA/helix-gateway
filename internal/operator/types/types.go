package types

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	SchemeGroupVersion = schema.GroupVersion{Group: "helix.io", Version: "v1alpha1"}
	SchemeBuilder      = runtime.NewSchemeBuilder(addKnownTypes)
	AddToScheme        = SchemeBuilder.AddToScheme
)

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&GatewayRoute{},
		&GatewayRouteList{},
		&GatewayUpstream{},
		&GatewayUpstreamList{},
		&HelixIPPolicy{},
		&HelixIPPolicyList{},
	)
	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
}

// ── GatewayRoute ──────────────────────────────────────────────────────────────

type GatewayRoute struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              GatewayRouteSpec   `json:"spec"`
	Status            GatewayRouteStatus `json:"status,omitempty"`
}

type GatewayRouteSpec struct {
	Active      bool        `json:"active"`
	Match       MatchRule   `json:"match"`
	UpstreamRef string      `json:"upstreamRef"`
	Plugins     []PluginRef `json:"plugins,omitempty"`
	StripPath   bool        `json:"stripPath,omitempty"`
}

type MatchRule struct {
	Hosts    []string          `json:"hosts,omitempty"`
	Path     string            `json:"path"`
	PathMode string            `json:"pathMode,omitempty"`
	Methods  []string          `json:"methods,omitempty"`
	Headers  map[string]string `json:"headers,omitempty"`
}

type PluginRef struct {
	Name   string         `json:"name"`
	Config map[string]any `json:"config,omitempty"`
}

type GatewayRouteStatus struct {
	RouteID    string             `json:"routeId,omitempty"`
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

type GatewayRouteList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GatewayRoute `json:"items"`
}

func (in *GatewayRoute) DeepCopyObject() runtime.Object {
	out := &GatewayRoute{}
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = deepCopyRouteSpec(in.Spec)
	out.Status = GatewayRouteStatus{
		RouteID:    in.Status.RouteID,
		Conditions: deepCopyConditions(in.Status.Conditions),
	}
	return out
}

func (in *GatewayRouteList) DeepCopyObject() runtime.Object {
	out := &GatewayRouteList{}
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	out.Items = make([]GatewayRoute, len(in.Items))
	for i := range in.Items {
		out.Items[i] = *in.Items[i].DeepCopyObject().(*GatewayRoute)
	}
	return out
}

func deepCopyRouteSpec(in GatewayRouteSpec) GatewayRouteSpec {
	out := in
	out.Match = deepCopyMatchRule(in.Match)
	if in.Plugins != nil {
		out.Plugins = make([]PluginRef, len(in.Plugins))
		for i, p := range in.Plugins {
			out.Plugins[i] = PluginRef{Name: p.Name, Config: deepCopyAnyMap(p.Config)}
		}
	}
	return out
}

func deepCopyMatchRule(in MatchRule) MatchRule {
	out := in
	if in.Hosts != nil {
		out.Hosts = make([]string, len(in.Hosts))
		copy(out.Hosts, in.Hosts)
	}
	if in.Methods != nil {
		out.Methods = make([]string, len(in.Methods))
		copy(out.Methods, in.Methods)
	}
	if in.Headers != nil {
		out.Headers = make(map[string]string, len(in.Headers))
		for k, v := range in.Headers {
			out.Headers[k] = v
		}
	}
	return out
}

// ── GatewayUpstream ───────────────────────────────────────────────────────────

type GatewayUpstream struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              GatewayUpstreamSpec   `json:"spec"`
	Status            GatewayUpstreamStatus `json:"status,omitempty"`
}

type GatewayUpstreamSpec struct {
	URL        string `json:"url"`
	HealthPath string `json:"healthPath,omitempty"`
}

type GatewayUpstreamStatus struct {
	UpstreamID string             `json:"upstreamId,omitempty"`
	Health     string             `json:"health,omitempty"`
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

type GatewayUpstreamList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GatewayUpstream `json:"items"`
}

func (in *GatewayUpstream) DeepCopyObject() runtime.Object {
	out := &GatewayUpstream{}
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	out.Status = GatewayUpstreamStatus{
		UpstreamID: in.Status.UpstreamID,
		Health:     in.Status.Health,
		Conditions: deepCopyConditions(in.Status.Conditions),
	}
	return out
}

func (in *GatewayUpstreamList) DeepCopyObject() runtime.Object {
	out := &GatewayUpstreamList{}
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	out.Items = make([]GatewayUpstream, len(in.Items))
	for i := range in.Items {
		out.Items[i] = *in.Items[i].DeepCopyObject().(*GatewayUpstream)
	}
	return out
}

// ── HelixIPPolicy ─────────────────────────────────────────────────────────────

type HelixIPPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              HelixIPPolicySpec   `json:"spec"`
	Status            HelixIPPolicyStatus `json:"status,omitempty"`
}

type HelixIPPolicySpec struct {
	Mode   string   `json:"mode"`
	CIDRs  []string `json:"cidrs"`
	Active bool     `json:"active"`
}

type HelixIPPolicyStatus struct {
	PolicyID   string             `json:"policyId,omitempty"`
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

type HelixIPPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HelixIPPolicy `json:"items"`
}

func (in *HelixIPPolicy) DeepCopyObject() runtime.Object {
	out := &HelixIPPolicy{}
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = HelixIPPolicySpec{
		Mode:   in.Spec.Mode,
		Active: in.Spec.Active,
	}
	if in.Spec.CIDRs != nil {
		out.Spec.CIDRs = make([]string, len(in.Spec.CIDRs))
		copy(out.Spec.CIDRs, in.Spec.CIDRs)
	}
	out.Status = HelixIPPolicyStatus{
		PolicyID:   in.Status.PolicyID,
		Conditions: deepCopyConditions(in.Status.Conditions),
	}
	return out
}

func (in *HelixIPPolicyList) DeepCopyObject() runtime.Object {
	out := &HelixIPPolicyList{}
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	out.Items = make([]HelixIPPolicy, len(in.Items))
	for i := range in.Items {
		out.Items[i] = *in.Items[i].DeepCopyObject().(*HelixIPPolicy)
	}
	return out
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func deepCopyConditions(in []metav1.Condition) []metav1.Condition {
	if in == nil {
		return nil
	}
	out := make([]metav1.Condition, len(in))
	copy(out, in)
	return out
}

func deepCopyAnyMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = deepCopyAnyValue(v)
	}
	return out
}

func deepCopyAnyValue(v any) any {
	switch val := v.(type) {
	case map[string]any:
		return deepCopyAnyMap(val)
	case []any:
		out := make([]any, len(val))
		for i, item := range val {
			out[i] = deepCopyAnyValue(item)
		}
		return out
	default:
		return v
	}
}
