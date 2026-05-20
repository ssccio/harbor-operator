package resources

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	registryv1alpha1 "github.com/ken/harbor-operator/api/v1alpha1"
)

// IngressRoute builds a Traefik traefik.io/v1alpha1 IngressRoute as an unstructured
// object so the operator does not take a hard Go-module dependency on Traefik types.
func IngressRoute(harbor *registryv1alpha1.Harbor) *unstructured.Unstructured {
	entryPoint := harbor.Spec.TLS.EntryPoint
	if entryPoint == "" {
		entryPoint = "websecure"
	}

	// All traffic goes to portal. Portal's nginx.conf proxies /api/, /c/,
	// /service/, /v2/ to harbor-core internally; Traefik only needs one rule.
	routes := []interface{}{
		map[string]interface{}{
			"match": "Host(`" + harbor.Spec.Hostname + "`)",
			"kind":  "Rule",
			"services": []interface{}{
				map[string]interface{}{"name": PortalServiceName(harbor), "port": int64(80)},
			},
		},
	}

	spec := map[string]interface{}{
		"entryPoints": []interface{}{entryPoint},
		"routes":      routes,
	}

	if tlsEnabled(harbor) {
		tlsSpec := map[string]interface{}{}
		if harbor.Spec.TLS.CertResolver != "" {
			tlsSpec["certResolver"] = harbor.Spec.TLS.CertResolver
		}
		spec["tls"] = tlsSpec
	}

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "traefik.io/v1alpha1",
			"kind":       "IngressRoute",
			"metadata": map[string]interface{}{
				"name":      name(harbor, "ingress"),
				"namespace": harbor.Namespace,
				"labels":    labelsUnstructured(harbor, "ingress"),
			},
			"spec": spec,
		},
	}
	return obj
}

func labelsUnstructured(harbor *registryv1alpha1.Harbor, component string) map[string]interface{} {
	m := labels(harbor, component)
	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
