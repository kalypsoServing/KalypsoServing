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

// KalypsoTritonServerSpec defines the desired state of KalypsoTritonServer
type KalypsoTritonServerSpec struct {
	// ApplicationRef is the reference to parent KalypsoApplication
	// +kubebuilder:validation:Required
	ApplicationRef string `json:"applicationRef"`

	// StorageUri is the S3/GCS path to model repository
	// +kubebuilder:validation:Required
	StorageUri string `json:"storageUri"`

	// TritonConfig defines the Triton server configuration
	// +kubebuilder:validation:Required
	TritonConfig TritonConfigSpec `json:"tritonConfig"`

	// Replicas is the number of replicas (default: 1)
	// +optional
	// +kubebuilder:default=1
	Replicas *int32 `json:"replicas,omitempty"`

	// Resources defines K8s resource requests/limits
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// Networking defines service port configuration
	// +optional
	Networking *NetworkingSpec `json:"networking,omitempty"`
}

// TritonConfigSpec defines the Triton server configuration
type TritonConfigSpec struct {
	// Image is the Triton container image (default: nvcr.io/nvidia/tritonserver)
	// +optional
	// +kubebuilder:default="nvcr.io/nvidia/tritonserver"
	Image string `json:"image,omitempty"`

	// Tag is the image tag
	// +optional
	// +kubebuilder:default="24.12-py3"
	Tag string `json:"tag,omitempty"`

	// Parameters are Triton runtime parameters
	// +optional
	Parameters []TritonParameter `json:"parameters,omitempty"`

	// BackendType is the backend type: python, tensorflow, pytorch, etc.
	// +optional
	// +kubebuilder:validation:Enum=python;tensorflow;pytorch;onnxruntime;tensorrt
	BackendType string `json:"backendType,omitempty"`

	// PythonBackend defines Python backend specific settings
	// +optional
	PythonBackend *PythonBackendSpec `json:"python_backend,omitempty"`
}

// TritonParameter defines a Triton runtime parameter
type TritonParameter struct {
	// Name is the parameter name
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Value is the parameter value
	// +kubebuilder:validation:Required
	Value string `json:"value"`
}

// PythonBackendSpec defines Python backend specific settings
type PythonBackendSpec struct {
	// ShmDefaultByteSize is the shared memory size in bytes
	// +optional
	// +kubebuilder:default=1048576
	ShmDefaultByteSize *int64 `json:"shmDefaultByteSize,omitempty"`

	// ExtraArgs are additional args passed to model initialize()
	// +optional
	ExtraArgs map[string]string `json:"extraArgs,omitempty"`
}

// NetworkingSpec defines the service port configuration
type NetworkingSpec struct {
	// HttpPort is the HTTP port (default: 8000)
	// +optional
	// +kubebuilder:default=8000
	HttpPort *int32 `json:"httpPort,omitempty"`

	// GrpcPort is the gRPC port (default: 8001)
	// +optional
	// +kubebuilder:default=8001
	GrpcPort *int32 `json:"grpcPort,omitempty"`

	// MetricsPort is the metrics port (default: 8002)
	// +optional
	// +kubebuilder:default=8002
	MetricsPort *int32 `json:"metricsPort,omitempty"`
}

// TritonServerPhase represents the current phase of the Triton server
// +kubebuilder:validation:Enum=Pending;Running;Failed
type TritonServerPhase string

const (
	// TritonServerPhasePending indicates the server is pending
	TritonServerPhasePending TritonServerPhase = "Pending"
	// TritonServerPhaseRunning indicates the server is running
	TritonServerPhaseRunning TritonServerPhase = "Running"
	// TritonServerPhaseFailed indicates the server has failed
	TritonServerPhaseFailed TritonServerPhase = "Failed"
)

// KalypsoTritonServerStatus defines the observed state of KalypsoTritonServer
type KalypsoTritonServerStatus struct {
	// Phase represents the current phase: Pending, Running, Failed
	// +optional
	Phase TritonServerPhase `json:"phase,omitempty"`

	// DeploymentName is the name of created K8s Deployment
	// +optional
	DeploymentName string `json:"deploymentName,omitempty"`

	// ServiceEndpoint is the Service endpoint URL
	// +optional
	ServiceEndpoint string `json:"serviceEndpoint,omitempty"`

	// AvailableReplicas is the number of available replicas
	// +optional
	AvailableReplicas int32 `json:"availableReplicas,omitempty"`

	// Message is a human-readable status message
	// +optional
	Message string `json:"message,omitempty"`

	// Conditions represent the current state of the KalypsoTritonServer resource
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:subresource:scale:specpath=.spec.replicas,statuspath=.status.availableReplicas
// +kubebuilder:printcolumn:name="Application",type=string,JSONPath=`.spec.applicationRef`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Replicas",type=integer,JSONPath=`.spec.replicas`
// +kubebuilder:printcolumn:name="Available",type=integer,JSONPath=`.status.availableReplicas`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// KalypsoTritonServer is the Schema for the kalypsotritonservers API
// It deploys and manages NVIDIA Triton Inference Servers
type KalypsoTritonServer struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of KalypsoTritonServer
	// +required
	Spec KalypsoTritonServerSpec `json:"spec"`

	// status defines the observed state of KalypsoTritonServer
	// +optional
	Status KalypsoTritonServerStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// KalypsoTritonServerList contains a list of KalypsoTritonServer
type KalypsoTritonServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []KalypsoTritonServer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KalypsoTritonServer{}, &KalypsoTritonServerList{})
}
