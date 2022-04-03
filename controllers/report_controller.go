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
	"database/sql"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/leihchen/curator-operator/db"
	"os"

	"github.com/robfig/cron"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	log "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"strings"
	"time"

	curatorv1alpha1 "github.com/operate-first/curator-operator/api/v1alpha1"
)

const (
	postgresURL = "host=postgresql.test-project.svc  port=5432 user=curator password=M5rBgWkN8LfjeyI8 dbname=curatordb sslmode=disable"
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

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *ReportReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	os.Setenv("TZ", "UTC")
	l := log.FromContext(ctx)
	l.Info("reconciling report", "req", req.NamespacedName)
	defer l.Info("finished reconciling report", "req", req.NamespacedName)

	report := &curatorv1alpha1.Report{}
	//report := reportOriginal.DeepCopy()
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
	now := time.Now().UTC()
	reportPeriod, err := getReportPeriod(now, l, report)
	if err != nil {
		return ctrl.Result{}, nil
	}
	//l.Info("reconciling report", "periodStart", reportPeriod.periodStart)
	//l.Info("reconciling report", "now", now)
	//l.Info("reconciling report", "periodEnd", reportPeriod.periodEnd)
	//l.Info("reconciling report", "waits for", reportPeriod.periodEnd.Sub(now))
	if reportPeriod.periodEnd.After(now) { // @fixme
		return ctrl.Result{RequeueAfter: reportPeriod.periodEnd.Sub(now)}, nil
	}
	//l.Info("reconciling report", "TakingEffect", req.NamespacedName)
	postgreQueryer, err := sql.Open("postgres", postgresURL)
	if err != nil {
		panic(err.Error())
	}
	defer postgreQueryer.Close()
	results, err := ExecuteSelect(postgreQueryer, "SELECT * FROM logs_1")
	for result, _ := range results {
		l.Info("reconciling report", "NewReport", result)
	}
	report.Status.LastReportTime = &metav1.Time{Time: reportPeriod.periodEnd}
	if err := r.Status().Update(ctx, report); err != nil {
		l.Info("reconciling report", "Update Err", err)
		return ctrl.Result{}, err
	}
	if report.Spec.Schedule != nil {
		reportSchedule, err := getSchedule(report.Spec.Schedule)
		if err != nil {
			return ctrl.Result{}, err // @fixme empty results ?
		}

		nextReportPeriod := getNextReportPeriod(reportSchedule, report.Status.LastReportTime.Time)

		// update the NextReportTime on the report status
		report.Status.NextReportTime = &metav1.Time{Time: nextReportPeriod.periodEnd}
		now = time.Now().UTC()
		nextRunTime := nextReportPeriod.periodEnd
		waitTime := nextRunTime.Sub(now)
		//l.Info("reconciling report", "waits for * 2", waitTime)
		if err := r.Status().Update(ctx, report); err != nil {
			l.Info("reconciling report", "Update Err", err)
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: waitTime}, nil
	}
	return ctrl.Result{}, nil // @fixme empty results ?
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

type reportSchedule interface {
	// Return the next activation time, later than the given time.
	// Next is invoked initially, and then each time the job runs..
	Next(time.Time) time.Time
}

type reportPeriod struct {
	periodEnd   time.Time
	periodStart time.Time
}

func getReportPeriod(now time.Time, logger logr.Logger, report *curatorv1alpha1.Report) (*reportPeriod, error) {
	var reportPeriod *reportPeriod

	// check if the report's schedule spec is set
	if report.Spec.Schedule != nil {
		reportSchedule, err := getSchedule(report.Spec.Schedule)
		if err != nil {
			return nil, err
		}

		if report.Status.LastReportTime != nil {
			reportPeriod = getNextReportPeriod(reportSchedule, report.Status.LastReportTime.Time)
		} else {
			if report.Spec.ReportingStart != nil {
				logger.Info(fmt.Sprintf("no last report time for report, using spec.reportingStart %s as starting point", report.Spec.ReportingStart.Time))
				reportPeriod = getNextReportPeriod(reportSchedule, report.Spec.ReportingStart.Time)
			} else if report.Status.NextReportTime != nil {
				logger.Info(fmt.Sprintf("no last report time for report, using status.nextReportTime %s as starting point", report.Status.NextReportTime.Time))
				reportPeriod = getNextReportPeriod(reportSchedule, report.Status.NextReportTime.Time)
			} else {
				// the current period, [now, nextScheduledTime]
				currentPeriod := getNextReportPeriod(reportSchedule, now)
				// the next full report period from [nextScheduledTime, nextScheduledTime+1]
				reportPeriod = getNextReportPeriod(reportSchedule, currentPeriod.periodEnd)
				report.Status.NextReportTime = &metav1.Time{Time: reportPeriod.periodStart}
			}
		}
	} else {
		var err error
		// if there's the Spec.Schedule field is unset, then the report must be a run-once report
		reportPeriod, err = getRunOnceReportPeriod(report)
		if err != nil {
			return nil, err
		}
	}

	if reportPeriod.periodStart.After(reportPeriod.periodEnd) {
		return nil, fmt.Errorf("periodStart should never come after periodEnd")
	}

	if report.Spec.ReportingEnd != nil && reportPeriod.periodEnd.After(report.Spec.ReportingEnd.Time) {
		//logger.Debugf("calculated Report periodEnd %s goes beyond spec.reportingEnd %s, setting periodEnd to reportingEnd", reportPeriod.periodEnd, report.Spec.ReportingEnd.Time)
		// we need to truncate the reportPeriod to align with the reportingEnd
		reportPeriod.periodEnd = report.Spec.ReportingEnd.Time
	}

	return reportPeriod, nil
}

func getNextReportPeriod(schedule reportSchedule, lastScheduled time.Time) *reportPeriod {
	periodStart := lastScheduled.UTC()
	periodEnd := schedule.Next(periodStart)
	return &reportPeriod{
		periodStart: periodStart.Truncate(time.Millisecond).UTC(),
		periodEnd:   periodEnd.Truncate(time.Millisecond).UTC(),
	}
}

func getSchedule(reportSched *curatorv1alpha1.ReportSchedule) (reportSchedule, error) {
	var cronSpec string
	switch reportSched.Period {
	case curatorv1alpha1.ReportPeriodCron:
		if reportSched.Cron == nil || reportSched.Cron.Expression == "" {
			return nil, fmt.Errorf("spec.schedule.cron.expression must be specified")
		}
		return cron.ParseStandard(reportSched.Cron.Expression)
	case curatorv1alpha1.ReportPeriodHourly:
		sched := reportSched.Hourly
		if sched == nil {
			sched = &curatorv1alpha1.ReportScheduleHourly{}
		}
		if err := validateMinute(sched.Minute); err != nil {
			return nil, err
		}
		if err := validateSecond(sched.Second); err != nil {
			return nil, err
		}
		cronSpec = fmt.Sprintf("%d %d * * * *", sched.Second, sched.Minute)
	case curatorv1alpha1.ReportPeriodDaily:
		sched := reportSched.Daily
		if sched == nil {
			sched = &curatorv1alpha1.ReportScheduleDaily{}
		}
		if err := validateHour(sched.Hour); err != nil {
			return nil, err
		}
		if err := validateMinute(sched.Minute); err != nil {
			return nil, err
		}
		if err := validateSecond(sched.Second); err != nil {
			return nil, err
		}
		cronSpec = fmt.Sprintf("%d %d %d * * *", sched.Second, sched.Minute, sched.Hour)
	case curatorv1alpha1.ReportPeriodWeekly:
		sched := reportSched.Weekly
		if sched == nil {
			sched = &curatorv1alpha1.ReportScheduleWeekly{}
		}
		dow := 0
		if sched.DayOfWeek != nil {
			var err error
			dow, err = convertDayOfWeek(*sched.DayOfWeek)
			if err != nil {
				return nil, err
			}
		}
		if err := validateHour(sched.Hour); err != nil {
			return nil, err
		}
		if err := validateMinute(sched.Minute); err != nil {
			return nil, err
		}
		if err := validateSecond(sched.Second); err != nil {
			return nil, err
		}
		cronSpec = fmt.Sprintf("%d %d %d * * %d", sched.Second, sched.Minute, sched.Hour, dow)
	case curatorv1alpha1.ReportPeriodMonthly:
		sched := reportSched.Monthly
		if sched == nil {
			sched = &curatorv1alpha1.ReportScheduleMonthly{}
		}
		dom := int64(1)
		if sched.DayOfMonth != nil {
			dom = *sched.DayOfMonth
		}
		if err := validateDayOfMonth(dom); err != nil {
			return nil, err
		}
		if err := validateHour(sched.Hour); err != nil {
			return nil, err
		}
		if err := validateMinute(sched.Minute); err != nil {
			return nil, err
		}
		if err := validateSecond(sched.Second); err != nil {
			return nil, err
		}
		cronSpec = fmt.Sprintf("%d %d %d %d * *", sched.Second, sched.Minute, sched.Hour, dom)
	default:
		return nil, fmt.Errorf("invalid Report.spec.schedule.period: %s", reportSched.Period)
	}
	return cron.Parse(cronSpec)
}

func getRunOnceReportPeriod(report *curatorv1alpha1.Report) (*reportPeriod, error) {
	if report.Spec.ReportingEnd == nil || report.Spec.ReportingStart == nil {
		return nil, fmt.Errorf("run-once reports must have both ReportingEnd and ReportingStart")
	}
	reportPeriod := &reportPeriod{
		periodStart: report.Spec.ReportingStart.UTC(),
		periodEnd:   report.Spec.ReportingEnd.UTC(),
	}
	return reportPeriod, nil
}

func validateHour(hour int64) error {
	if hour >= 0 && hour <= 23 {
		return nil
	}
	return fmt.Errorf("invalid hour: %d, must be between 0 and 23", hour)
}

func validateMinute(minute int64) error {
	if minute >= 0 && minute <= 59 {
		return nil
	}
	return fmt.Errorf("invalid minute: %d, must be between 0 and 59", minute)
}

func validateSecond(second int64) error {
	if second >= 0 && second <= 59 {
		return nil
	}
	return fmt.Errorf("invalid second: %d, must be between 0 and 59", second)
}

func validateDayOfMonth(dom int64) error {
	if dom >= 1 && dom <= 31 {
		return nil
	}
	return fmt.Errorf("invalid day of month: %d, must be between 1 and 31", dom)
}

func convertDayOfWeek(dow string) (int, error) {
	switch strings.ToLower(dow) {
	case "sun", "sunday":
		return 0, nil
	case "mon", "monday":
		return 1, nil
	case "tue", "tues", "tuesday":
		return 2, nil
	case "wed", "weds", "wednesday":
		return 3, nil
	case "thur", "thurs", "thursday":
		return 4, nil
	case "fri", "friday":
		return 5, nil
	case "sat", "saturday":
		return 6, nil
	}
	return 0, fmt.Errorf("invalid day of week: %s", dow)
}

func ExecuteSelect(queryer db.Queryer, query string) ([]Row, error) {
	rows, err := queryer.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var results []Row
	for rows.Next() {
		// Create a slice of interface{}'s to represent each column,
		// and a second slice to contain pointers to each item in the columns slice.
		columns := make([]interface{}, len(cols))
		columnPointers := make([]interface{}, len(cols))
		for i := range columns {
			columnPointers[i] = &columns[i]
		}

		// Scan the result into the column pointers...
		if err := rows.Scan(columnPointers...); err != nil {
			return nil, err
		}

		// Create our map, and retrieve the value for each column from the pointers slice,
		// storing it in the map with the name of the column as the key.
		m := make(map[string]interface{})
		for i, colName := range cols {
			val := columnPointers[i].(*interface{})
			m[colName] = *val
		}
		results = append(results, Row(m))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

type Row map[string]interface{}
