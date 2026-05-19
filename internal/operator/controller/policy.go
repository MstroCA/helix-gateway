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

const policyFinalizer = "helix.io/policy"

type HelixIPPolicyReconciler struct {
	client.Client
	Admin *adminclient.Client
}

func (r *HelixIPPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var cr helixv1.HelixIPPolicy
	if err := r.Get(ctx, req.NamespacedName, &cr); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !cr.DeletionTimestamp.IsZero() {
		return r.handleDelete(ctx, &cr)
	}

	if !controllerutil.ContainsFinalizer(&cr, policyFinalizer) {
		controllerutil.AddFinalizer(&cr, policyFinalizer)
		if err := r.Update(ctx, &cr); err != nil {
			return ctrl.Result{}, err
		}
	}

	return r.handleUpsert(ctx, &cr)
}

func (r *HelixIPPolicyReconciler) handleUpsert(ctx context.Context, cr *helixv1.HelixIPPolicy) (ctrl.Result, error) {
	p := &helixstore.IPPolicy{
		Name:   cr.Name,
		Mode:   cr.Spec.Mode,
		CIDRs:  cr.Spec.CIDRs,
		Active: cr.Spec.Active,
	}

	var (
		result *helixstore.IPPolicy
		err    error
	)
	if cr.Status.PolicyID == "" {
		result, err = r.Admin.CreateIPPolicy(p)
	} else {
		p.ID = cr.Status.PolicyID
		result, err = r.Admin.UpdateIPPolicy(cr.Status.PolicyID, p)
	}
	if err != nil {
		return r.setCondition(ctx, cr, "Ready", metav1.ConditionFalse, "SyncFailed", err.Error())
	}

	cr.Status.PolicyID = result.ID
	return r.setCondition(ctx, cr, "Ready", metav1.ConditionTrue, "Synced", fmt.Sprintf("policy %s synced", result.ID))
}

func (r *HelixIPPolicyReconciler) handleDelete(ctx context.Context, cr *helixv1.HelixIPPolicy) (ctrl.Result, error) {
	if controllerutil.ContainsFinalizer(cr, policyFinalizer) {
		if cr.Status.PolicyID != "" {
			if err := r.Admin.DeleteIPPolicy(cr.Status.PolicyID); err != nil {
				return ctrl.Result{}, err
			}
		}
		controllerutil.RemoveFinalizer(cr, policyFinalizer)
		if err := r.Update(ctx, cr); err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

func (r *HelixIPPolicyReconciler) setCondition(
	ctx context.Context,
	cr *helixv1.HelixIPPolicy,
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

func (r *HelixIPPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&helixv1.HelixIPPolicy{}).
		Complete(r)
}
