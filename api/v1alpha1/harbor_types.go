/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HarborPhase describes the high-level lifecycle state of a Harbor instance.
type HarborPhase string

const (
	HarborPhasePending     HarborPhase = "Pending"
	HarborPhaseReconciling HarborPhase = "Reconciling"
	HarborPhaseReady       HarborPhase = "Ready"
	HarborPhaseFailed      HarborPhase = "Failed"
)

// RedisType selects internal (operator-managed) or external Redis.
// +kubebuilder:validation:Enum=internal;external
type RedisType string

const (
	RedisInternal RedisType = "internal"
	RedisExternal RedisType = "external"
)

// DatabaseSpec configures the Harbor PostgreSQL backend.
type HarborDatabaseSpec struct {
	// secretRef names a Secret containing keys: host, port, username, password, database.
	// +kubebuilder:validation:Required
	SecretRef corev1.LocalObjectReference `json:"secretRef"`
}

// RedisSpec configures the Redis instance for Harbor caching and job queuing.
type RedisSpec struct {
	// type selects internal (operator-managed Deployment) or external Redis.
	// +kubebuilder:default=internal
	// +optional
	Type RedisType `json:"type,omitempty"`

	// secretRef names a Secret containing key REDIS_URL (redis://[:password@]host:port/db).
	// Required when type is external.
	// +optional
	SecretRef *corev1.LocalObjectReference `json:"secretRef,omitempty"`
}

// S3StorageSpec configures S3-compatible object storage for Harbor artifacts.
type S3StorageSpec struct {
	// endpoint is the S3 endpoint URL (e.g. http://minio.minio.svc:9000).
	// +kubebuilder:validation:MinLength=1
	Endpoint string `json:"endpoint"`

	// bucket is the S3 bucket name for Harbor registry blobs.
	// +kubebuilder:validation:MinLength=1
	Bucket string `json:"bucket"`

	// region is the S3 region. Defaults to us-east-1 for MinIO.
	// +kubebuilder:default=us-east-1
	// +optional
	Region string `json:"region,omitempty"`

	// secretRef names a Secret containing keys: accessKey, secretKey.
	// +kubebuilder:validation:Required
	SecretRef corev1.LocalObjectReference `json:"secretRef"`
}

// StorageType selects the Harbor registry storage backend.
// +kubebuilder:validation:Enum=s3;filesystem
type StorageType string

const (
	StorageS3         StorageType = "s3"
	StorageFilesystem StorageType = "filesystem"
)

// HarborStorageSpec configures artifact storage for the registry component.
type HarborStorageSpec struct {
	// type selects the storage backend. Defaults to s3.
	// +kubebuilder:default=s3
	// +optional
	Type StorageType `json:"type,omitempty"`

	// s3 configures S3-compatible storage. Required when type is s3.
	// +optional
	S3 *S3StorageSpec `json:"s3,omitempty"`

	// size is the PVC size when type is filesystem. Defaults to 50Gi.
	// +kubebuilder:default="50Gi"
	// +optional
	Size resource.Quantity `json:"size,omitempty"`

	// storageClassName overrides the default StorageClass (filesystem mode only).
	// +optional
	StorageClassName *string `json:"storageClassName,omitempty"`

	// retain prevents PVC deletion when the Harbor CR is deleted. Defaults to true.
	// +kubebuilder:default=true
	// +optional
	Retain *bool `json:"retain,omitempty"`
}

// TrivySpec configures the Trivy vulnerability scanner component.
type TrivySpec struct {
	// enabled deploys the Trivy scanner sidecar. Defaults to true.
	// +kubebuilder:default=true
	// +optional
	Enabled *bool `json:"enabled,omitempty"`
}

// TLSSpec configures the Traefik IngressRoute TLS settings.
type HarborTLSSpec struct {
	// enabled creates IngressRoutes on the websecure entrypoint. Defaults to true.
	// +kubebuilder:default=true
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// entryPoint is the Traefik entrypoint to bind. Defaults to websecure.
	// +kubebuilder:default=websecure
	// +optional
	EntryPoint string `json:"entryPoint,omitempty"`

	// certResolver names a Traefik certResolver for automatic TLS.
	// Leave empty to rely on Cloudflare edge TLS termination.
	// +optional
	CertResolver string `json:"certResolver,omitempty"`
}

// HarborSpec defines the desired state of Harbor.
type HarborSpec struct {
	// image is the Harbor version tag applied to all component images
	// (e.g. "v2.13.0"). All goharbor/* images share a version tag.
	// +kubebuilder:default="v2.13.0"
	// +optional
	Image string `json:"image,omitempty"`

	// hostname is the public FQDN for Harbor (e.g. harbor.sscc.io).
	// Used for the Traefik IngressRoute and Harbor's externalURL.
	// +kubebuilder:validation:MinLength=1
	Hostname string `json:"hostname"`

	// adminPasswordSecretRef names a Secret containing key password for the
	// Harbor admin account. Operator creates it with a generated password if absent.
	// +optional
	AdminPasswordSecretRef *corev1.LocalObjectReference `json:"adminPasswordSecretRef,omitempty"`

	// database configures the PostgreSQL backend. Uses host Postgres at 10.0.0.100 by default.
	// +kubebuilder:validation:Required
	Database HarborDatabaseSpec `json:"database"`

	// redis configures the Redis instance for caching and job queuing.
	// +optional
	Redis RedisSpec `json:"redis,omitempty"`

	// storage configures artifact storage for the registry component.
	// +optional
	Storage HarborStorageSpec `json:"storage,omitempty"`

	// trivy configures the vulnerability scanner.
	// +optional
	Trivy TrivySpec `json:"trivy,omitempty"`

	// resources sets CPU/memory requests and limits applied to all Harbor components.
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// tls controls Traefik IngressRoute TLS configuration.
	// +optional
	TLS HarborTLSSpec `json:"tls,omitempty"`
}

// HarborStatus defines the observed state of Harbor.
type HarborStatus struct {
	// conditions reflect the current health of the Harbor resource.
	// Types: Available, Progressing, Degraded.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// observedGeneration is the .metadata.generation the controller last acted on.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// phase is a human-readable summary of the current lifecycle state.
	// +optional
	Phase HarborPhase `json:"phase,omitempty"`

	// url is the public URL of the Harbor instance.
	// +optional
	URL string `json:"url,omitempty"`

	// readyComponents is the count of Harbor components currently Ready.
	// +optional
	ReadyComponents int32 `json:"readyComponents,omitempty"`

	// totalComponents is the total count of managed Harbor components.
	// +optional
	TotalComponents int32 `json:"totalComponents,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Hostname",type=string,JSONPath=`.spec.hostname`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.readyComponents`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Harbor is the Schema for the harbors API.
type Harbor struct {
	metav1.TypeMeta `json:",inline"`

	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// +required
	Spec HarborSpec `json:"spec"`

	// +optional
	Status HarborStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// HarborList contains a list of Harbor.
type HarborList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Harbor `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Harbor{}, &HarborList{})
}
