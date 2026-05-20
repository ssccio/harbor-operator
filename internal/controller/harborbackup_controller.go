/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package controller contains the Harbor and HarborBackup reconcilers.
//
// HarborBackupReconciler is a Level-3 stub. The CRD and RBAC are registered
// so the operator owns the type from day one, but the actual backup logic
// (snapshot PVC, dump Postgres, ship to S3) is not yet implemented.
// A Degraded condition with reason=NotImplemented is set on every CR so that
// users know the feature is pending rather than silently doing nothing.
package controller

import (
	"context"
	"fmt"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	registryv1alpha1 "github.com/ken/harbor-operator/api/v1alpha1"
)

// HarborBackupReconciler reconciles a HarborBackup object.
type HarborBackupReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=registry.gocadmium.dev,resources=harborbackups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=registry.gocadmium.dev,resources=harborbackups/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=registry.gocadmium.dev,resources=harborbackups/finalizers,verbs=update

// Reconcile sets a Degraded/NotImplemented condition and emits a warning event.
// Replace this entire body when L3 backup logic is implemented.
func (r *HarborBackupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	backup := &registryv1alpha1.HarborBackup{}
	if err := r.Get(ctx, req.NamespacedName, backup); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Backup is not yet implemented. Stamp a Degraded condition so the CR
	// reflects reality rather than looking healthy by default.
	apimeta.SetStatusCondition(&backup.Status.Conditions, metav1.Condition{
		Type:               "Degraded",
		Status:             metav1.ConditionTrue,
		Reason:             "NotImplemented",
		Message:            "HarborBackup is not yet implemented; set spec.schedule or trigger a manual backup when available",
		ObservedGeneration: backup.Generation,
	})
	backup.Status.Phase = registryv1alpha1.HarborBackupPhasePending

	if err := r.Status().Update(ctx, backup); err != nil {
		return ctrl.Result{}, fmt.Errorf("update backup status: %w", err)
	}
	return ctrl.Result{}, nil
}

// SetupWithManager registers the controller with the manager.
func (r *HarborBackupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&registryv1alpha1.HarborBackup{}).
		Named("harborbackup").
		Complete(r)
}
