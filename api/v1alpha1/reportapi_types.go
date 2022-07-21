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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ReportAPISpec defines the desired state of ReportAPI
type ReportAPISpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// ReportingEnd specifies the time this Report should end
	ReportingEnd *metav1.Time `json:"reportingEnd"`

	// ReportingStart specifies the time this Report should start from
	// This is intended for allowing a Report to start from the past
	// +optional
	ReportingStart *metav1.Time `json:"reportingStart,omitempty"`

	// Specifies how to treat concurrent executions of a Job.
	// Valid values are:
	// - "Day" (default): daily (24 hrs) report ends on ReportingEnd;
	// - "Week": weekly (7 days) report ends on ReportingEnd;
	// - "Month": monthly (30 calendar days) report ends on ReportingEnd
	// +optional
	ReportAPIPeriod ReportAPIPeriod `json:"reportPeriod,omitempty"`

	//+kubebuilder:validation:MinLength=0
	// +optional

	Namespace string `json:"namespace,omitempty"`

	// Show report aforementioned metrics only
	// +kubebuilder:validation:MaxItems=11
	// +kubebuilder:validation:MinItems=1
	// +optional
	MetricsName []MetricsName `json:"metricsName,omitempty"`
}

// Only one of the following choice may be specified.
// If none of the following policies is specified, the default one
// is DailyReport.
// +kubebuilder:validation:Enum=Day;Week;Month
type ReportAPIPeriod string

// +kubebuilder:validation:Enum=pod;pod_usage_memory_byte_seconds;pod_request_cpu_core_seconds;pod_limit_cpu_core_seconds;pod_usage_memory_byte_seconds;pod_request_memory_byte_seconds;node_capacity_cpu_cores;node_capacity_cpu_core_seconds;node_capacity_memory_bytes;node_capacity_memory_byte_seconds;pod_limit_memory_byte_seconds;node_capacity_cpu_cores;
type MetricsName string

const (
	// AllowConcurrent allows CronJobs to run concurrently.
	Daily ReportAPIPeriod = "Day"

	// ForbidConcurrent forbids concurrent runs, skipping next run if previous
	// hasn't finished yet.
	Weekly ReportAPIPeriod = "Week"

	// ReplaceConcurrent cancels currently running job and replaces it with a new one.
	Monthly ReportAPIPeriod = "Month"
)

// ReportAPIStatus defines the observed state of ReportAPI
type ReportAPIStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ReportAPI is the Schema for the reportapis API
type ReportAPI struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ReportAPISpec   `json:"spec,omitempty"`
	Status ReportAPIStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ReportAPIList contains a list of ReportAPI
type ReportAPIList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ReportAPI `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ReportAPI{}, &ReportAPIList{})
}
