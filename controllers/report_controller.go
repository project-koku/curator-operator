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

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	log "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	curatorv1alpha1 "github.com/timflannagan/curator-operator/api/v1alpha1"
)

// ReportReconciler reconciles a Report object
type ReportReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=curator.operatorfirst.io,resources=reports,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=curator.operatorfirst.io,resources=reports/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=curator.operatorfirst.io,resources=reports/finalizers,verbs=update
//+kubebuilder:rbac:groups=curator.operatorfirst.io,resources=curatorconfigs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=curator.operatorfirst.io,resources=curatorconfigs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=curator.operatorfirst.io,resources=curatorconfigs/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *ReportReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx)
	l.Info("reconciling report", "req", req.NamespacedName)
	defer l.Info("finished reconciling report", "req", req.NamespacedName)

	report := &curatorv1alpha1.Report{}
	if err := r.Client.Get(ctx, req.NamespacedName, report); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if err := r.validateReport(report); err != nil {
		// TODO(tflannag): Need to be careful about endless cycles here
		// TODO(tflannag): Fire off an Event and complain in the Report's status about validation error
		l.Error(err, "failed to validate the report")
		return ctrl.Result{}, err
	}

	if err := r.ensureReport(report); err != nil {
		l.Error(err, "failed to ensure the report")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *ReportReconciler) validateReport(report *curatorv1alpha1.Report) error {
	return nil
}

func (r *ReportReconciler) ensureReport(report *curatorv1alpha1.Report) error {
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ReportReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&curatorv1alpha1.Report{}).
		Watches(&source.Kind{Type: &curatorv1alpha1.CuratorConfig{}}, requeueReportsHandler(mgr.GetClient(), mgr.GetLogger())).
		Complete(r)
}

func requeueReportsHandler(c client.Client, log logr.Logger) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(obj client.Object) []reconcile.Request {
		reports := &curatorv1alpha1.ReportList{}
		if err := c.List(context.Background(), reports); err != nil {
			return nil
		}

		var res []reconcile.Request
		for _, report := range reports.Items {
			log.Info("requeuing report", "name", report.GetName(), "ns", report.GetNamespace())
			res = append(res, reconcile.Request{
				NamespacedName: client.ObjectKeyFromObject(&report),
			})
		}
		return res
	})
}
