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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ReportPeriodCron    ReportPeriod = "cron"
	ReportPeriodHourly  ReportPeriod = "hourly"
	ReportPeriodDaily   ReportPeriod = "daily"
	ReportPeriodWeekly  ReportPeriod = "weekly"
	ReportPeriodMonthly ReportPeriod = "monthly"
)

type ReportPeriod string

type ReportSchedule struct {
	Period ReportPeriod `json:"period"`

	Cron    *ReportScheduleCron    `json:"cron,omitempty"`
	Hourly  *ReportScheduleHourly  `json:"hourly,omitempty"`
	Daily   *ReportScheduleDaily   `json:"daily,omitempty"`
	Weekly  *ReportScheduleWeekly  `json:"weekly,omitempty"`
	Monthly *ReportScheduleMonthly `json:"monthly,omitempty"`
}

type ReportScheduleCron struct {
	Expression string `json:"expression,omitempty"`
}

type ReportScheduleHourly struct {
	Minute int64 `json:"minute,omitempty"`
	Second int64 `json:"second,omitempty"`
}

type ReportScheduleDaily struct {
	Hour   int64 `json:"hour,omitempty"`
	Minute int64 `json:"minute,omitempty"`
	Second int64 `json:"second,omitempty"`
}

type ReportScheduleWeekly struct {
	DayOfWeek *string `json:"dayOfWeek,omitempty"`
	Hour      int64   `json:"hour,omitempty"`
	Minute    int64   `json:"minute,omitempty"`
	Second    int64   `json:"second,omitempty"`
}

type ReportScheduleMonthly struct {
	DayOfMonth *int64 `json:"dayOfMonth,omitempty"`
	Hour       int64  `json:"hour,omitempty"`
	Minute     int64  `json:"minute,omitempty"`
	Second     int64  `json:"second,omitempty"`
}

// ReportSpec defines the desired state of Report
type ReportSpec struct {
	Schedule       *ReportSchedule `json:"schedule,omitempty"`
	ReportingEnd   *metav1.Time    `json:"reportingEnd"`
	ReportingStart *metav1.Time    `json:"reportingStart"`
	Namespace      string          `json:"namespace"`
}

// ReportStatus defines the observed state of Report
type ReportStatus struct {
	LastReportTime *metav1.Time `json:"lastReportTime,omitempty"`
	NextReportTime *metav1.Time `json:"nextReportTime,omitempty"`
	Conditions     string       `json:"conditions,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Report is the Schema for the reports API
type Report struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ReportSpec   `json:"spec,omitempty"`
	Status ReportStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ReportList contains a list of Report
type ReportList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Report `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Report{}, &ReportList{})
}
