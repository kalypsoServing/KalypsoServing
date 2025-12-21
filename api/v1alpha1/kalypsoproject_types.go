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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// KalypsoProjectSpec defines the desired state of KalypsoProject
type KalypsoProjectSpec struct {
	// DisplayName is the human-readable project name
	// +optional
	DisplayName string `json:"displayName,omitempty"`

	// Owner is the team or user owning the project
	// +optional
	Owner string `json:"owner,omitempty"`

	// Environments defines environment-specific configurations
	// +optional
	Environments map[string]EnvironmentSpec `json:"environments,omitempty"`

	// ModelRegistry defines common model registry settings
	// +optional
	ModelRegistry *ModelRegistrySpec `json:"modelRegistry,omitempty"`
}

// EnvironmentSpec defines the configuration for a specific environment
type EnvironmentSpec struct {
	// Namespace is the target namespace name for this environment
	// +kubebuilder:validation:Required
	Namespace string `json:"namespace"`

	// Description provides a description of the environment
	// +optional
	Description string `json:"description,omitempty"`

	// LimitRange defines the K8s LimitRange configuration for the namespace
	// +optional
	LimitRange *LimitRangeSpec `json:"limitRange,omitempty"`

	// ResourceQuota defines the K8s ResourceQuota configuration for the namespace
	// +optional
	ResourceQuota *ResourceQuotaSpec `json:"resourceQuota,omitempty"`
}

// LimitRangeSpec defines the LimitRange configuration
type LimitRangeSpec struct {
	// Limits is a list of LimitRangeItem objects
	// +optional
	Limits []corev1.LimitRangeItem `json:"limits,omitempty"`
}

// ResourceQuotaSpec defines the ResourceQuota configuration
type ResourceQuotaSpec struct {
	// Limits defines the resource limits for the namespace
	// +optional
	Limits corev1.ResourceList `json:"limits,omitempty"`

	// Requests defines the resource requests for the namespace
	// +optional
	Requests corev1.ResourceList `json:"requests,omitempty"`
}

// ModelRegistrySpec defines the model registry configuration
type ModelRegistrySpec struct {
	// URL is the model registry URL (e.g., S3 path)
	// +kubebuilder:validation:Required
	URL string `json:"url"`

	// SecretRef is the reference to credentials secret
	// +optional
	SecretRef string `json:"secretRef,omitempty"`
}

// ProjectPhase represents the current phase of the project
// +kubebuilder:validation:Enum=Provisioning;Ready;Failed
type ProjectPhase string

const (
	// ProjectPhaseProvisioning indicates the project is being provisioned
	ProjectPhaseProvisioning ProjectPhase = "Provisioning"
	// ProjectPhaseReady indicates the project is ready
	ProjectPhaseReady ProjectPhase = "Ready"
	// ProjectPhaseFailed indicates the project has failed
	ProjectPhaseFailed ProjectPhase = "Failed"
)

// KalypsoProjectStatus defines the observed state of KalypsoProject
type KalypsoProjectStatus struct {
	// Phase represents the current phase of the project: Provisioning, Ready, Failed
	// +optional
	Phase ProjectPhase `json:"phase,omitempty"`

	// Conditions represent the current state of the KalypsoProject resource
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// CreatedNamespaces lists the namespaces that have been created for this project
	// +optional
	CreatedNamespaces []string `json:"createdNamespaces,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Owner",type=string,JSONPath=`.spec.owner`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// KalypsoProject is the Schema for the kalypsoprojects API
// It manages logical namespaces for ML model serving projects
type KalypsoProject struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of KalypsoProject
	// +required
	Spec KalypsoProjectSpec `json:"spec"`

	// status defines the observed state of KalypsoProject
	// +optional
	Status KalypsoProjectStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// KalypsoProjectList contains a list of KalypsoProject
type KalypsoProjectList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []KalypsoProject `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KalypsoProject{}, &KalypsoProjectList{})
}
