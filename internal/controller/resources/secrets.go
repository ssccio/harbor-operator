package resources

import (
	"crypto/rand"
	"encoding/hex"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	registryv1alpha1 "github.com/ken/harbor-operator/api/v1alpha1"
)

// InternalSecret builds the Secret holding auto-generated Harbor inter-service tokens.
// Keys: secretKey (16-char AES key), coreSecret, jobserviceSecret.
// The operator creates this once and never rotates it (rotation requires Harbor restart).
func InternalSecret(harbor *registryv1alpha1.Harbor) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      InternalSecretName(harbor),
			Namespace: harbor.Namespace,
			Labels:    labels(harbor, "internal"),
		},
		StringData: map[string]string{
			"secretKey":        randomHex(8),  // 16 hex chars = 16-byte AES key
			"coreSecret":       randomHex(16),
			"jobserviceSecret": randomHex(16),
		},
	}
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic("harbor-operator: failed to generate random secret: " + err.Error())
	}
	return hex.EncodeToString(b)
}
