package resources

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	registryv1alpha1 "github.com/ken/harbor-operator/api/v1alpha1"
)

func defaultResources() corev1.ResourceRequirements {
	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("100m"),
			corev1.ResourceMemory: resource.MustParse("256Mi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("500m"),
			corev1.ResourceMemory: resource.MustParse("512Mi"),
		},
	}
}

func mergeResources(override corev1.ResourceRequirements) corev1.ResourceRequirements {
	base := defaultResources()
	if len(override.Requests) > 0 {
		base.Requests = override.Requests
	}
	if len(override.Limits) > 0 {
		base.Limits = override.Limits
	}
	return base
}

// dbEnvFromSecret returns envVars that read DB connection fields from a Secret.
func dbEnvFromSecret(secretName string) []corev1.EnvVar {
	field := func(key, secretKey string) corev1.EnvVar {
		return corev1.EnvVar{
			Name: key,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
					Key:                  secretKey,
				},
			},
		}
	}
	return []corev1.EnvVar{
		field("POSTGRESQL_HOST", "host"),
		field("POSTGRESQL_PORT", "port"),
		field("POSTGRESQL_USERNAME", "username"),
		field("POSTGRESQL_PASSWORD", "password"),
		field("POSTGRESQL_DATABASE", "database"),
	}
}

// redisURLEnvVar returns an EnvVar for the given name set to either the
// internal Redis URL (static value) or the external Redis URL read from the
// user-provided Secret key REDIS_URL.
func redisURLEnvVar(harbor *registryv1alpha1.Harbor, envVarName string) corev1.EnvVar {
	if harbor.Spec.Redis.Type == registryv1alpha1.RedisExternal && harbor.Spec.Redis.SecretRef != nil {
		return corev1.EnvVar{
			Name: envVarName,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: harbor.Spec.Redis.SecretRef.Name},
					Key:                  "REDIS_URL",
				},
			},
		}
	}
	return corev1.EnvVar{Name: envVarName, Value: RedisURL(harbor)}
}

func internalSecretEnv(secretName string) []corev1.EnvVar {
	field := func(key, secretKey string) corev1.EnvVar {
		return corev1.EnvVar{
			Name: key,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
					Key:                  secretKey,
				},
			},
		}
	}
	return []corev1.EnvVar{
		field("CORE_SECRET", "coreSecret"),
		field("JOBSERVICE_SECRET", "jobserviceSecret"),
		field("SECRET_KEY", "secretKey"),
	}
}

func one() *int32 {
	n := int32(1)
	return &n
}

// CoreDeployment builds the harbor-core Deployment.
func CoreDeployment(harbor *registryv1alpha1.Harbor) *appsv1.Deployment {
	ls := labels(harbor, ComponentCore)
	ver := harborVersion(harbor)
	internalSec := InternalSecretName(harbor)
	dbSec := harbor.Spec.Database.SecretRef.Name
	extEndpoint := "https://" + harbor.Spec.Hostname

	trivyURL := ""
	if trivyEnabled(harbor) {
		trivyURL = fmt.Sprintf("http://%s:8080", TrivyServiceName(harbor))
	}

	env := []corev1.EnvVar{
		{Name: "EXT_ENDPOINT", Value: extEndpoint},
		{Name: "DATABASE_TYPE", Value: "postgresql"},
		{Name: "POSTGRESQL_SSLMODE", Value: "disable"},
		{Name: "REGISTRY_URL", Value: fmt.Sprintf("http://%s:5000", RegistryServiceName(harbor))},
		{Name: "REGISTRY_CONTROLLER_URL", Value: fmt.Sprintf("http://%s:8080", RegistryServiceName(harbor))},
		{Name: "JOBSERVICE_URL", Value: fmt.Sprintf("http://%s:8080", JobserviceServiceName(harbor))},
		{Name: "TOKEN_SERVICE_URL", Value: extEndpoint + "/service/token"},
		{Name: "PORTAL_URL", Value: fmt.Sprintf("http://%s:80", PortalServiceName(harbor))},
		{Name: "LOG_LEVEL", Value: "info"},
		{Name: "CONFIG_PATH", Value: "/etc/core/app.conf"},
		{Name: "SYNC_QUOTA", Value: "true"},
		{Name: "CHART_CACHE_DRIVER", Value: "redis"},
		{Name: "WITH_TRIVY", Value: fmt.Sprintf("%v", trivyEnabled(harbor))},
	}

	if trivyEnabled(harbor) {
		env = append(env, corev1.EnvVar{Name: "TRIVY_ADAPTER_URL", Value: trivyURL})
	}

	env = append(env, dbEnvFromSecret(dbSec)...)
	env = append(env, internalSecretEnv(internalSec)...)

	env = append(env, redisURLEnvVar(harbor, "_REDIS_URL_CORE"))

	if harbor.Spec.AdminPasswordSecretRef != nil {
		env = append(env, corev1.EnvVar{
			Name: "HARBOR_ADMIN_PASSWORD",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: harbor.Spec.AdminPasswordSecretRef.Name},
					Key:                  "password",
				},
			},
		})
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name(harbor, ComponentCore),
			Namespace: harbor.Namespace,
			Labels:    ls,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: one(),
			Selector: &metav1.LabelSelector{MatchLabels: ls},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: ls},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:      ComponentCore,
							Image:     fmt.Sprintf("goharbor/harbor-core:%s", ver),
							Env:       env,
							Resources: mergeResources(harbor.Spec.Resources),
							Ports: []corev1.ContainerPort{
								{Name: "http", ContainerPort: 8080, Protocol: corev1.ProtocolTCP},
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "config", MountPath: "/etc/core/app.conf", SubPath: "app.conf"},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/api/v2.0/ping",
										Port: intstr.FromInt(8080),
									},
								},
								InitialDelaySeconds: 30,
								PeriodSeconds:       10,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/api/v2.0/ping",
										Port: intstr.FromInt(8080),
									},
								},
								InitialDelaySeconds: 10,
								PeriodSeconds:       5,
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: CoreConfigMapName(harbor),
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

// PortalDeployment builds the harbor-portal Deployment (nginx serving UI static files).
func PortalDeployment(harbor *registryv1alpha1.Harbor) *appsv1.Deployment {
	ls := labels(harbor, ComponentPortal)
	ver := harborVersion(harbor)

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name(harbor, ComponentPortal),
			Namespace: harbor.Namespace,
			Labels:    ls,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: one(),
			Selector: &metav1.LabelSelector{MatchLabels: ls},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: ls},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  ComponentPortal,
							Image: fmt.Sprintf("goharbor/harbor-portal:%s", ver),
							Ports: []corev1.ContainerPort{
								{Name: "http", ContainerPort: 8080, Protocol: corev1.ProtocolTCP},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("50m"),
									corev1.ResourceMemory: resource.MustParse("64Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("200m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/",
										Port: intstr.FromInt(8080),
									},
								},
								InitialDelaySeconds: 10,
								PeriodSeconds:       10,
							},
						},
					},
				},
			},
		},
	}
}

// JobserviceDeployment builds the harbor-jobservice Deployment.
func JobserviceDeployment(harbor *registryv1alpha1.Harbor) *appsv1.Deployment {
	ls := labels(harbor, ComponentJobservice)
	ver := harborVersion(harbor)
	internalSec := InternalSecretName(harbor)

	env := []corev1.EnvVar{
		{Name: "CORE_URL", Value: fmt.Sprintf("http://%s:8080", CoreServiceName(harbor))},
		{Name: "TOKEN_SERVICE_URL", Value: "https://" + harbor.Spec.Hostname + "/service/token"},
		{Name: "REGISTRY_URL", Value: fmt.Sprintf("http://%s:5000", RegistryServiceName(harbor))},
		{Name: "REGISTRY_CONTROLLER_URL", Value: fmt.Sprintf("http://%s:8080", RegistryServiceName(harbor))},
		{Name: "JOB_SERVICE_POOL_WORKERS", Value: "10"},
		{Name: "JOB_SERVICE_POOL_BACKEND", Value: "redis"},
		{Name: "JOB_SERVICE_POOL_REDIS_NAMESPACE", Value: "harbor_job_service_namespace"},
		{Name: "JOB_SERVICE_LOGGER_SWEEPER_DURATION", Value: "1"},
		{Name: "LOG_LEVEL", Value: "INFO"},
	}
	env = append(env, redisURLEnvVar(harbor, "JOB_SERVICE_POOL_REDIS_URL"))
	env = append(env, internalSecretEnv(internalSec)...)

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name(harbor, ComponentJobservice),
			Namespace: harbor.Namespace,
			Labels:    ls,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: one(),
			Selector: &metav1.LabelSelector{MatchLabels: ls},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: ls},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:      ComponentJobservice,
							Image:     fmt.Sprintf("goharbor/harbor-jobservice:%s", ver),
							Env:       env,
							Resources: mergeResources(harbor.Spec.Resources),
							Ports: []corev1.ContainerPort{
								{Name: "http", ContainerPort: 8080, Protocol: corev1.ProtocolTCP},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/api/v1/stats",
										Port: intstr.FromInt(8080),
									},
								},
								InitialDelaySeconds: 20,
								PeriodSeconds:       10,
							},
						},
					},
				},
			},
		},
	}
}

// RegistryDeployment builds the harbor-registry Deployment with registryctl sidecar.
func RegistryDeployment(harbor *registryv1alpha1.Harbor) *appsv1.Deployment {
	ls := labels(harbor, ComponentRegistry)
	ver := harborVersion(harbor)
	internalSec := InternalSecretName(harbor)

	registryEnv := []corev1.EnvVar{
		{Name: "REGISTRY_HTTP_SECRET", ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: internalSec},
				Key:                  "secretKey",
			},
		}},
	}

	if harbor.Spec.Storage.Type == registryv1alpha1.StorageS3 && harbor.Spec.Storage.S3 != nil {
		s3 := harbor.Spec.Storage.S3
		registryEnv = append(registryEnv,
			corev1.EnvVar{
				Name: "REGISTRY_STORAGE_S3_ACCESSKEY",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: s3.SecretRef.Name},
						Key:                  "accessKey",
					},
				},
			},
			corev1.EnvVar{
				Name: "REGISTRY_STORAGE_S3_SECRETKEY",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: s3.SecretRef.Name},
						Key:                  "secretKey",
					},
				},
			},
		)
	}

	registryctlEnv := internalSecretEnv(internalSec)

	configVol := corev1.Volume{
		Name: "registry-config",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: RegistryConfigMapName(harbor),
				},
			},
		},
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name(harbor, ComponentRegistry),
			Namespace: harbor.Namespace,
			Labels:    ls,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: one(),
			Selector: &metav1.LabelSelector{MatchLabels: ls},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: ls},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  ComponentRegistry,
							Image: fmt.Sprintf("goharbor/registry-photon:%s", ver),
							Env:   registryEnv,
							Ports: []corev1.ContainerPort{
								{Name: "registry", ContainerPort: 5000, Protocol: corev1.ProtocolTCP},
								{Name: "metrics", ContainerPort: 5001, Protocol: corev1.ProtocolTCP},
							},
							Resources: mergeResources(harbor.Spec.Resources),
							VolumeMounts: []corev1.VolumeMount{
								{Name: "registry-config", MountPath: "/etc/registry/config.yml", SubPath: "config.yml"},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/",
										Port: intstr.FromInt(5001),
									},
								},
								InitialDelaySeconds: 10,
								PeriodSeconds:       10,
							},
						},
						{
							Name:  "registryctl",
							Image: fmt.Sprintf("goharbor/harbor-registryctl:%s", ver),
							Env:   registryctlEnv,
							Ports: []corev1.ContainerPort{
								{Name: "registryctl", ContainerPort: 8080, Protocol: corev1.ProtocolTCP},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("50m"),
									corev1.ResourceMemory: resource.MustParse("64Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("200m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "registry-config", MountPath: "/etc/registry/config.yml", SubPath: "config.yml"},
							},
						},
					},
					Volumes: []corev1.Volume{configVol},
				},
			},
		},
	}
}

// TrivyDeployment builds the harbor-trivy Deployment.
func TrivyDeployment(harbor *registryv1alpha1.Harbor) *appsv1.Deployment {
	ls := labels(harbor, ComponentTrivy)
	ver := harborVersion(harbor)

	env := []corev1.EnvVar{
		{Name: "SCANNER_LOG_LEVEL", Value: "info"},
		{Name: "SCANNER_TRIVY_CACHE_DIR", Value: "/home/scanner/.cache/trivy"},
		{Name: "SCANNER_TRIVY_REPORTS_DIR", Value: "/home/scanner/.cache/reports"},
		{Name: "SCANNER_TRIVY_VULN_TYPE", Value: "os,library"},
		{Name: "SCANNER_TRIVY_SEVERITY", Value: "UNKNOWN,LOW,MEDIUM,HIGH,CRITICAL"},
		{Name: "SCANNER_TRIVY_IGNORE_UNFIXED", Value: "false"},
		{Name: "SCANNER_REDIS_NAMESPACE", Value: "harbor.scanner.trivy"},
		{Name: "SCANNER_TRIVY_TIMEOUT", Value: "5m0s"},
	}
	env = append(env, redisURLEnvVar(harbor, "SCANNER_STORE_REDIS_URL"))
	env = append(env, redisURLEnvVar(harbor, "SCANNER_JOB_QUEUE_REDIS_URL"))

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name(harbor, ComponentTrivy),
			Namespace: harbor.Namespace,
			Labels:    ls,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: one(),
			Selector: &metav1.LabelSelector{MatchLabels: ls},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: ls},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:      ComponentTrivy,
							Image:     fmt.Sprintf("goharbor/trivy-adapter-photon:%s", ver),
							Env:       env,
							Resources: mergeResources(harbor.Spec.Resources),
							Ports: []corev1.ContainerPort{
								{Name: "http", ContainerPort: 8080, Protocol: corev1.ProtocolTCP},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/probe/healthy",
										Port: intstr.FromInt(8080),
									},
								},
								InitialDelaySeconds: 60,
								PeriodSeconds:       10,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/probe/ready",
										Port: intstr.FromInt(8080),
									},
								},
								InitialDelaySeconds: 30,
								PeriodSeconds:       5,
							},
						},
					},
				},
			},
		},
	}
}
