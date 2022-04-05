package reporting

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/robfig/cron"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	curatorv1alpha1 "github.com/operate-first/curator-operator/api/v1alpha1"
	"github.com/operate-first/curator-operator/internal/db"
)

type reportSchedule interface {
	// Return the next activation time, later than the given time.
	// Next is invoked initially, and then each time the job runs..
	Next(time.Time) time.Time
}

type ReportPeriod struct {
	PeriodEnd   time.Time
	PeriodStart time.Time
}

func GetReportPeriod(now time.Time, logger logr.Logger, report *curatorv1alpha1.Report) (*ReportPeriod, error) {
	var reportPeriod *ReportPeriod

	// check if the report's schedule spec is set
	if report.Spec.Schedule != nil {
		reportSchedule, err := GetSchedule(report.Spec.Schedule)
		if err != nil {
			return nil, err
		}

		if report.Status.LastReportTime != nil {
			reportPeriod = GetNextReportPeriod(reportSchedule, report.Status.LastReportTime.Time)
		} else {
			if report.Spec.ReportingStart != nil {
				logger.Info(fmt.Sprintf("no last report time for report, using spec.reportingStart %s as starting point", report.Spec.ReportingStart.Time))
				reportPeriod = GetNextReportPeriod(reportSchedule, report.Spec.ReportingStart.Time)
			} else if report.Status.NextReportTime != nil {
				logger.Info(fmt.Sprintf("no last report time for report, using status.nextReportTime %s as starting point", report.Status.NextReportTime.Time))
				reportPeriod = GetNextReportPeriod(reportSchedule, report.Status.NextReportTime.Time)
			} else {
				// the current period, [now, nextScheduledTime]
				currentPeriod := GetNextReportPeriod(reportSchedule, now)
				// the next full report period from [nextScheduledTime, nextScheduledTime+1]
				reportPeriod = GetNextReportPeriod(reportSchedule, currentPeriod.PeriodEnd)
				report.Status.NextReportTime = &metav1.Time{Time: reportPeriod.PeriodStart}
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

	if reportPeriod.PeriodStart.After(reportPeriod.PeriodEnd) {
		return nil, fmt.Errorf("PeriodStart should never come after PeriodEnd")
	}

	if report.Spec.ReportingEnd != nil && reportPeriod.PeriodEnd.After(report.Spec.ReportingEnd.Time) {
		//logger.Debugf("calculated Report PeriodEnd %s goes beyond spec.reportingEnd %s, setting PeriodEnd to reportingEnd", reportPeriod.PeriodEnd, report.Spec.ReportingEnd.Time)
		// we need to truncate the reportPeriod to align with the reportingEnd
		reportPeriod.PeriodEnd = report.Spec.ReportingEnd.Time
	}

	return reportPeriod, nil
}

func GetNextReportPeriod(schedule reportSchedule, lastScheduled time.Time) *ReportPeriod {
	PeriodStart := lastScheduled.UTC()
	PeriodEnd := schedule.Next(PeriodStart)
	return &ReportPeriod{
		PeriodStart: PeriodStart.Truncate(time.Millisecond).UTC(),
		PeriodEnd:   PeriodEnd.Truncate(time.Millisecond).UTC(),
	}
}

func GetSchedule(reportSched *curatorv1alpha1.ReportSchedule) (reportSchedule, error) {
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

func getRunOnceReportPeriod(report *curatorv1alpha1.Report) (*ReportPeriod, error) {
	if report.Spec.ReportingEnd == nil || report.Spec.ReportingStart == nil {
		return nil, fmt.Errorf("run-once reports must have both ReportingEnd and ReportingStart")
	}
	reportPeriod := &ReportPeriod{
		PeriodStart: report.Spec.ReportingStart.UTC(),
		PeriodEnd:   report.Spec.ReportingEnd.UTC(),
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
