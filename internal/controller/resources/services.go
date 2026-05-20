package resources

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	registryv1alpha1 "github.com/ken/harbor-operator/api/v1alpha1"
)

func CoreService(harbor *registryv1alpha1.Harbor) *corev1.Service {
	ls := labels(harbor, ComponentCore)
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      CoreServiceName(harbor),
			Namespace: harbor.Namespace,
			Labels:    ls,
		},
		Spec: corev1.ServiceSpec{
			Selector: ls,
			Ports: []corev1.ServicePort{
				{Name: "http", Port: 80, TargetPort: intstr.FromInt(8080)},
			},
		},
	}
}

func PortalService(harbor *registryv1alpha1.Harbor) *corev1.Service {
	ls := labels(harbor, ComponentPortal)
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      PortalServiceName(harbor),
			Namespace: harbor.Namespace,
			Labels:    ls,
		},
		Spec: corev1.ServiceSpec{
			Selector: ls,
			// harbor-portal nginx listens on 80; targetPort matches.
			Ports: []corev1.ServicePort{
				{Name: "http", Port: 80, TargetPort: intstr.FromInt(80)},
			},
		},
	}
}

func JobserviceService(harbor *registryv1alpha1.Harbor) *corev1.Service {
	ls := labels(harbor, ComponentJobservice)
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      JobserviceServiceName(harbor),
			Namespace: harbor.Namespace,
			Labels:    ls,
		},
		Spec: corev1.ServiceSpec{
			Selector: ls,
			Ports: []corev1.ServicePort{
				{Name: "http", Port: 8080, TargetPort: intstr.FromInt(8080)},
			},
		},
	}
}

func RegistryService(harbor *registryv1alpha1.Harbor) *corev1.Service {
	ls := labels(harbor, ComponentRegistry)
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      RegistryServiceName(harbor),
			Namespace: harbor.Namespace,
			Labels:    ls,
		},
		Spec: corev1.ServiceSpec{
			Selector: ls,
			Ports: []corev1.ServicePort{
				{Name: "registry", Port: 5000, TargetPort: intstr.FromInt(5000)},
				{Name: "controller", Port: 8080, TargetPort: intstr.FromInt(8080)},
			},
		},
	}
}

func TrivyService(harbor *registryv1alpha1.Harbor) *corev1.Service {
	ls := labels(harbor, ComponentTrivy)
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      TrivyServiceName(harbor),
			Namespace: harbor.Namespace,
			Labels:    ls,
		},
		Spec: corev1.ServiceSpec{
			Selector: ls,
			Ports: []corev1.ServicePort{
				{Name: "http", Port: 8080, TargetPort: intstr.FromInt(8080)},
			},
		},
	}
}
