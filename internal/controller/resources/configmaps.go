package resources

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	registryv1alpha1 "github.com/ken/harbor-operator/api/v1alpha1"
)

// CoreConfigMap builds the ConfigMap containing harbor-core's app.conf.
func CoreConfigMap(harbor *registryv1alpha1.Harbor) *corev1.ConfigMap {
	extEndpoint := "https://" + harbor.Spec.Hostname

	trivyURL := ""
	if trivyEnabled(harbor) {
		trivyURL = fmt.Sprintf("http://%s:8080", TrivyServiceName(harbor))
	}

	appConf := fmt.Sprintf(`appname = Harbor
runmode = prod
enablexsrf = true
xsrfexpire = 3600
keypath = /etc/core/private_key.pem
secret = _SECRET_KEY_PLACEHOLDER_
ext_endpoint = %s
auth_mode = db_auth
self_registration = on
ldap_url =
ldap_search_dn =
ldap_search_pwd =
ldap_base_dn =
ldap_filter =
ldap_uid = cn
ldap_scope = 2
ldap_timeout = 5
ldap_verify_cert = true
ldap_group_base_dn =
ldap_group_filter =
ldap_group_gid =
ldap_group_scope = 2
db_host = _DB_HOST_PLACEHOLDER_
db_port = _DB_PORT_PLACEHOLDER_
db_name = _DB_NAME_PLACEHOLDER_
db_user = _DB_USER_PLACEHOLDER_
db_password = _DB_PASSWORD_PLACEHOLDER_
db_ssl_mode = disable
max_open_conns = 1000
max_idle_conns = 100
redis_url = _REDIS_URL_PLACEHOLDER_
token_expiration = 30
admiral_url =
with_notary = false
with_trivy = %v
trivy_adapter_url = %s
registry_url = http://%s:5000
registry_controller_url = http://%s:8080
token_service_url = %s/service/token
jobservice_url = http://%s:8080
portal_url = http://%s:80
`,
		extEndpoint,
		trivyEnabled(harbor),
		trivyURL,
		RegistryServiceName(harbor),
		RegistryServiceName(harbor),
		CoreServiceName(harbor),
		JobserviceServiceName(harbor),
		PortalServiceName(harbor),
	)

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      CoreConfigMapName(harbor),
			Namespace: harbor.Namespace,
			Labels:    labels(harbor, ComponentCore),
		},
		Data: map[string]string{
			"app.conf": appConf,
		},
	}
}

// RegistryConfigMap builds the ConfigMap containing the OCI registry config.yml.
func RegistryConfigMap(harbor *registryv1alpha1.Harbor) *corev1.ConfigMap {
	var storageSection string

	if harbor.Spec.Storage.Type == registryv1alpha1.StorageS3 && harbor.Spec.Storage.S3 != nil {
		s3 := harbor.Spec.Storage.S3
		region := s3.Region
		if region == "" {
			region = "us-east-1"
		}
		storageSection = fmt.Sprintf(`
  s3:
    region: %s
    bucket: %s
    regionendpoint: %s
    secure: false
    skipverify: false
    v4auth: true
    chunksize: 5242880
    rootdirectory: /registry
`, region, s3.Bucket, s3.Endpoint)
	} else {
		storageSection = `
  filesystem:
    rootdirectory: /storage
`
	}

	registryYAML := fmt.Sprintf(`version: 0.1
log:
  level: info
  formatter: text
storage:%s
http:
  addr: :5000
  secret: _REGISTRY_HTTP_SECRET_PLACEHOLDER_
  debug:
    addr: :5001
auth:
  token:
    issuer: harbor-token-issuer
    realm: %s/service/token
    rootcertbundle: /etc/registry/root.crt
    service: harbor-registry
validation:
  disabled: true
compatibility:
  schema1:
    enabled: false
`, storageSection, "https://"+harbor.Spec.Hostname)

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      RegistryConfigMapName(harbor),
			Namespace: harbor.Namespace,
			Labels:    labels(harbor, ComponentRegistry),
		},
		Data: map[string]string{
			"config.yml": registryYAML,
		},
	}
}
