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

package controller

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	servingv1alpha1 "github.com/kalypsoServing/KalypsoServing/api/v1alpha1"
)

const (
	// TritonServerFinalizerName is the finalizer name for KalypsoTritonServer
	TritonServerFinalizerName = "serving.kalypso.io/tritonserver-finalizer"
	// TritonServerLabelKey is the label key for triton server identification
	TritonServerLabelKey = "kalypso-serving.io/tritonserver"
)

// KalypsoTritonServerReconciler reconciles a KalypsoTritonServer object
type KalypsoTritonServerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=serving.serving.kalypso.io,resources=kalypsotritonservers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=serving.serving.kalypso.io,resources=kalypsotritonservers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=serving.serving.kalypso.io,resources=kalypsotritonservers/finalizers,verbs=update
// +kubebuilder:rbac:groups=serving.serving.kalypso.io,resources=kalypsotritonservers/scale,verbs=get;update;patch
// +kubebuilder:rbac:groups=serving.serving.kalypso.io,resources=kalypsoapplications,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *KalypsoTritonServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Fetch the KalypsoTritonServer instance
	server := &servingv1alpha1.KalypsoTritonServer{}
	if err := r.Get(ctx, req.NamespacedName, server); err != nil {
		if errors.IsNotFound(err) {
			log.Info("KalypsoTritonServer resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get KalypsoTritonServer")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !server.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, server)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(server, TritonServerFinalizerName) {
		controllerutil.AddFinalizer(server, TritonServerFinalizerName)
		if err := r.Update(ctx, server); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Set initial status
	if server.Status.Phase == "" {
		server.Status.Phase = servingv1alpha1.TritonServerPhasePending
		if err := r.Status().Update(ctx, server); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Validate applicationRef existence
	app := &servingv1alpha1.KalypsoApplication{}
	appKey := types.NamespacedName{
		Name:      server.Spec.ApplicationRef,
		Namespace: server.Namespace,
	}
	if err := r.Get(ctx, appKey, app); err != nil {
		if errors.IsNotFound(err) {
			log.Error(err, "Referenced KalypsoApplication not found", "applicationRef", server.Spec.ApplicationRef)
			r.setFailedStatus(ctx, server, fmt.Sprintf("KalypsoApplication '%s' not found", server.Spec.ApplicationRef))
			return ctrl.Result{RequeueAfter: 30000000000}, nil // 30 seconds
		}
		return ctrl.Result{}, err
	}

	// Reconcile Deployment
	deploymentName := fmt.Sprintf("%s-deploy", server.Name)
	if err := r.reconcileDeployment(ctx, server, app, deploymentName); err != nil {
		log.Error(err, "Failed to reconcile Deployment")
		r.setFailedStatus(ctx, server, fmt.Sprintf("Failed to reconcile Deployment: %v", err))
		return ctrl.Result{}, err
	}

	// Reconcile Service
	serviceName := fmt.Sprintf("%s-svc", server.Name)
	if err := r.reconcileService(ctx, server, serviceName); err != nil {
		log.Error(err, "Failed to reconcile Service")
		r.setFailedStatus(ctx, server, fmt.Sprintf("Failed to reconcile Service: %v", err))
		return ctrl.Result{}, err
	}

	// Get Deployment status
	deployment := &appsv1.Deployment{}
	if err := r.Get(ctx, types.NamespacedName{Name: deploymentName, Namespace: server.Namespace}, deployment); err != nil {
		return ctrl.Result{}, err
	}

	// Re-fetch the server to get the latest version before updating status
	if err := r.Get(ctx, req.NamespacedName, server); err != nil {
		return ctrl.Result{}, err
	}

	// Update status
	httpPort := int32(8000)
	if server.Spec.Networking != nil && server.Spec.Networking.HttpPort != nil {
		httpPort = *server.Spec.Networking.HttpPort
	}

	server.Status.DeploymentName = deploymentName
	server.Status.AvailableReplicas = deployment.Status.AvailableReplicas
	server.Status.ServiceEndpoint = fmt.Sprintf("http://%s.%s.svc:%d", serviceName, server.Namespace, httpPort)

	if deployment.Status.AvailableReplicas > 0 {
		server.Status.Phase = servingv1alpha1.TritonServerPhaseRunning
		server.Status.Message = "Triton Server is ready to serve inference."
		meta.SetStatusCondition(&server.Status.Conditions, metav1.Condition{
			Type:               "Available",
			Status:             metav1.ConditionTrue,
			Reason:             "DeploymentReady",
			Message:            fmt.Sprintf("Deployment has %d available replicas", deployment.Status.AvailableReplicas),
			LastTransitionTime: metav1.Now(),
		})
	} else {
		server.Status.Phase = servingv1alpha1.TritonServerPhasePending
		server.Status.Message = "Waiting for Triton Server to become ready."
		meta.SetStatusCondition(&server.Status.Conditions, metav1.Condition{
			Type:               "Available",
			Status:             metav1.ConditionFalse,
			Reason:             "DeploymentNotReady",
			Message:            "Deployment has no available replicas",
			LastTransitionTime: metav1.Now(),
		})
	}

	if err := r.Status().Update(ctx, server); err != nil {
		if errors.IsConflict(err) {
			// Conflict error - requeue to retry
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, err
	}

	log.Info("Successfully reconciled KalypsoTritonServer",
		"server", server.Name,
		"deployment", deploymentName,
		"availableReplicas", deployment.Status.AvailableReplicas)

	return ctrl.Result{}, nil
}

// reconcileDelete handles the deletion of a KalypsoTritonServer
func (r *KalypsoTritonServerReconciler) reconcileDelete(ctx context.Context, server *servingv1alpha1.KalypsoTritonServer) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Deployment and Service will be garbage collected via OwnerReferences

	// Remove finalizer
	controllerutil.RemoveFinalizer(server, TritonServerFinalizerName)
	if err := r.Update(ctx, server); err != nil {
		if errors.IsConflict(err) {
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, err
	}

	log.Info("Successfully deleted KalypsoTritonServer", "server", server.Name)
	return ctrl.Result{}, nil
}

// reconcileDeployment ensures the Deployment exists with proper configuration
func (r *KalypsoTritonServerReconciler) reconcileDeployment(ctx context.Context, server *servingv1alpha1.KalypsoTritonServer, app *servingv1alpha1.KalypsoApplication, deploymentName string) error {
	replicas := int32(1)
	if server.Spec.Replicas != nil {
		replicas = *server.Spec.Replicas
	}

	image := "nvcr.io/nvidia/tritonserver"
	if server.Spec.TritonConfig.Image != "" {
		image = server.Spec.TritonConfig.Image
	}

	tag := "24.12-py3"
	if server.Spec.TritonConfig.Tag != "" {
		tag = server.Spec.TritonConfig.Tag
	}

	// Build container args
	args := []string{
		"tritonserver",
		fmt.Sprintf("--model-repository=%s", server.Spec.StorageUri),
	}

	for _, param := range server.Spec.TritonConfig.Parameters {
		args = append(args, fmt.Sprintf("--%s=%s", param.Name, param.Value))
	}

	// Build ports
	httpPort := int32(8000)
	grpcPort := int32(8001)
	metricsPort := int32(8002)

	if server.Spec.Networking != nil {
		if server.Spec.Networking.HttpPort != nil {
			httpPort = *server.Spec.Networking.HttpPort
		}
		if server.Spec.Networking.GrpcPort != nil {
			grpcPort = *server.Spec.Networking.GrpcPort
		}
		if server.Spec.Networking.MetricsPort != nil {
			metricsPort = *server.Spec.Networking.MetricsPort
		}
	}

	labels := map[string]string{
		TritonServerLabelKey: server.Name,
		ApplicationLabelKey:  server.Spec.ApplicationRef,
		ManagedByLabelKey:    ManagedByLabelValue,
	}

	// Build environment variables from Application storage config
	var envVars []corev1.EnvVar
	var envFrom []corev1.EnvFromSource

	if app.Spec.Storage != nil {
		// Add secret reference for S3 credentials
		if app.Spec.Storage.SecretName != "" {
			envFrom = append(envFrom, corev1.EnvFromSource{
				SecretRef: &corev1.SecretEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: app.Spec.Storage.SecretName,
					},
				},
			})
		}

		// Add S3 endpoint for MinIO or other S3-compatible storage
		if app.Spec.Storage.Endpoint != "" {
			envVars = append(envVars, corev1.EnvVar{
				Name:  "AWS_ENDPOINT_URL",
				Value: app.Spec.Storage.Endpoint,
			})
			// Also set S3_ENDPOINT for compatibility
			envVars = append(envVars, corev1.EnvVar{
				Name:  "S3_ENDPOINT",
				Value: app.Spec.Storage.Endpoint,
			})
		}

		// Add region if specified
		if app.Spec.Storage.Region != "" {
			envVars = append(envVars, corev1.EnvVar{
				Name:  "AWS_DEFAULT_REGION",
				Value: app.Spec.Storage.Region,
			})
		}
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: server.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, deployment, func() error {
		// Set labels
		if deployment.Labels == nil {
			deployment.Labels = make(map[string]string)
		}
		for k, v := range labels {
			deployment.Labels[k] = v
		}

		// Set spec
		deployment.Spec.Replicas = &replicas
		deployment.Spec.Selector = &metav1.LabelSelector{
			MatchLabels: labels,
		}
		deployment.Spec.Template = corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: labels,
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:    "tritonserver",
						Image:   fmt.Sprintf("%s:%s", image, tag),
						Args:    args,
						Env:     envVars,
						EnvFrom: envFrom,
						Ports: []corev1.ContainerPort{
							{Name: "http", ContainerPort: httpPort, Protocol: corev1.ProtocolTCP},
							{Name: "grpc", ContainerPort: grpcPort, Protocol: corev1.ProtocolTCP},
							{Name: "metrics", ContainerPort: metricsPort, Protocol: corev1.ProtocolTCP},
						},
						ReadinessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path: "/v2/health/ready",
									Port: intstr.FromInt(int(httpPort)),
								},
							},
							InitialDelaySeconds: 10,
							PeriodSeconds:       5,
						},
						LivenessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path: "/v2/health/live",
									Port: intstr.FromInt(int(httpPort)),
								},
							},
							InitialDelaySeconds: 15,
							PeriodSeconds:       10,
						},
					},
				},
			},
		}

		// Set resources if specified
		if server.Spec.Resources != nil {
			deployment.Spec.Template.Spec.Containers[0].Resources = *server.Spec.Resources
		}

		// Set owner reference
		return controllerutil.SetControllerReference(server, deployment, r.Scheme)
	})

	return err
}

// reconcileService ensures the Service exists with proper configuration
func (r *KalypsoTritonServerReconciler) reconcileService(ctx context.Context, server *servingv1alpha1.KalypsoTritonServer, serviceName string) error {
	httpPort := int32(8000)
	grpcPort := int32(8001)
	metricsPort := int32(8002)

	if server.Spec.Networking != nil {
		if server.Spec.Networking.HttpPort != nil {
			httpPort = *server.Spec.Networking.HttpPort
		}
		if server.Spec.Networking.GrpcPort != nil {
			grpcPort = *server.Spec.Networking.GrpcPort
		}
		if server.Spec.Networking.MetricsPort != nil {
			metricsPort = *server.Spec.Networking.MetricsPort
		}
	}

	labels := map[string]string{
		TritonServerLabelKey: server.Name,
		ApplicationLabelKey:  server.Spec.ApplicationRef,
		ManagedByLabelKey:    ManagedByLabelValue,
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: server.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, service, func() error {
		// Set labels
		if service.Labels == nil {
			service.Labels = make(map[string]string)
		}
		for k, v := range labels {
			service.Labels[k] = v
		}

		// Set spec (preserve ClusterIP if already set)
		service.Spec.Selector = map[string]string{
			TritonServerLabelKey: server.Name,
		}
		service.Spec.Ports = []corev1.ServicePort{
			{
				Name:       "http",
				Port:       httpPort,
				TargetPort: intstr.FromString("http"),
				Protocol:   corev1.ProtocolTCP,
			},
			{
				Name:       "grpc",
				Port:       grpcPort,
				TargetPort: intstr.FromString("grpc"),
				Protocol:   corev1.ProtocolTCP,
			},
			{
				Name:       "metrics",
				Port:       metricsPort,
				TargetPort: intstr.FromString("metrics"),
				Protocol:   corev1.ProtocolTCP,
			},
		}
		service.Spec.Type = corev1.ServiceTypeClusterIP

		// Set owner reference
		return controllerutil.SetControllerReference(server, service, r.Scheme)
	})

	return err
}

// setFailedStatus updates the server status to Failed
func (r *KalypsoTritonServerReconciler) setFailedStatus(ctx context.Context, server *servingv1alpha1.KalypsoTritonServer, message string) {
	server.Status.Phase = servingv1alpha1.TritonServerPhaseFailed
	server.Status.Message = message
	meta.SetStatusCondition(&server.Status.Conditions, metav1.Condition{
		Type:               "Available",
		Status:             metav1.ConditionFalse,
		Reason:             "ReconciliationFailed",
		Message:            message,
		LastTransitionTime: metav1.Now(),
	})
	_ = r.Status().Update(ctx, server)
}

// SetupWithManager sets up the controller with the Manager.
func (r *KalypsoTritonServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&servingv1alpha1.KalypsoTritonServer{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Named("kalypsotritonserver").
		Complete(r)
}
