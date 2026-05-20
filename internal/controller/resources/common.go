package resources

import (
	registryv1alpha1 "github.com/ken/harbor-operator/api/v1alpha1"
)

// Component names
const (
	ComponentCore       = "core"
	ComponentPortal     = "portal"
	ComponentJobservice = "jobservice"
	ComponentRegistry   = "registry"
	ComponentTrivy      = "trivy"
	ComponentRedis      = "redis"
)

// defaultHarborVersion is the version tag appended to all goharbor/* images
// when spec.image is not set. Must be a bare tag (e.g. "v2.13.0"), not a
// full image reference — it is interpolated as "goharbor/harbor-core:<tag>".
const defaultHarborVersion = "v2.13.0"

func name(harbor *registryv1alpha1.Harbor, component string) string {
	return harbor.Name + "-harbor-" + component
}

func labels(harbor *registryv1alpha1.Harbor, component string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "harbor",
		"app.kubernetes.io/instance":   harbor.Name,
		"app.kubernetes.io/component":  component,
		"app.kubernetes.io/managed-by": "harbor-operator",
	}
}

func harborVersion(harbor *registryv1alpha1.Harbor) string {
	if harbor.Spec.Image != "" {
		return harbor.Spec.Image
	}
	return defaultHarborVersion
}

// InternalSecretName is the Secret holding auto-generated Harbor inter-service tokens.
func InternalSecretName(harbor *registryv1alpha1.Harbor) string {
	return harbor.Name + "-harbor-internal"
}

// CoreConfigMapName is the ConfigMap holding harbor-core app.conf.
func CoreConfigMapName(harbor *registryv1alpha1.Harbor) string {
	return harbor.Name + "-harbor-core-config"
}

// RegistryConfigMapName is the ConfigMap holding registry config.yml.
func RegistryConfigMapName(harbor *registryv1alpha1.Harbor) string {
	return harbor.Name + "-harbor-registry-config"
}

// CoreServiceName returns the ClusterIP Service name for harbor-core.
func CoreServiceName(harbor *registryv1alpha1.Harbor) string {
	return name(harbor, ComponentCore)
}

// PortalServiceName returns the ClusterIP Service name for harbor-portal.
func PortalServiceName(harbor *registryv1alpha1.Harbor) string {
	return name(harbor, ComponentPortal)
}

// JobserviceServiceName returns the ClusterIP Service name for harbor-jobservice.
func JobserviceServiceName(harbor *registryv1alpha1.Harbor) string {
	return name(harbor, ComponentJobservice)
}

// RegistryServiceName returns the ClusterIP Service name for harbor-registry.
func RegistryServiceName(harbor *registryv1alpha1.Harbor) string {
	return name(harbor, ComponentRegistry)
}

// TrivyServiceName returns the ClusterIP Service name for harbor-trivy.
func TrivyServiceName(harbor *registryv1alpha1.Harbor) string {
	return name(harbor, ComponentTrivy)
}

// RedisServiceName returns the ClusterIP Service name for internal Redis.
func RedisServiceName(harbor *registryv1alpha1.Harbor) string {
	return name(harbor, ComponentRedis)
}

func trivyEnabled(harbor *registryv1alpha1.Harbor) bool {
	if harbor.Spec.Trivy.Enabled == nil {
		return true
	}
	return *harbor.Spec.Trivy.Enabled
}

func tlsEnabled(harbor *registryv1alpha1.Harbor) bool {
	if harbor.Spec.TLS.Enabled == nil {
		return true
	}
	return *harbor.Spec.TLS.Enabled
}
