package resources

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	registryv1alpha1 "github.com/ken/harbor-operator/api/v1alpha1"
)

// RedisDeployment builds the internal Redis Deployment for Harbor job queuing and caching.
func RedisDeployment(harbor *registryv1alpha1.Harbor) *appsv1.Deployment {
	ls := labels(harbor, ComponentRedis)
	one := int32(1)

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name(harbor, ComponentRedis),
			Namespace: harbor.Namespace,
			Labels:    ls,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &one,
			Selector: &metav1.LabelSelector{MatchLabels: ls},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: ls},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "redis",
							Image: "redis:7.2-alpine",
							Ports: []corev1.ContainerPort{
								{Name: "redis", ContainerPort: 6379, Protocol: corev1.ProtocolTCP},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("50m"),
									corev1.ResourceMemory: resource.MustParse("64Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("200m"),
									corev1.ResourceMemory: resource.MustParse("256Mi"),
								},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									Exec: &corev1.ExecAction{
										Command: []string{"redis-cli", "ping"},
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

// RedisService builds the ClusterIP Service for internal Redis.
func RedisService(harbor *registryv1alpha1.Harbor) *corev1.Service {
	ls := labels(harbor, ComponentRedis)
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      RedisServiceName(harbor),
			Namespace: harbor.Namespace,
			Labels:    ls,
		},
		Spec: corev1.ServiceSpec{
			Selector: ls,
			Ports: []corev1.ServicePort{
				{Name: "redis", Port: 6379, TargetPort: intstr.FromInt(6379)},
			},
		},
	}
}

// RedisURL builds the Redis connection URL for internal mode.
func RedisURL(harbor *registryv1alpha1.Harbor) string {
	return "redis://" + RedisServiceName(harbor) + ":6379/0"
}
