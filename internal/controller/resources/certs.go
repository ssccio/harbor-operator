package resources

// certs.go — generates the Harbor internal CA certificate used for JWT token
// signing between harbor-core and harbor-registry.
//
// Harbor's token-based auth flow:
//   - harbor-core signs JWTs with a private RSA key at /etc/core/private_key.pem
//   - harbor-registry verifies those JWTs using the matching CA cert at /etc/registry/root.crt
//
// The operator generates a self-signed RSA-4096 CA on first reconcile and
// stores both the private key and certificate in a single Secret. The Secret
// is create-once (same policy as InternalSecret) because rotating the cert
// requires restarting both core and registry simultaneously.

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	registryv1alpha1 "github.com/ken/harbor-operator/api/v1alpha1"
)

// CoreCertSecretName returns the name of the Secret holding the Harbor
// internal CA private key and certificate.
func CoreCertSecretName(harbor *registryv1alpha1.Harbor) string {
	return harbor.Name + "-harbor-core-certs"
}

// CoreCertSecret generates a new self-signed RSA CA Secret.
// Call this only when the Secret does not already exist.
func CoreCertSecret(harbor *registryv1alpha1.Harbor) (*corev1.Secret, error) {
	privKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, err
	}

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:   "harbor-token-issuer",
			Organization: []string{"Harbor"},
		},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour), // 10 years
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &privKey.PublicKey, privKey)
	if err != nil {
		return nil, err
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privKey)})

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      CoreCertSecretName(harbor),
			Namespace: harbor.Namespace,
			Labels:    labels(harbor, ComponentCore),
		},
		Data: map[string][]byte{
			"private_key.pem": keyPEM,
			"root.crt":        certPEM,
		},
	}, nil
}

// coreCertVolumeAndMount returns the Volume + VolumeMount that injects the
// private key into harbor-core at /etc/core/private_key.pem.
func coreCertVolumeAndMount(harbor *registryv1alpha1.Harbor) (corev1.Volume, corev1.VolumeMount) {
	vol := corev1.Volume{
		Name: "core-certs",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: CoreCertSecretName(harbor),
				Items: []corev1.KeyToPath{
					{Key: "private_key.pem", Path: "private_key.pem"},
				},
			},
		},
	}
	mount := corev1.VolumeMount{
		Name:      "core-certs",
		MountPath: "/etc/core/private_key.pem",
		SubPath:   "private_key.pem",
		ReadOnly:  true,
	}
	return vol, mount
}

// registryCertVolumeAndMount returns the Volume + VolumeMount that injects
// the CA certificate into harbor-registry at /etc/registry/root.crt.
func registryCertVolumeAndMount(harbor *registryv1alpha1.Harbor) (corev1.Volume, corev1.VolumeMount) {
	vol := corev1.Volume{
		Name: "registry-certs",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: CoreCertSecretName(harbor),
				Items: []corev1.KeyToPath{
					{Key: "root.crt", Path: "root.crt"},
				},
			},
		},
	}
	mount := corev1.VolumeMount{
		Name:      "registry-certs",
		MountPath: "/etc/registry/root.crt",
		SubPath:   "root.crt",
		ReadOnly:  true,
	}
	return vol, mount
}
