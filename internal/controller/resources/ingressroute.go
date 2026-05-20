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

	host := "Host(`" + harbor.Spec.Hostname + "`)"

	// Harbor routing — Traefik evaluates routes top-to-bottom; specific paths first.
	//
	//  /api/         → core (REST API)
	//  /c/           → core (legacy cookie auth)
	//  /service/     → core (token service, notifications)
	//  /v2/          → registry (OCI distribution API, proxied through core auth)
	//  /             → portal (Angular SPA, plain nginx)
	routes := []interface{}{
		map[string]interface{}{
			"match": host + " && PathPrefix(`/api/`)",
			"kind":  "Rule",
			"services": []interface{}{
				map[string]interface{}{"name": CoreServiceName(harbor), "port": int64(80)},
			},
		},
		map[string]interface{}{
			"match": host + " && PathPrefix(`/c/`)",
			"kind":  "Rule",
			"services": []interface{}{
				map[string]interface{}{"name": CoreServiceName(harbor), "port": int64(80)},
			},
		},
		map[string]interface{}{
			"match": host + " && PathPrefix(`/service/`)",
			"kind":  "Rule",
			"services": []interface{}{
				map[string]interface{}{"name": CoreServiceName(harbor), "port": int64(80)},
			},
		},
		map[string]interface{}{
			"match": host + " && PathPrefix(`/v2/`)",
			"kind":  "Rule",
			"services": []interface{}{
				map[string]interface{}{"name": CoreServiceName(harbor), "port": int64(80)},
			},
		},
		map[string]interface{}{
			"match": host,
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
