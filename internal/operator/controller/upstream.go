package controller

import (
	"context"
	"fmt"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	adminclient "helix/internal/admin/client"
	helixstore "helix/internal/store"
	helixv1 "helix/internal/operator/types"
)

const upstreamFinalizer = "helix.io/upstream"

type GatewayUpstreamReconciler struct {
	client.Client
	Admin *adminclient.Client
}

func (r *GatewayUpstreamReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var cr helixv1.GatewayUpstream
	if err := r.Get(ctx, req.NamespacedName, &cr); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !cr.DeletionTimestamp.IsZero() {
		return r.handleDelete(ctx, &cr)
	}

	if !controllerutil.ContainsFinalizer(&cr, upstreamFinalizer) {
		controllerutil.AddFinalizer(&cr, upstreamFinalizer)
		if err := r.Update(ctx, &cr); err != nil {
			return ctrl.Result{}, err
		}
	}

	return r.handleUpsert(ctx, &cr)
}

func (r *GatewayUpstreamReconciler) handleUpsert(ctx context.Context, cr *helixv1.GatewayUpstream) (ctrl.Result, error) {
	u := &helixstore.Upstream{
		Name:       cr.Name,
		URL:        cr.Spec.URL,
		HealthPath: cr.Spec.HealthPath,
	}

	var (
		result *helixstore.Upstream
		err    error
	)
	if cr.Status.UpstreamID == "" {
		result, err = r.Admin.CreateUpstream(u)
	} else {
		u.ID = cr.Status.UpstreamID
		result, err = r.Admin.UpdateUpstream(cr.Status.UpstreamID, u)
	}
	if err != nil {
		return r.setCondition(ctx, cr, "Ready", metav1.ConditionFalse, "SyncFailed", err.Error())
	}

	cr.Status.UpstreamID = result.ID
	return r.setCondition(ctx, cr, "Ready", metav1.ConditionTrue, "Synced", fmt.Sprintf("upstream %s synced", result.ID))
}

func (r *GatewayUpstreamReconciler) handleDelete(ctx context.Context, cr *helixv1.GatewayUpstream) (ctrl.Result, error) {
	if controllerutil.ContainsFinalizer(cr, upstreamFinalizer) {
		if cr.Status.UpstreamID != "" {
			if err := r.Admin.DeleteUpstream(cr.Status.UpstreamID); err != nil {
				return ctrl.Result{}, err
			}
		}
		controllerutil.RemoveFinalizer(cr, upstreamFinalizer)
		if err := r.Update(ctx, cr); err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

func (r *GatewayUpstreamReconciler) setCondition(
	ctx context.Context,
	cr *helixv1.GatewayUpstream,
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

func (r *GatewayUpstreamReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&helixv1.GatewayUpstream{}).
		Complete(r)
}
