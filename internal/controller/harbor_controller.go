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
// # Reconcile contract
//
// On every change to a Harbor CR the controller:
//  1. Adds a finalizer so cleanup runs before the CR is removed from etcd.
//  2. Creates (once, never updates) the internal-secrets Secret containing
//     auto-generated Harbor inter-service tokens (secretKey, coreSecret,
//     jobserviceSecret). Rotating these requires a full Harbor restart, so
//     they are set-once.
//  3. Creates/updates the CoreConfigMap (app.conf) and RegistryConfigMap
//     (config.yml) ConfigMaps. These are fully idempotent — safe to recreate.
//  4. If redis.type == internal, reconciles the operator-managed Redis
//     Deployment + Service.
//  5. Reconciles Deployments and Services for core, portal, jobservice,
//     registry (+registryctl sidecar), and trivy (when enabled).
//  6. Reconciles a Traefik IngressRoute targeting harbor-core on port 80.
//  7. Updates status.phase, status.conditions, and status.readyComponents.
//
// On deletion the finalizer runs cleanup() which explicitly drops the
// IngressRoute (an unstructured resource whose owner-reference GC may lag).
// All typed resources (Deployments, Services, ConfigMaps, Secrets) are
// garbage-collected automatically via owner references.
package controller

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	registryv1alpha1 "github.com/ken/harbor-operator/api/v1alpha1"
	"github.com/ken/harbor-operator/internal/controller/resources"
)

// harborFinalizer is placed on every Harbor CR to ensure the IngressRoute is
// explicitly deleted before the CR disappears from etcd.
const harborFinalizer = "registry.gocadmium.dev/finalizer"

// HarborReconciler reconciles a Harbor object.
type HarborReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=registry.gocadmium.dev,resources=harbors,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=registry.gocadmium.dev,resources=harbors/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=registry.gocadmium.dev,resources=harbors/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=traefik.io,resources=ingressroutes,verbs=get;list;watch;create;update;patch;delete

// Reconcile drives a Harbor CR toward the desired state described in its spec.
// See the package-level doc comment for the full reconcile contract.
func (r *HarborReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	harbor := &registryv1alpha1.Harbor{}
	if err := r.Get(ctx, req.NamespacedName, harbor); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Deletion path: run cleanup then remove finalizer.
	if !harbor.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(harbor, harborFinalizer) {
			if err := r.cleanup(ctx, harbor); err != nil {
				return ctrl.Result{}, err
			}
			controllerutil.RemoveFinalizer(harbor, harborFinalizer)
			return ctrl.Result{}, r.Update(ctx, harbor)
		}
		return ctrl.Result{}, nil
	}

	// Ensure finalizer is present before doing any work.
	if !controllerutil.ContainsFinalizer(harbor, harborFinalizer) {
		controllerutil.AddFinalizer(harbor, harborFinalizer)
		if err := r.Update(ctx, harbor); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Signal reconcile in progress; ignore status update error (best-effort).
	setProgressing(harbor, "Reconciling", "reconcile started")
	_ = updateStatus(ctx, r.Client, harbor)

	// Re-fetch to pick up the new resource version written by status update.
	if err := r.Get(ctx, req.NamespacedName, harbor); err != nil {
		return ctrl.Result{}, err
	}

	total, ready, err := r.reconcileComponents(ctx, harbor)
	if err != nil {
		// Re-fetch then stamp failed status.
		_ = r.Get(ctx, req.NamespacedName, harbor)
		setDegraded(harbor, "ReconcileError", err.Error())
		_ = updateStatus(ctx, r.Client, harbor)
		return ctrl.Result{}, err
	}

	// Re-fetch for final status write.
	if err := r.Get(ctx, req.NamespacedName, harbor); err != nil {
		return ctrl.Result{}, err
	}
	if ready < total {
		setProgressing(harbor, "ComponentsNotReady", fmt.Sprintf("%d/%d components ready", ready, total))
	} else {
		setReady(harbor, ready, total)
	}
	return ctrl.Result{}, updateStatus(ctx, r.Client, harbor)
}

// cleanup explicitly removes the Traefik IngressRoute before the CR is deleted.
// Typed owned resources are garbage-collected by the API server via owner
// references; the unstructured IngressRoute gets the same treatment in theory,
// but explicit deletion avoids any edge-case lag in cross-group GC.
func (r *HarborReconciler) cleanup(ctx context.Context, harbor *registryv1alpha1.Harbor) error {
	ir := resources.IngressRoute(harbor)
	existing := &unstructured.Unstructured{}
	existing.SetGroupVersionKind(ir.GroupVersionKind())
	existing.SetName(ir.GetName())
	existing.SetNamespace(ir.GetNamespace())

	err := r.Delete(ctx, existing)
	if client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("delete ingressroute: %w", err)
	}
	return nil
}

// SetupWithManager registers the reconciler and declares which owned types
// should trigger re-reconcile when they change.
func (r *HarborReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&registryv1alpha1.Harbor{}).
		// Watches trigger a reconcile whenever an owned Deployment, Service,
		// ConfigMap, or Secret is created, updated, or deleted — this is how
		// the operator auto-heals drift on managed resources.
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Secret{}).
		Named("harbor").
		Complete(r)
}
