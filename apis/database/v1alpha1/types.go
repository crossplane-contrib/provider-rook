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
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/crossplane/provider-rook/apis/v1alpha1"
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
	Name        string               `json:"name"`
	Namespace   string               `json:"namespace"`
	Annotations v1alpha1.Annotations `json:"annotations,omitempty"`
	Master      ServerSpec           `json:"master"`
	TServer     ServerSpec           `json:"tserver"`
}

// A YugabyteClusterSpec defines the desired state of a YugabyteCluster.
type YugabyteClusterSpec struct {
	xpv1.ResourceSpec         `json:",inline"`
	YugabyteClusterParameters `json:"forProvider"`
}

// A YugabyteClusterStatus defines the current state of a YugabyteCluster.
type YugabyteClusterStatus struct {
	xpv1.ResourceStatus `json:",inline"`
}

// +kubebuilder:object:root=true

// A YugabyteCluster configures a Rook 'ybclusters.yugabytedb.rook.io'
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="STATE",type="string",JSONPath=".status.state"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Cluster,categories={crossplane,managed,rook}
type YugabyteCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   YugabyteClusterSpec   `json:"spec"`
	Status YugabyteClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// YugabyteClusterList contains a list of YugabyteCluster
type YugabyteClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []YugabyteCluster `json:"items"`
}

// A CockroachClusterParameters defines the desired state of a CockroachCluster.
type CockroachClusterParameters struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	// The annotations-related configuration to add/set on each Pod related object.
	Annotations         v1alpha1.Annotations      `json:"annotations,omitempty"`
	Storage             v1alpha1.StorageScopeSpec `json:"scope,omitempty"`
	Network             NetworkSpec               `json:"network,omitempty"`
	Secure              bool                      `json:"secure,omitempty"`
	CachePercent        int                       `json:"cachePercent,omitempty"`
	MaxSQLMemoryPercent int                       `json:"maxSQLMemoryPercent,omitempty"`
}

// A CockroachClusterSpec defines the desired state of a CockroachCluster.
type CockroachClusterSpec struct {
	xpv1.ResourceSpec          `json:",inline"`
	CockroachClusterParameters `json:"forProvider"`
}

// A CockroachClusterStatus defines the current state of a CockroachCluster.
type CockroachClusterStatus struct {
	xpv1.ResourceStatus `json:",inline"`
}

// +kubebuilder:object:root=true

// A CockroachCluster configures a Rook 'clusters.cockroachdb.rook.io'
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="STATE",type="string",JSONPath=".status.state"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Cluster,categories={crossplane,managed,rook}
type CockroachCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CockroachClusterSpec   `json:"spec"`
	Status CockroachClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// CockroachClusterList contains a list of CockroachCluster
type CockroachClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CockroachCluster `json:"items"`
}
