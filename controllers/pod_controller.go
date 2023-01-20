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
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

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

//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete

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

	componentName := instance.Spec.Component
	appName := instance.Name + "-" + componentName

	// Check whether the monitor component deploy exists
	monitorDeployExisted, err := r.monitorComponentDeployExisted(ctx, appName, instance.Namespace)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Create the monitor component deploy if no monitor component deploy
	if !monitorDeployExisted {
		if err := r.updateInstanceStatus(ctx, instance, "Creating monitor deploy"); err != nil {
			return ctrl.Result{}, err
		}

		monitorDeploy := r.monitorComponentDeployment(appName, instance.Namespace, 1)
		if err := controllerutil.SetControllerReference(instance, monitorDeploy, r.Scheme); err != nil {
			_ = r.updateInstanceStatus(ctx, instance, "Failed to bind monitor deploy")
			logger.Error(err, fmt.Sprintf("Error while binding monitor deploy: %s", err))
			return ctrl.Result{}, err
		}
		if err := r.Client.Create(ctx, monitorDeploy); err != nil {
			_ = r.updateInstanceStatus(ctx, instance, "Failed to create monitor deploy")
			logger.Error(err, fmt.Sprintf("Error while creating monitor deploy: %s", err))
			return ctrl.Result{}, err
		}

		if err := r.updateInstanceStatus(ctx, instance, "Created monitor deploy"); err != nil {
			return ctrl.Result{}, err
		}

		return ctrl.Result{
			Requeue: true,
		}, nil
	}

	// Check whether the monitor component svc exists
	monitorSvcExisted, err := r.monitorComponentSvcExisted(ctx, appName, instance.Namespace)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Create the monitor component svc if no monitor component svc
	if !monitorSvcExisted {
		if err := r.updateInstanceStatus(ctx, instance, "Creating monitor svc"); err != nil {
			return ctrl.Result{}, err
		}

		monitorSvc := r.monitorComponentService(appName, instance.Namespace)
		if err := controllerutil.SetControllerReference(instance, monitorSvc, r.Scheme); err != nil {
			_ = r.updateInstanceStatus(ctx, instance, "Failed to bind monitor svc")
			logger.Error(err, fmt.Sprintf("Error while binding monitor svc: %s", err))
			return ctrl.Result{}, err
		}
		if err := r.Client.Create(ctx, monitorSvc); err != nil {
			_ = r.updateInstanceStatus(ctx, instance, "Failed to create monitor svc")
			logger.Error(err, fmt.Sprintf("Error while creating monitor svc: %s", err))
			return ctrl.Result{}, err
		}

		if err := r.updateInstanceStatus(ctx, instance, "Created monitor svc"); err != nil {
			return ctrl.Result{}, err
		}

		return ctrl.Result{
			Requeue: true,
		}, nil
	}

	if err := r.updateInstanceStatus(ctx, instance, "Successful"); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PodReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&monitorv1alpha1.Pod{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Complete(r)
}

func (r *PodReconciler) monitorComponentDeployExisted(ctx context.Context, appName, namespace string) (bool, error) {
	logger := log.FromContext(ctx)

	if _, err := r.getMonitorComponentDeployment(ctx, appName, namespace); err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		logger.Error(err, fmt.Sprintf("Error while getting existed monitor deploy: %s", err))
		return false, err
	}
	return true, nil
}

func (r *PodReconciler) getMonitorComponentDeployment(ctx context.Context, appName, namespace string) (*appsv1.Deployment, error) {
	logger := log.FromContext(ctx)

	existedMonitorDeploy := &appsv1.Deployment{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      appName,
		Namespace: namespace,
	}, existedMonitorDeploy); err != nil {
		logger.Error(err, fmt.Sprintf("Error while checking existed monitor deploy: %s", err))
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

func (r *PodReconciler) monitorComponentSvcExisted(ctx context.Context, appName, namespace string) (bool, error) {
	logger := log.FromContext(ctx)

	if _, err := r.getMonitorComponentService(ctx, appName, namespace); err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		logger.Error(err, fmt.Sprintf("Error while checking existed monitor svc: %s", err))
		return false, err
	}
	return true, nil
}

func (r *PodReconciler) getMonitorComponentService(ctx context.Context, appName, namespace string) (*corev1.Service, error) {
	logger := log.FromContext(ctx)

	existedMonitorSvc := &corev1.Service{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      appName + "-svc",
		Namespace: namespace,
	}, existedMonitorSvc); err != nil {
		logger.Error(err, fmt.Sprintf("Error while getting monitor svc: %s", err))
		return nil, err
	}
	return existedMonitorSvc, nil
}

func (r *PodReconciler) monitorComponentService(appName, namespace string) *corev1.Service {
	appLabels := map[string]string{"app": appName}
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      appName + "-svc",
			Namespace: namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: appLabels,
			Type:     corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					Name:       "web",
					Port:       3000,
					TargetPort: intstr.FromInt(3000),
				},
			},
		},
	}
}

func (r *PodReconciler) updateInstanceStatus(ctx context.Context, instance *monitorv1alpha1.Pod, result string) error {
	logger := log.FromContext(ctx)

	instance.Status.Result = result
	if err := r.Client.Status().Update(ctx, instance); err != nil {
		logger.Error(err, fmt.Sprintf("Error while updating instance status: %s", err))
		return err
	}
	return nil
}
