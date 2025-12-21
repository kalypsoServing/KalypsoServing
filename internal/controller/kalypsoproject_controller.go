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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	servingv1alpha1 "github.com/kalypsoServing/KalypsoServing/api/v1alpha1"
)

const (
	// ProjectLabelKey is the label key for project identification
	ProjectLabelKey = "kalypso-serving.io/project"
	// EnvironmentLabelKey is the label key for environment identification
	EnvironmentLabelKey = "kalypso-serving.io/environment"
	// ManagedByLabelKey is the label key for managed-by identification
	ManagedByLabelKey = "app.kubernetes.io/managed-by"
	// ManagedByLabelValue is the label value for managed-by
	ManagedByLabelValue = "kalypso-serving"
	// FinalizerName is the finalizer name for KalypsoProject
	FinalizerName = "serving.kalypso.io/finalizer"
)

// KalypsoProjectReconciler reconciles a KalypsoProject object
type KalypsoProjectReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=serving.serving.kalypso.io,resources=kalypsoprojects,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=serving.serving.kalypso.io,resources=kalypsoprojects/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=serving.serving.kalypso.io,resources=kalypsoprojects/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=resourcequotas,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=limitranges,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *KalypsoProjectReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Fetch the KalypsoProject instance
	project := &servingv1alpha1.KalypsoProject{}
	if err := r.Get(ctx, req.NamespacedName, project); err != nil {
		if errors.IsNotFound(err) {
			log.Info("KalypsoProject resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get KalypsoProject")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !project.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, project)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(project, FinalizerName) {
		controllerutil.AddFinalizer(project, FinalizerName)
		if err := r.Update(ctx, project); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Set initial status
	if project.Status.Phase == "" {
		project.Status.Phase = servingv1alpha1.ProjectPhaseProvisioning
		if err := r.Status().Update(ctx, project); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Reconcile namespaces for each environment
	createdNamespaces := []string{}
	for envName, envSpec := range project.Spec.Environments {
		nsName := envSpec.Namespace
		if nsName == "" {
			nsName = fmt.Sprintf("%s-%s", project.Name, envName)
		}

		// Reconcile namespace
		if err := r.reconcileNamespace(ctx, project, envName, nsName); err != nil {
			log.Error(err, "Failed to reconcile namespace", "namespace", nsName)
			r.setFailedStatus(ctx, project, fmt.Sprintf("Failed to create namespace %s: %v", nsName, err))
			return ctrl.Result{}, err
		}

		// Reconcile ResourceQuota if specified
		if envSpec.ResourceQuota != nil {
			if err := r.reconcileResourceQuota(ctx, project, envName, nsName, envSpec.ResourceQuota); err != nil {
				log.Error(err, "Failed to reconcile ResourceQuota", "namespace", nsName)
				r.setFailedStatus(ctx, project, fmt.Sprintf("Failed to create ResourceQuota in %s: %v", nsName, err))
				return ctrl.Result{}, err
			}
		}

		// Reconcile LimitRange if specified
		if envSpec.LimitRange != nil {
			if err := r.reconcileLimitRange(ctx, project, envName, nsName, envSpec.LimitRange); err != nil {
				log.Error(err, "Failed to reconcile LimitRange", "namespace", nsName)
				r.setFailedStatus(ctx, project, fmt.Sprintf("Failed to create LimitRange in %s: %v", nsName, err))
				return ctrl.Result{}, err
			}
		}

		createdNamespaces = append(createdNamespaces, nsName)
	}

	// Update status to Ready
	project.Status.Phase = servingv1alpha1.ProjectPhaseReady
	project.Status.CreatedNamespaces = createdNamespaces
	meta.SetStatusCondition(&project.Status.Conditions, metav1.Condition{
		Type:               "NamespaceCreated",
		Status:             metav1.ConditionTrue,
		Reason:             "NamespacesReady",
		Message:            fmt.Sprintf("All %d namespaces are ready", len(createdNamespaces)),
		LastTransitionTime: metav1.Now(),
	})

	if err := r.Status().Update(ctx, project); err != nil {
		return ctrl.Result{}, err
	}

	log.Info("Successfully reconciled KalypsoProject", "project", project.Name, "namespaces", createdNamespaces)
	return ctrl.Result{}, nil
}

// reconcileDelete handles the deletion of a KalypsoProject
func (r *KalypsoProjectReconciler) reconcileDelete(ctx context.Context, project *servingv1alpha1.KalypsoProject) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Delete all managed namespaces
	for _, nsName := range project.Status.CreatedNamespaces {
		ns := &corev1.Namespace{}
		if err := r.Get(ctx, client.ObjectKey{Name: nsName}, ns); err != nil {
			if errors.IsNotFound(err) {
				continue
			}
			return ctrl.Result{}, err
		}

		// Check if namespace is managed by this project
		if ns.Labels[ProjectLabelKey] == project.Name {
			log.Info("Deleting namespace", "namespace", nsName)
			if err := r.Delete(ctx, ns); err != nil && !errors.IsNotFound(err) {
				return ctrl.Result{}, err
			}
		}
	}

	// Remove finalizer
	controllerutil.RemoveFinalizer(project, FinalizerName)
	if err := r.Update(ctx, project); err != nil {
		return ctrl.Result{}, err
	}

	log.Info("Successfully deleted KalypsoProject", "project", project.Name)
	return ctrl.Result{}, nil
}

// reconcileNamespace ensures the namespace exists with proper labels
func (r *KalypsoProjectReconciler) reconcileNamespace(ctx context.Context, project *servingv1alpha1.KalypsoProject, envName, nsName string) error {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: nsName,
			Labels: map[string]string{
				ProjectLabelKey:     project.Name,
				EnvironmentLabelKey: envName,
				ManagedByLabelKey:   ManagedByLabelValue,
			},
		},
	}

	existingNs := &corev1.Namespace{}
	err := r.Get(ctx, client.ObjectKey{Name: nsName}, existingNs)
	if err != nil {
		if errors.IsNotFound(err) {
			return r.Create(ctx, ns)
		}
		return err
	}

	// Update labels if needed
	if existingNs.Labels == nil {
		existingNs.Labels = make(map[string]string)
	}
	existingNs.Labels[ProjectLabelKey] = project.Name
	existingNs.Labels[EnvironmentLabelKey] = envName
	existingNs.Labels[ManagedByLabelKey] = ManagedByLabelValue

	return r.Update(ctx, existingNs)
}

// reconcileResourceQuota ensures the ResourceQuota exists in the namespace
func (r *KalypsoProjectReconciler) reconcileResourceQuota(ctx context.Context, project *servingv1alpha1.KalypsoProject, envName, nsName string, quotaSpec *servingv1alpha1.ResourceQuotaSpec) error {
	quotaName := fmt.Sprintf("%s-quota", project.Name)

	hard := corev1.ResourceList{}
	for k, v := range quotaSpec.Limits {
		hard[k] = v
	}
	for k, v := range quotaSpec.Requests {
		reqKey := corev1.ResourceName(fmt.Sprintf("requests.%s", k))
		hard[reqKey] = v
	}

	quota := &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      quotaName,
			Namespace: nsName,
			Labels: map[string]string{
				ProjectLabelKey:     project.Name,
				EnvironmentLabelKey: envName,
				ManagedByLabelKey:   ManagedByLabelValue,
			},
		},
		Spec: corev1.ResourceQuotaSpec{
			Hard: hard,
		},
	}

	existingQuota := &corev1.ResourceQuota{}
	err := r.Get(ctx, client.ObjectKey{Name: quotaName, Namespace: nsName}, existingQuota)
	if err != nil {
		if errors.IsNotFound(err) {
			return r.Create(ctx, quota)
		}
		return err
	}

	existingQuota.Spec = quota.Spec
	return r.Update(ctx, existingQuota)
}

// reconcileLimitRange ensures the LimitRange exists in the namespace
func (r *KalypsoProjectReconciler) reconcileLimitRange(ctx context.Context, project *servingv1alpha1.KalypsoProject, envName, nsName string, limitSpec *servingv1alpha1.LimitRangeSpec) error {
	limitName := fmt.Sprintf("%s-limits", project.Name)

	limitRange := &corev1.LimitRange{
		ObjectMeta: metav1.ObjectMeta{
			Name:      limitName,
			Namespace: nsName,
			Labels: map[string]string{
				ProjectLabelKey:     project.Name,
				EnvironmentLabelKey: envName,
				ManagedByLabelKey:   ManagedByLabelValue,
			},
		},
		Spec: corev1.LimitRangeSpec{
			Limits: limitSpec.Limits,
		},
	}

	existingLimit := &corev1.LimitRange{}
	err := r.Get(ctx, client.ObjectKey{Name: limitName, Namespace: nsName}, existingLimit)
	if err != nil {
		if errors.IsNotFound(err) {
			return r.Create(ctx, limitRange)
		}
		return err
	}

	existingLimit.Spec = limitRange.Spec
	return r.Update(ctx, existingLimit)
}

// setFailedStatus updates the project status to Failed
func (r *KalypsoProjectReconciler) setFailedStatus(ctx context.Context, project *servingv1alpha1.KalypsoProject, message string) {
	project.Status.Phase = servingv1alpha1.ProjectPhaseFailed
	meta.SetStatusCondition(&project.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionFalse,
		Reason:             "ReconciliationFailed",
		Message:            message,
		LastTransitionTime: metav1.Now(),
	})
	_ = r.Status().Update(ctx, project)
}

// SetupWithManager sets up the controller with the Manager.
func (r *KalypsoProjectReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&servingv1alpha1.KalypsoProject{}).
		Owns(&corev1.Namespace{}).
		Named("kalypsoproject").
		Complete(r)
}
