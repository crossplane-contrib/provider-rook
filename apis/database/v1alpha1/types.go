/*
Copyright 2019 The Crossplane Authors.

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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// A YuagbyteClusterSpec defines the desired state of a YugabyteCluster.
type YugabyteClusterSpec struct {

	// A Secret containing JSON encoded credentials for a Google Service Account
	// that will be used to authenticate to this GCP YugabyteCluster.
	Secret corev1.SecretKeySelector `json:"credentialsSecretRef"`

	// ProjectID is the project name (not numerical ID) of this GCP YugabyteCluster.
	ProjectID string `json:"projectID"`
}

// +kubebuilder:object:root=true

// A YugabyteCluster configures a Rook 'YugabyteCluster'
// +kubebuilder:printcolumn:name="PROJECT-ID",type="string",JSONPath=".spec.projectID"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="SECRET-NAME",type="string",JSONPath=".spec.credentialsSecretRef.name",priority=1
type YugabyteCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec YugabyteClusterSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// YugabyteClusterList contains a list of YugabyteCluster
type YugabyteClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []YugabyteCluster `json:"items"`
}
