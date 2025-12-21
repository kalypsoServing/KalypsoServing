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

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	servingv1alpha1 "github.com/kalypsoServing/KalypsoServing/api/v1alpha1"
)

const (
	// ApplicationFinalizerName is the finalizer name for KalypsoApplication
	ApplicationFinalizerName = "serving.kalypso.io/application-finalizer"
	// ApplicationLabelKey is the label key for application identification
	ApplicationLabelKey = "kalypso-serving.io/application"
)

// KalypsoApplicationReconciler reconciles a KalypsoApplication object
type KalypsoApplicationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=serving.serving.kalypso.io,resources=kalypsoapplications,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=serving.serving.kalypso.io,resources=kalypsoapplications/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=serving.serving.kalypso.io,resources=kalypsoapplications/finalizers,verbs=update
// +kubebuilder:rbac:groups=serving.serving.kalypso.io,resources=kalypsoprojects,verbs=get;list;watch
// +kubebuilder:rbac:groups=serving.serving.kalypso.io,resources=kalypsotritionservers,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *KalypsoApplicationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Fetch the KalypsoApplication instance
	app := &servingv1alpha1.KalypsoApplication{}
	if err := r.Get(ctx, req.NamespacedName, app); err != nil {
		if errors.IsNotFound(err) {
			log.Info("KalypsoApplication resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get KalypsoApplication")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !app.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, app)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(app, ApplicationFinalizerName) {
		controllerutil.AddFinalizer(app, ApplicationFinalizerName)
		if err := r.Update(ctx, app); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Set initial status
	if app.Status.Phase == "" {
		app.Status.Phase = servingv1alpha1.ApplicationPhasePending
		if err := r.Status().Update(ctx, app); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Validate projectRef existence
	project := &servingv1alpha1.KalypsoProject{}
	projectKey := types.NamespacedName{
		Name:      app.Spec.ProjectRef,
		Namespace: app.Namespace,
	}
	if err := r.Get(ctx, projectKey, project); err != nil {
		if errors.IsNotFound(err) {
			log.Error(err, "Referenced KalypsoProject not found", "projectRef", app.Spec.ProjectRef)
			r.setFailedStatus(ctx, app, fmt.Sprintf("KalypsoProject '%s' not found", app.Spec.ProjectRef))
			// Requeue after some time to check again
			return ctrl.Result{RequeueAfter: 30000000000}, nil // 30 seconds
		}
		return ctrl.Result{}, err
	}

	// Verify project is ready
	if project.Status.Phase != servingv1alpha1.ProjectPhaseReady {
		log.Info("Referenced KalypsoProject is not ready yet", "projectRef", app.Spec.ProjectRef, "phase", project.Status.Phase)
		// Re-fetch before updating status
		if err := r.Get(ctx, req.NamespacedName, app); err != nil {
			return ctrl.Result{}, err
		}
		meta.SetStatusCondition(&app.Status.Conditions, metav1.Condition{
			Type:               "ProjectReady",
			Status:             metav1.ConditionFalse,
			Reason:             "ProjectNotReady",
			Message:            fmt.Sprintf("KalypsoProject '%s' is in phase: %s", app.Spec.ProjectRef, project.Status.Phase),
			LastTransitionTime: metav1.Now(),
		})
		if err := r.Status().Update(ctx, app); err != nil {
			if errors.IsConflict(err) {
				return ctrl.Result{Requeue: true}, nil
			}
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: 10000000000}, nil // 10 seconds
	}

	// Count active TritonServers for this application
	activeModels, err := r.countActiveTritonServers(ctx, app)
	if err != nil {
		log.Error(err, "Failed to count active TritonServers")
		// Continue anyway, just log the error
	}

	// Re-fetch the app to get the latest version before updating status
	if err := r.Get(ctx, req.NamespacedName, app); err != nil {
		return ctrl.Result{}, err
	}

	// Update status to Ready
	app.Status.Phase = servingv1alpha1.ApplicationPhaseReady
	app.Status.ActiveModels = activeModels
	app.Status.GatewayEndpoint = fmt.Sprintf("http://istio-gateway.istio-system.svc/%s", app.Name)

	meta.SetStatusCondition(&app.Status.Conditions, metav1.Condition{
		Type:               "ProjectReady",
		Status:             metav1.ConditionTrue,
		Reason:             "ProjectValidated",
		Message:            fmt.Sprintf("KalypsoProject '%s' is ready", app.Spec.ProjectRef),
		LastTransitionTime: metav1.Now(),
	})

	meta.SetStatusCondition(&app.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		Reason:             "ApplicationReady",
		Message:            "KalypsoApplication is ready to serve",
		LastTransitionTime: metav1.Now(),
	})

	if err := r.Status().Update(ctx, app); err != nil {
		if errors.IsConflict(err) {
			// Conflict error - requeue to retry
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, err
	}

	log.Info("Successfully reconciled KalypsoApplication",
		"application", app.Name,
		"project", app.Spec.ProjectRef,
		"activeModels", activeModels)

	return ctrl.Result{}, nil
}

// reconcileDelete handles the deletion of a KalypsoApplication
func (r *KalypsoApplicationReconciler) reconcileDelete(ctx context.Context, app *servingv1alpha1.KalypsoApplication) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// TODO: Add cleanup logic for Istio Gateway resources if needed

	// Remove finalizer
	controllerutil.RemoveFinalizer(app, ApplicationFinalizerName)
	if err := r.Update(ctx, app); err != nil {
		if errors.IsConflict(err) {
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, err
	}

	log.Info("Successfully deleted KalypsoApplication", "application", app.Name)
	return ctrl.Result{}, nil
}

// countActiveTritonServers counts the number of TritonServers belonging to this application
func (r *KalypsoApplicationReconciler) countActiveTritonServers(ctx context.Context, app *servingv1alpha1.KalypsoApplication) (int, error) {
	tritonServers := &servingv1alpha1.KalypsoTritonServerList{}
	if err := r.List(ctx, tritonServers, client.InNamespace(app.Namespace)); err != nil {
		return 0, err
	}

	count := 0
	for _, server := range tritonServers.Items {
		if server.Spec.ApplicationRef == app.Name {
			count++
		}
	}
	return count, nil
}

// setFailedStatus updates the application status to Failed
func (r *KalypsoApplicationReconciler) setFailedStatus(ctx context.Context, app *servingv1alpha1.KalypsoApplication, message string) {
	app.Status.Phase = servingv1alpha1.ApplicationPhaseFailed
	meta.SetStatusCondition(&app.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionFalse,
		Reason:             "ReconciliationFailed",
		Message:            message,
		LastTransitionTime: metav1.Now(),
	})
	_ = r.Status().Update(ctx, app)
}

// SetupWithManager sets up the controller with the Manager.
func (r *KalypsoApplicationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&servingv1alpha1.KalypsoApplication{}).
		Named("kalypsoapplication").
		Complete(r)
}
