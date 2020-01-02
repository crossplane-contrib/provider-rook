/*
Copyright 2020 The Crossplane Authors.

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

package cockroach

import (
	"context"
	"reflect"

	rookv1alpha1 "github.com/rook/rook/pkg/apis/cockroachdb.rook.io/v1alpha1"
	rook "github.com/rook/rook/pkg/apis/rook.io/v1alpha2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplaneio/stack-rook/apis/database/v1alpha1"
	"github.com/crossplaneio/stack-rook/pkg/clients"
)

// NewClient returns a new Kubernetes client with Rook Yugabyte types
// registered.
func NewClient(ctx context.Context, s *corev1.Secret) (client.Client, error) {
	scheme := runtime.NewScheme()
	scheme.AddKnownTypes(rookv1alpha1.SchemeGroupVersion,
		&rookv1alpha1.Cluster{},
		&rookv1alpha1.ClusterList{},
	)

	metav1.AddToGroupVersion(scheme, rookv1alpha1.SchemeGroupVersion)

	return clients.NewClient(ctx, s, scheme)
}

// CrossToRook converts a Crossplane Yugabyte cluster object to a Rook Yugabyte
// cluster object.
func CrossToRook(c *v1alpha1.CockroachCluster) *rookv1alpha1.Cluster {
	params := c.Spec.CockroachClusterParameters
	return &rookv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      params.Name,
			Namespace: params.Namespace,
		},
		Spec: rookv1alpha1.ClusterSpec{
			Annotations: rook.Annotations(params.Annotations),
			Storage: rook.StorageScopeSpec{
				NodeCount: params.Storage.NodeCount,
				Selection: rook.Selection{
					VolumeClaimTemplates: params.Storage.VolumeClaimTemplates,
				},
			},
			Network: rookv1alpha1.NetworkSpec{
				Ports: convertPorts(params.Network.Ports),
			},
			Secure:              params.Secure,
			CachePercent:        params.CachePercent,
			MaxSQLMemoryPercent: params.MaxSQLMemoryPercent,
		},
	}
}

// NeedsUpdate determines whether the external Rook Cockroach cluster needs to be
// updated.
func NeedsUpdate(c *v1alpha1.CockroachCluster, e *rookv1alpha1.Cluster) bool {
	params := c.Spec.CockroachClusterParameters
	if !reflect.DeepEqual(rook.Annotations(params.Annotations), e.Spec.Annotations) {
		return true
	}
	if !reflect.DeepEqual(params.Storage.NodeCount, e.Spec.Storage.NodeCount) {
		return true
	}
	if !reflect.DeepEqual(params.Storage.VolumeClaimTemplates, e.Spec.Storage.VolumeClaimTemplates) {
		return true
	}
	if !reflect.DeepEqual(convertPorts(params.Network.Ports), e.Spec.Network.Ports) {
		return true
	}
	if !reflect.DeepEqual(params.CachePercent, e.Spec.CachePercent) {
		return true
	}
	if !reflect.DeepEqual(params.MaxSQLMemoryPercent, e.Spec.MaxSQLMemoryPercent) {
		return true
	}
	return false
}

func convertPorts(ports []v1alpha1.PortSpec) []rookv1alpha1.PortSpec {
	rookports := make([]rookv1alpha1.PortSpec, len(ports))
	for i, p := range ports {
		rookports[i] = rookv1alpha1.PortSpec{
			Name: p.Name,
			Port: p.Port,
		}
	}
	return rookports
}
