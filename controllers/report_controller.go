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

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *ReportReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx)
	l.Info("reconciling request", "req", req.NamespacedName)
	defer l.Info("finished reconciling req")

	report := &curatorv1alpha1.Report{}
	if err := r.Get(ctx, req.NamespacedName, report); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if err := r.reconcileCronJob(ctx, report); err != nil {
		l.Error(err, "failed to create the CronJob resource")
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *ReportReconciler) reconcileCronJob(ctx context.Context, d *curatorv1alpha1.Report) error {
	l := log.FromContext(ctx)

	cronJob := &batchv1.CronJob{}
	if err := r.Get(ctx, types.NamespacedName{Name: d.Name, Namespace: d.Namespace}, cronJob); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		// TODO(tflannag): Gracefully handle apierrors.IsAlreadyExists(err) from the r.Create call.
		l.Info("generating a new cronjob for report", "name", d.Name, "namespace", d.Namespace)
		cronJob = newCronJobFromReport(d, r.Scheme)
		return r.Create(ctx, cronJob)
	}

	// TODO(tflannag): Support updating an existing CronJob resource?
	l.Info("cronjob already exists -- not creating another one")
	return nil
}

func newCronJobFromReport(d *curatorv1alpha1.Report, scheme *runtime.Scheme) *batchv1.CronJob {
	// TODO(tflannag): Add owner references for this generated CronJob resource where the
	// parent is the parameter's Report resource.
	log.Log.Info(scheme.Name())

	var reportPeriod string
	var scheduleOfReport string

	if d.Spec.ReportFrequency == "day" {
		reportPeriod = "SELECT generate_report('day');"
		scheduleOfReport = "05 0 * * *"
	} else if d.Spec.ReportFrequency == "week" {
		reportPeriod = "SELECT generate_report('week');"
		scheduleOfReport = "05 0 * * 0"
	} else {
		reportPeriod = "SELECT generate_report('month');"
		scheduleOfReport = "05 0 1 * *"
	}
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
			Schedule:          scheduleOfReport,
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
											Name:  "PGDATABASE",
											Value: d.Spec.DatabaseName,
										},
										{
											Name:  "PGUSER",
											Value: d.Spec.DatabaseUser,
										},
										{
											Name:  "PGPASSWORD",
											Value: d.Spec.DatabasePassword,
										},
										{
											Name:  "PGHOST",
											Value: d.Spec.DatabaseHostName,
										},
										{
											Name:  "PGPORT",
											Value: d.Spec.DatabasePort,
										},
									},
									Command: []string{"psql", "-c", reportPeriod},
									Args: []string{
										"",
									},
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
	// TODO(tflannag): watches CronJob resources that contains an owner reference to a Report resource
	return ctrl.NewControllerManagedBy(mgr).
		For(&curatorv1alpha1.Report{}).
		Complete(r)
}
