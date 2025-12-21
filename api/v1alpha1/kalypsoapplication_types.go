/*
Copyright 2025.

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// KalypsoApplicationSpec defines the desired state of KalypsoApplication
type KalypsoApplicationSpec struct {
	// ProjectRef is the reference to parent KalypsoProject
	// +kubebuilder:validation:Required
	ProjectRef string `json:"projectRef"`

	// Description provides a description of the application
	// +optional
	Description string `json:"description,omitempty"`

	// Source defines the Git repository configuration
	// +optional
	Source *GitSourceSpec `json:"source,omitempty"`

	// Storage defines common storage/secret configuration for all TritonServers
	// +optional
	Storage *StorageSpec `json:"storage,omitempty"`
}

// GitSourceSpec defines the Git repository configuration
type GitSourceSpec struct {
	// GitRepository is the Git repository URL
	// +kubebuilder:validation:Required
	GitRepository string `json:"gitRepository"`

	// Branch is the target branch (default: main)
	// +optional
	// +kubebuilder:default="main"
	Branch string `json:"branch,omitempty"`

	// BuildWorkflow is the path to GitHub Action workflow file
	// +optional
	BuildWorkflow string `json:"buildWorkflow,omitempty"`
}

// StorageSpec defines the storage configuration
type StorageSpec struct {
	// SecretName is the name of secret containing credentials
	// +kubebuilder:validation:Required
	SecretName string `json:"secretName"`

	// Region is the cloud region for storage
	// +optional
	Region string `json:"region,omitempty"`

	// Endpoint is the S3-compatible endpoint URL (for MinIO, etc.)
	// +optional
	Endpoint string `json:"endpoint,omitempty"`
}

// ApplicationPhase represents the current phase of the application
// +kubebuilder:validation:Enum=Pending;Ready;Failed
type ApplicationPhase string

const (
	// ApplicationPhasePending indicates the application is pending
	ApplicationPhasePending ApplicationPhase = "Pending"
	// ApplicationPhaseReady indicates the application is ready
	ApplicationPhaseReady ApplicationPhase = "Ready"
	// ApplicationPhaseFailed indicates the application has failed
	ApplicationPhaseFailed ApplicationPhase = "Failed"
)

// KalypsoApplicationStatus defines the observed state of KalypsoApplication
type KalypsoApplicationStatus struct {
	// Phase represents the current phase of the application: Pending, Ready, Failed
	// +optional
	Phase ApplicationPhase `json:"phase,omitempty"`

	// ActiveModels is the count of active models under this application
	// +optional
	ActiveModels int `json:"activeModels,omitempty"`

	// GatewayEndpoint is the Istio Gateway endpoint URL
	// +optional
	GatewayEndpoint string `json:"gatewayEndpoint,omitempty"`

	// Conditions represent the current state of the KalypsoApplication resource
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Project",type=string,JSONPath=`.spec.projectRef`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Models",type=integer,JSONPath=`.status.activeModels`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// KalypsoApplication is the Schema for the kalypsoapplications API
// It serves as the entry point and router for traffic through Istio Gateway
type KalypsoApplication struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of KalypsoApplication
	// +required
	Spec KalypsoApplicationSpec `json:"spec"`

	// status defines the observed state of KalypsoApplication
	// +optional
	Status KalypsoApplicationStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// KalypsoApplicationList contains a list of KalypsoApplication
type KalypsoApplicationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []KalypsoApplication `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KalypsoApplication{}, &KalypsoApplicationList{})
}
