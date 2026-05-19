package controller

import (
	"context"
	"fmt"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	adminclient "helix/internal/admin/client"
	helixstore "helix/internal/store"
	helixv1 "helix/internal/operator/types"
)

const routeFinalizer = "helix.io/route"

type GatewayRouteReconciler struct {
	client.Client
	Admin *adminclient.Client
}

func (r *GatewayRouteReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var cr helixv1.GatewayRoute
	if err := r.Get(ctx, req.NamespacedName, &cr); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !cr.DeletionTimestamp.IsZero() {
		return r.handleDelete(ctx, &cr)
	}

	if !controllerutil.ContainsFinalizer(&cr, routeFinalizer) {
		controllerutil.AddFinalizer(&cr, routeFinalizer)
		if err := r.Update(ctx, &cr); err != nil {
			return ctrl.Result{}, err
		}
	}

	return r.handleUpsert(ctx, &cr)
}

func (r *GatewayRouteReconciler) handleUpsert(ctx context.Context, cr *helixv1.GatewayRoute) (ctrl.Result, error) {
	upstreamID, err := r.resolveUpstreamID(ctx, cr)
	if err != nil {
		return r.setCondition(ctx, cr, "Ready", metav1.ConditionFalse, "UpstreamNotReady", err.Error())
	}

	route := &helixstore.Route{
		Name:       cr.Name,
		Active:     cr.Spec.Active,
		UpstreamID: upstreamID,
		StripPath:  cr.Spec.StripPath,
		Match: helixstore.MatchRule{
			Hosts:    cr.Spec.Match.Hosts,
			Path:     cr.Spec.Match.Path,
			PathMode: helixstore.PathMode(cr.Spec.Match.PathMode),
			Methods:  cr.Spec.Match.Methods,
			Headers:  cr.Spec.Match.Headers,
		},
	}
	for _, p := range cr.Spec.Plugins {
		route.Plugins = append(route.Plugins, helixstore.PluginRef{
			Name:   p.Name,
			Config: p.Config,
		})
	}
	if route.Match.PathMode == "" {
		route.Match.PathMode = helixstore.PathModePrefix
	}

	var result *helixstore.Route
	if cr.Status.RouteID == "" {
		result, err = r.Admin.CreateRoute(route)
	} else {
		route.ID = cr.Status.RouteID
		result, err = r.Admin.UpdateRoute(cr.Status.RouteID, route)
	}
	if err != nil {
		return r.setCondition(ctx, cr, "Ready", metav1.ConditionFalse, "SyncFailed", err.Error())
	}

	cr.Status.RouteID = result.ID
	return r.setCondition(ctx, cr, "Ready", metav1.ConditionTrue, "Synced", fmt.Sprintf("route %s synced", result.ID))
}

func (r *GatewayRouteReconciler) resolveUpstreamID(ctx context.Context, cr *helixv1.GatewayRoute) (string, error) {
	var upstream helixv1.GatewayUpstream
	key := types.NamespacedName{Namespace: cr.Namespace, Name: cr.Spec.UpstreamRef}
	if err := r.Get(ctx, key, &upstream); err != nil {
		return "", fmt.Errorf("upstream %q not found: %w", cr.Spec.UpstreamRef, err)
	}
	if upstream.Status.UpstreamID == "" {
		return "", fmt.Errorf("upstream %q not yet synced — waiting for status.upstreamId", cr.Spec.UpstreamRef)
	}
	return upstream.Status.UpstreamID, nil
}

func (r *GatewayRouteReconciler) handleDelete(ctx context.Context, cr *helixv1.GatewayRoute) (ctrl.Result, error) {
	if controllerutil.ContainsFinalizer(cr, routeFinalizer) {
		if cr.Status.RouteID != "" {
			if err := r.Admin.DeleteRoute(cr.Status.RouteID); err != nil {
				return ctrl.Result{}, err
			}
		}
		controllerutil.RemoveFinalizer(cr, routeFinalizer)
		if err := r.Update(ctx, cr); err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

func (r *GatewayRouteReconciler) setCondition(
	ctx context.Context,
	cr *helixv1.GatewayRoute,
	condType string,
	status metav1.ConditionStatus,
	reason, msg string,
) (ctrl.Result, error) {
	apimeta.SetStatusCondition(&cr.Status.Conditions, metav1.Condition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            msg,
		ObservedGeneration: cr.Generation,
	})
	if err := r.Status().Update(ctx, cr); err != nil {
		return ctrl.Result{}, err
	}
	if status == metav1.ConditionFalse {
		return ctrl.Result{}, fmt.Errorf("%s", msg)
	}
	return ctrl.Result{}, nil
}

func (r *GatewayRouteReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&helixv1.GatewayRoute{}).
		Complete(r)
}
