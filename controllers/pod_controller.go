/*
Copyright 2022.

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

package controllers

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	monitorv1alpha1 "github.com/luoxiaojun1992/operator-learning/api/v1alpha1"
)

// PodReconciler reconciles a Pod object
type PodReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=monitor.example.com,resources=pods,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=monitor.example.com,resources=pods/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=monitor.example.com,resources=pods/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// the Pod object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.0/pkg/reconcile
func (r *PodReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	instance := &monitorv1alpha1.Pod{}
	err := r.Client.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		logger.Error(err, fmt.Sprintf("Error while getting instance: %s", err))
		return ctrl.Result{}, err
	}

	metricConfig := instance.Spec.Metric
	appName := instance.Name + "." + metricConfig

	// Check whether the monitor component exists
	monitorExisted, err := r.monitorComponentExisted(ctx, appName, instance.Namespace)
	if err != nil {
		logger.Error(err, fmt.Sprintf("Error while checking existed monitor: %s", err))
		return ctrl.Result{}, err
	}

	// Create the monitor component if no monitor component
	if !monitorExisted {
		monitorDeploy := r.monitorComponentDeployment(appName, instance.Namespace, 1)
		err = r.Client.Create(ctx, monitorDeploy)
		if err != nil {
			logger.Error(err, fmt.Sprintf("Error while creating monitor: %s", err))
			return ctrl.Result{}, err
		}
		return ctrl.Result{
			Requeue: true,
		}, nil
	}

	instance.Status.Result = "Successful"
	err = r.Client.Status().Update(ctx, instance)
	if err != nil {
		logger.Error(err, fmt.Sprintf("Error while updating instance: %s", err))
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PodReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&monitorv1alpha1.Pod{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}

func (r *PodReconciler) monitorComponentExisted(ctx context.Context, appName, namespace string) (bool, error) {
	if _, err := r.getMonitorComponentDeployment(ctx, appName, namespace); err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (r *PodReconciler) getMonitorComponentDeployment(ctx context.Context, appName, namespace string) (*appsv1.Deployment, error) {
	existedMonitorDeploy := &appsv1.Deployment{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      appName,
		Namespace: namespace,
	}, existedMonitorDeploy); err != nil {
		return nil, err
	}
	return existedMonitorDeploy, nil
}

func (r *PodReconciler) monitorComponentDeployment(appName, namespace string, replicas int32) *appsv1.Deployment {
	appLabels := map[string]string{"app": appName}
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      appName,
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: appLabels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: appLabels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "grafana",
							Image: "grafana/grafana",
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 3000,
								},
							},
						},
					},
				},
			},
		},
	}
}
