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
	runtimev1alpha1 "github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"

	rook "github.com/rook/rook/pkg/apis/rook.io/v1alpha2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ServerSpec describes server related settings of the cluster
type ServerSpec struct {
	Replicas            int32                        `json:"replicas,omitempty"`
	Network             NetworkSpec                  `json:"network,omitempty"`
	VolumeClaimTemplate corev1.PersistentVolumeClaim `json:"volumeClaimTemplate,omitempty"`
}

// NetworkSpec describes network related settings of the cluster
type NetworkSpec struct {
	// Set of named ports that can be configured for this resource
	Ports []PortSpec `json:"ports,omitempty"`
}

// PortSpec is named port
type PortSpec struct {
	// Name of port
	Name string `json:"name,omitempty"`
	// Port number
	Port int32 `json:"port,omitempty"`
}

// A YugabyteClusterParameters defines the desired state of a YugabyteCluster.
type YugabyteClusterParameters struct {
	Annotations rook.Annotations `json:"annotations,omitempty"`
	Master      ServerSpec       `json:"master"`
	TServer     ServerSpec       `json:"tserver"`
}

// A YugabyteClusterSpec defines the desired state of a YugabyteCluster.
type YugabyteClusterSpec struct {
	runtimev1alpha1.ResourceSpec `json:",inline"`
	YugabyteClusterParameters    `json:",inline"`
}

// A YugabyteClusterStatus defines the current state of a YugabyteCluster.
type YugabyteClusterStatus struct {
	runtimev1alpha1.ResourceStatus `json:",inline"`
}

// +kubebuilder:object:root=true

// A YugabyteCluster configures a Rook 'YugabyteCluster'
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="PROJECT-ID",type="string",JSONPath=".spec.projectID"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="SECRET-NAME",type="string",JSONPath=".spec.credentialsSecretRef.name",priority=1
type YugabyteCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   YugabyteClusterSpec   `json:"spec,omitempty"`
	Status YugabyteClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// YugabyteClusterList contains a list of YugabyteCluster
type YugabyteClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []YugabyteCluster `json:"items"`
}

// A YugabyteClusterClassSpecTemplate is a template for the spec of a dynamically
// provisioned YugabyteCluster.
type YugabyteClusterClassSpecTemplate struct {
	runtimev1alpha1.NonPortableClassSpecTemplate `json:",inline"`
	YugabyteClusterParameters                    `json:",inline"`
}

// +kubebuilder:object:root=true

// A YugabyteClusterClass is a non-portable resource class. It defines the desired
// spec of resource claims that use it to dynamically provision a managed
// resource.
// +kubebuilder:printcolumn:name="PROVIDER-REF",type="string",JSONPath=".specTemplate.providerRef.name"
// +kubebuilder:printcolumn:name="RECLAIM-POLICY",type="string",JSONPath=".specTemplate.reclaimPolicy"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
type YugabyteClusterClass struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// SpecTemplate is a template for the spec of a dynamically provisioned
	// YugabyteCluster.
	SpecTemplate YugabyteClusterClassSpecTemplate `json:"specTemplate"`
}

// +kubebuilder:object:root=true

// YugabyteClusterClassList contains a list of cloud memorystore resource classes.
type YugabyteClusterClassList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []YugabyteClusterClass `json:"items"`
}
