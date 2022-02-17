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

// ReportSpec defines the desired state of Report
type ReportSpec struct {
	Schedule       string       `json:"schedule"`
	ReportPeriod   string       `json:"reportPeriod,omitempty"`
	ReportingEnd   *metav1.Time `json:"reportingEnd"`
	ReportingStart *metav1.Time `json:"reportingStart"`
	Namespace      string       `json:"namespace"`
}

// ReportStatus defines the observed state of Report
type ReportStatus struct {
	LastScheduleTime *metav1.Time `json:"lastScheduleTime,omitempty"`
	Conditions       string       `json:"conditions,omitempty"`
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
