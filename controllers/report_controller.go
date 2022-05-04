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

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	log "sigs.k8s.io/controller-runtime/pkg/log"

	curatorv1alpha1 "github.com/operate-first/curator-operator/api/v1alpha1"
)

// ReportReconciler reconciles a Report object
type ReportReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=curator.operatefirst.io,resources=reports,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=curator.operatefirst.io,resources=reports/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=curator.operatefirst.io,resources=reports/finalizers,verbs=update
//+kubebuilder:rbac:groups=curator.operatefirst.io,resources=curatorconfigs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=curator.operatefirst.io,resources=curatorconfigs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=curator.operatefirst.io,resources=curatorconfigs/finalizers,verbs=update
//+kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=batch,resources=cronjobs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=batch,resources=cronjobs/finalizers,verbs=update
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=batch,resources=jobs/status,verbs=get

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *ReportReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx)
	fmt.Println("Enter reconcile", req.NamespacedName)
	//l.Info("reconciling report", "req", req.NamespacedName)

	l.Info("Enter reconcile", "req", req)
	Report := &curatorv1alpha1.Report{}

	//err := r.Get(ctx, types.NamespacedName, Fetchdata)

	err := r.Get(ctx, types.NamespacedName{Name: req.Name, Namespace: req.Namespace}, Report)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			l.Info("Report resource not found. Ignoring since object must be deleted.")
			return ctrl.Result{}, nil
		}
	}

	if err := r.createJob(Report); err != nil {
		l.Error(err, "failed to create the CronJob resource")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *ReportReconciler) createJob(d *curatorv1alpha1.Report) error {
	if _, err := FetchCronJob(d.Name, d.Namespace, r.Client); err != nil {
		if err := r.Client.Create(context.TODO(), ReportCronJob(d, r.Scheme)); err != nil {
			return err
		}
	}

	return nil
}

func FetchReportJob(name, namespace string, client client.Client) (*batchv1.CronJob, error) {
	cronJob := &batchv1.CronJob{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, cronJob)
	return cronJob, err
}

func ReportCronJob(d *curatorv1alpha1.Report, scheme *runtime.Scheme) *batchv1.CronJob {
	cronjob := &batchv1.CronJob{
		TypeMeta: metav1.TypeMeta{
			Kind: "Cronjob",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      d.Name,
			Namespace: d.Spec.CronjobNamespace,
		},
		Spec: batchv1.CronJobSpec{
			ConcurrencyPolicy: "Forbid",
			Schedule:          d.Spec.ScheduleForReport,
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "dayreport",
									Image: "docker.io/library/postgres:13.0",
									Env: []corev1.EnvVar{
										{
											Name:  "DATABASE_NAME",
											Value: d.Spec.DatabaseName,
										},
										{
											Name:  "DATABASE_USER",
											Value: d.Spec.DatabaseUser,
										},
										{
											Name:  "DATABASE_PASSWORD",
											Value: d.Spec.DatabasePassword,
										},
										{
											Name:  "DATABASE_HOST_NAME",
											Value: d.Spec.DatabaseHostName,
										},
										{
											Name:  "PORT_NUMBER",
											Value: d.Spec.DatabasePort,
										},
									},
									Command: []string{"python3", "echo", "Hello"},
								},
							},

							RestartPolicy: "Never",
						},
					},
				},
			},
		},
	}
	return cronjob

}

// SetupWithManager sets up the controller with the Manager.
func (r *ReportReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&curatorv1alpha1.Report{}).
		Complete(r)
}
