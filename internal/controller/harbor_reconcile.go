package controller

// harbor_reconcile.go — idempotent resource-ensure helpers for the Harbor controller.
//
// Each helper follows the same pattern:
//  1. Build the desired object via a resources.Xxx() builder.
//  2. Set the controller owner reference so the API server garbage-collects
//     the resource when the Harbor CR is deleted.
//  3. Call CreateOrUpdate with a mutate function that copies only the fields
//     the operator manages, leaving API-server-assigned fields (ClusterIP,
//     resource version, etc.) intact.
//
// IMPORTANT: always set Name + Namespace on the *existing* object BEFORE
// calling CreateOrUpdate, not inside the mutate function. CreateOrUpdate uses
// the existing object's identity to issue the Get; if it's empty the lookup
// fails and every reconcile creates a duplicate.

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	registryv1alpha1 "github.com/ken/harbor-operator/api/v1alpha1"
	"github.com/ken/harbor-operator/internal/controller/resources"
)

// reconcileComponents creates or updates every resource owned by the Harbor CR
// and returns the total expected component count and the number currently ready.
// "Ready" is defined as at least one replica reporting ReadyReplicas > 0 on the
// owning Deployment.
func (r *HarborReconciler) reconcileComponents(ctx context.Context, harbor *registryv1alpha1.Harbor) (total, ready int32, err error) {
	// ── Secrets ────────────────────────────────────────────────────────────────
	// Internal Harbor inter-service tokens: created once, never updated.
	// Rotating them requires restarting all Harbor pods.
	if err = r.ensureInternalSecret(ctx, harbor); err != nil {
		return 0, 0, fmt.Errorf("internal secret: %w", err)
	}
	// Core TLS cert: private key mounted in harbor-core, root.crt in harbor-registry.
	// Created once — rotating requires restarting both core and registry simultaneously.
	if err = r.ensureCoreCertSecret(ctx, harbor); err != nil {
		return 0, 0, fmt.Errorf("core cert secret: %w", err)
	}

	// ── ConfigMaps ─────────────────────────────────────────────────────────────
	// app.conf is consumed by harbor-core via a volume mount at /etc/core/app.conf.
	if err = r.ensureConfigMap(ctx, harbor, resources.CoreConfigMap(harbor)); err != nil {
		return 0, 0, fmt.Errorf("core configmap: %w", err)
	}
	// config.yml is consumed by harbor-registry at /etc/registry/config.yml.
	if err = r.ensureConfigMap(ctx, harbor, resources.RegistryConfigMap(harbor)); err != nil {
		return 0, 0, fmt.Errorf("registry configmap: %w", err)
	}
	// registryctl config at /etc/registryctl/config.yml for the sidecar.
	if err = r.ensureConfigMap(ctx, harbor, resources.RegistryctlConfigMap(harbor)); err != nil {
		return 0, 0, fmt.Errorf("registryctl configmap: %w", err)
	}
	// jobservice config at /etc/jobservice/config.yml.
	if err = r.ensureConfigMap(ctx, harbor, resources.JobserviceConfigMap(harbor)); err != nil {
		return 0, 0, fmt.Errorf("jobservice configmap: %w", err)
	}

	// ── Internal Redis (optional) ──────────────────────────────────────────────
	// Only deployed when redis.type == "internal" (the default).
	// When external Redis is configured, the user supplies the connection URL
	// in a Secret referenced by spec.redis.secretRef.
	if harbor.Spec.Redis.Type != registryv1alpha1.RedisExternal {
		if err = r.ensureDeployment(ctx, harbor, resources.RedisDeployment(harbor)); err != nil {
			return 0, 0, fmt.Errorf("redis deployment: %w", err)
		}
		if err = r.ensureService(ctx, harbor, resources.RedisService(harbor)); err != nil {
			return 0, 0, fmt.Errorf("redis service: %w", err)
		}
	}

	// ── Harbor component Deployments ───────────────────────────────────────────
	// Build the list dynamically so trivy can be toggled via spec.trivy.enabled.
	deployments := []*appsv1.Deployment{
		resources.CoreDeployment(harbor),
		resources.PortalDeployment(harbor),
		resources.JobserviceDeployment(harbor),
		// Registry includes a registryctl sidecar in the same Pod.
		resources.RegistryDeployment(harbor),
	}
	if trivyEnabled(harbor) {
		deployments = append(deployments, resources.TrivyDeployment(harbor))
	}

	// ── Harbor component Services ──────────────────────────────────────────────
	svcs := []*corev1.Service{
		resources.CoreService(harbor),
		resources.PortalService(harbor),
		resources.JobserviceService(harbor),
		resources.RegistryService(harbor),
	}
	if trivyEnabled(harbor) {
		svcs = append(svcs, resources.TrivyService(harbor))
	}

	for _, deploy := range deployments {
		if err = r.ensureDeployment(ctx, harbor, deploy); err != nil {
			return 0, 0, fmt.Errorf("deployment %s: %w", deploy.Name, err)
		}
		total++
		if deploymentReady(ctx, r.Client, deploy.Namespace, deploy.Name) {
			ready++
		}
	}

	for _, svc := range svcs {
		if err = r.ensureService(ctx, harbor, svc); err != nil {
			return 0, 0, fmt.Errorf("service %s: %w", svc.Name, err)
		}
	}

	// ── Traefik IngressRoute ────────────────────────────────────────────────────
	if err = r.ensureIngressRoute(ctx, harbor); err != nil {
		return 0, 0, fmt.Errorf("ingressroute: %w", err)
	}

	return total, ready, nil
}

// ensureCoreCertSecret generates and creates the Harbor internal CA cert Secret
// if it does not already exist. Create-once: the cert is shared between
// harbor-core (private key) and harbor-registry (root.crt) for JWT signing.
func (r *HarborReconciler) ensureCoreCertSecret(ctx context.Context, harbor *registryv1alpha1.Harbor) error {
	existing := &corev1.Secret{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      resources.CoreCertSecretName(harbor),
		Namespace: harbor.Namespace,
	}, existing)
	if err == nil {
		return nil // already exists
	}
	if !errors.IsNotFound(err) {
		return err
	}

	desired, err := resources.CoreCertSecret(harbor)
	if err != nil {
		return fmt.Errorf("generate core cert: %w", err)
	}
	if err := controllerutil.SetControllerReference(harbor, desired, r.Scheme); err != nil {
		return err
	}
	return r.Create(ctx, desired)
}

// ensureInternalSecret creates the Harbor internal-secrets Secret if it does
// not already exist. It is intentionally never updated after first creation.
func (r *HarborReconciler) ensureInternalSecret(ctx context.Context, harbor *registryv1alpha1.Harbor) error {
	desired := resources.InternalSecret(harbor)
	if err := controllerutil.SetControllerReference(harbor, desired, r.Scheme); err != nil {
		return err
	}

	existing := &corev1.Secret{}
	err := r.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, existing)
	if errors.IsNotFound(err) {
		return r.Create(ctx, desired)
	}
	return err // nil = already exists, no-op
}

// ensureConfigMap creates or updates a ConfigMap, replacing its data entirely.
func (r *HarborReconciler) ensureConfigMap(ctx context.Context, harbor *registryv1alpha1.Harbor, desired *corev1.ConfigMap) error {
	if err := controllerutil.SetControllerReference(harbor, desired, r.Scheme); err != nil {
		return err
	}
	existing := &corev1.ConfigMap{}
	existing.Name = desired.Name
	existing.Namespace = desired.Namespace
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, existing, func() error {
		existing.Data = desired.Data
		existing.Labels = desired.Labels
		return nil
	})
	return err
}

// ensureDeployment creates or updates a Deployment, replacing the entire spec.
// The operator is the sole owner of the Deployment spec; manual edits will be
// reverted on the next reconcile.
func (r *HarborReconciler) ensureDeployment(ctx context.Context, harbor *registryv1alpha1.Harbor, desired *appsv1.Deployment) error {
	if err := controllerutil.SetControllerReference(harbor, desired, r.Scheme); err != nil {
		return err
	}
	existing := &appsv1.Deployment{}
	existing.Name = desired.Name
	existing.Namespace = desired.Namespace
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, existing, func() error {
		existing.Labels = desired.Labels
		existing.Spec = desired.Spec
		return nil
	})
	return err
}

// ensureService creates or updates a Service.
// ClusterIP is assigned by the API server on creation and must be preserved on
// subsequent updates — overwriting it with an empty string causes a 422.
func (r *HarborReconciler) ensureService(ctx context.Context, harbor *registryv1alpha1.Harbor, desired *corev1.Service) error {
	if err := controllerutil.SetControllerReference(harbor, desired, r.Scheme); err != nil {
		return err
	}
	existing := &corev1.Service{}
	existing.Name = desired.Name
	existing.Namespace = desired.Namespace
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, existing, func() error {
		existing.Labels = desired.Labels
		savedClusterIP := existing.Spec.ClusterIP
		existing.Spec = desired.Spec
		existing.Spec.ClusterIP = savedClusterIP
		return nil
	})
	return err
}

// ensureIngressRoute creates or updates a Traefik IngressRoute using an
// unstructured object so the operator does not depend on Traefik Go types.
//
// Owner references on unstructured objects are set manually because
// controllerutil.SetControllerReference requires a typed client.Object.
func (r *HarborReconciler) ensureIngressRoute(ctx context.Context, harbor *registryv1alpha1.Harbor) error {
	desired := resources.IngressRoute(harbor)

	// Set owner reference manually on the unstructured object.
	desired.SetOwnerReferences([]metav1.OwnerReference{
		{
			APIVersion:         harbor.APIVersion,
			Kind:               harbor.Kind,
			Name:               harbor.Name,
			UID:                harbor.UID,
			BlockOwnerDeletion: boolPtr(true),
			Controller:         boolPtr(true),
		},
	})

	existing := &unstructured.Unstructured{}
	existing.SetGroupVersionKind(desired.GroupVersionKind())
	existing.SetName(desired.GetName())
	existing.SetNamespace(desired.GetNamespace())

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, existing, func() error {
		existing.Object["spec"] = desired.Object["spec"]
		existing.SetLabels(desired.GetLabels())
		existing.SetOwnerReferences(desired.GetOwnerReferences())
		return nil
	})
	return err
}

// deploymentReady returns true when at least one replica in the Deployment is
// reporting as ready. Used to compute status.readyComponents.
func deploymentReady(ctx context.Context, c client.Client, namespace, name string) bool {
	d := &appsv1.Deployment{}
	if err := c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, d); err != nil {
		return false
	}
	return d.Status.ReadyReplicas > 0
}

// trivyEnabled reads spec.trivy.enabled, defaulting to true when unset.
// Mirrors the helper in resources/common.go but scoped to the controller package.
func trivyEnabled(harbor *registryv1alpha1.Harbor) bool {
	if harbor.Spec.Trivy.Enabled == nil {
		return true
	}
	return *harbor.Spec.Trivy.Enabled
}

func boolPtr(b bool) *bool { return &b }
