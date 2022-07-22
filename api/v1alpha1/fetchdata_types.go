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
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// FetchDataSpec defines the desired state of FetchData
type FetchDataSpec struct {
	//Namespace in which cron job is created
	CronjobNamespace string `json:"cronjobNamespace,omitempty"`

	//Schedule period for the CronJob
	Schedule string `json:"schedule,omitempty"`

	//Koku metrics pvc zipped files storage path
	BackupSrc string `json:"backupSrc,omitempty"`

	//Koku-metrics-pvc path to unzip files
	UnzipDir string `json:"unzipDir,omitempty"`

	// Value for the Database Name Environment Variable
	DatabaseName string `json:"databaseName,omitempty"`

	//Value for the Database Password Environment Variable
	DatabasePassword string `json:"databasePassword,omitempty"`

	// Value for the Database User Environment Variable
	DatabaseUser string `json:"databaseUser,omitempty"`

	// Value for the Database HostName Environment Variable
	DatabaseHostName string `json:"databaseHostName,omitempty"`

	// Value for the Database Environment Variable in order to define the port which it should use. It will be used in its container as well
	DatabasePort string `json:"databasePort,omitempty"`

	// If data has to be stored in one more additional storage similar to or any S3 Storage
	// True if you wish to store data or False if you don't want data to leave the openshift cluster.
	HAS_S3_ACCESS string `json:"has_s3_access,omitempty"`

	//Access key Id for S3 bucket
	// Default value: nil
	AWS_ACCESS_KEY_ID string `json:"aws_access_key_id,omitempty"`

	//Secret access key Id for S3 bucket
	// Default value: nil
	AWS_SECRET_ACCESS_KEY string `json:"aws_secret_access_key,omitempty"`

	//Bucket name where you want data to be stored
	// Default value: nil
	BUCKET_NAME string `json:"bucket_name,omitempty"`

	// Host name to access the S3 bucket
	// Default value: nil
	S3_HOST_NAME string `json:"s3_host_name,omitempty"`
}

// FetchDataStatus defines the observed state of FetchData
type FetchDataStatus struct {
	//Name of the CronJob object created and managed by it
	CronJobName string `json:"cronJobName"`

	//CronJobStatus represents the current state of a cronjob
	CronJobStatus batchv1.CronJobStatus `json:"cronJobStatus"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// FetchData is the Schema for the fetchdata API
type FetchData struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FetchDataSpec   `json:"spec,omitempty"`
	Status FetchDataStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// FetchDataList contains a list of FetchData
type FetchDataList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FetchData `json:"items"`
}

func init() {
	SchemeBuilder.Register(&FetchData{}, &FetchDataList{})
}
