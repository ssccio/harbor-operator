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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HarborBackupPhase describes the lifecycle state of a HarborBackup.
type HarborBackupPhase string

const (
	HarborBackupPhasePending   HarborBackupPhase = "Pending"
	HarborBackupPhaseRunning   HarborBackupPhase = "Running"
	HarborBackupPhaseSucceeded HarborBackupPhase = "Succeeded"
	HarborBackupPhaseFailed    HarborBackupPhase = "Failed"
)

// BackupS3Destination configures S3 destination for Harbor backups.
type BackupS3Destination struct {
	// bucket is the S3 bucket for backup storage.
	// +kubebuilder:validation:MinLength=1
	Bucket string `json:"bucket"`

	// prefix is the key prefix within the bucket.
	// +optional
	Prefix string `json:"prefix,omitempty"`

	// credentialsSecretRef names a Secret with keys: accessKey, secretKey.
	// +kubebuilder:validation:Required
	CredentialsSecretRef corev1.LocalObjectReference `json:"credentialsSecretRef"`
}

// BackupDestinationSpec configures where backups are written.
type BackupDestinationSpec struct {
	// s3 configures S3-compatible backup storage.
	// +optional
	S3 *BackupS3Destination `json:"s3,omitempty"`
}

// HarborBackupSpec defines the desired state of HarborBackup.
type HarborBackupSpec struct {
	// harborRef references the Harbor CR to back up.
	// +kubebuilder:validation:Required
	HarborRef corev1.LocalObjectReference `json:"harborRef"`

	// destination configures where backup artifacts are written.
	// +optional
	Destination BackupDestinationSpec `json:"destination,omitempty"`

	// schedule is a cron expression for automatic backups (e.g. "0 3 * * *").
	// Leave empty for on-demand only.
	// +optional
	Schedule string `json:"schedule,omitempty"`

	// retentionDays is the number of days to retain backup artifacts.
	// +kubebuilder:default=30
	// +optional
	RetentionDays int32 `json:"retentionDays,omitempty"`
}

// HarborBackupStatus defines the observed state of HarborBackup.
type HarborBackupStatus struct {
	// conditions reflect the current health of the HarborBackup resource.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// phase is the current lifecycle state of the backup.
	// +optional
	Phase HarborBackupPhase `json:"phase,omitempty"`

	// lastBackupTime is the timestamp of the most recent successful backup.
	// +optional
	LastBackupTime *metav1.Time `json:"lastBackupTime,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// HarborBackup is the Schema for the harborbackups API
type HarborBackup struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of HarborBackup
	// +required
	Spec HarborBackupSpec `json:"spec"`

	// status defines the observed state of HarborBackup
	// +optional
	Status HarborBackupStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// HarborBackupList contains a list of HarborBackup
type HarborBackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []HarborBackup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&HarborBackup{}, &HarborBackupList{})
}
