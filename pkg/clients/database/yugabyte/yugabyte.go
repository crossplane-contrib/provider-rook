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

package yugabyte

import (
	"context"
	"reflect"

	rook "github.com/rook/rook/pkg/apis/rook.io/v1alpha2"
	rookv1alpha1 "github.com/rook/rook/pkg/apis/yugabytedb.rook.io/v1alpha1"
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
		&rookv1alpha1.YBCluster{},
		&rookv1alpha1.YBClusterList{},
	)

	return clients.NewClient(ctx, s, scheme)
}

// CrossToRook converts a Crossplane Yugabyte cluster object to a Rook Yugabyte
// cluster object.
func CrossToRook(c *v1alpha1.YugabyteCluster) *rookv1alpha1.YBCluster {
	return &rookv1alpha1.YBCluster{
		TypeMeta: c.TypeMeta,
		ObjectMeta: metav1.ObjectMeta{
			Name:      c.Spec.YugabyteClusterParameters.Name,
			Namespace: c.Spec.YugabyteClusterParameters.Namespace,
		},
		Spec: rookv1alpha1.YBClusterSpec{
			Annotations: rook.Annotations(c.Spec.YugabyteClusterParameters.Annotations),
			Master:      convertServer(c.Spec.YugabyteClusterParameters.Master),
			TServer:     convertServer(c.Spec.YugabyteClusterParameters.TServer),
		},
	}
}

// NeedsUpdate determines whether the external Rook Yugabyte cluster needs to be
// updated.
func NeedsUpdate(c *v1alpha1.YugabyteCluster, e *rookv1alpha1.YBCluster) bool {
	if !reflect.DeepEqual(rook.Annotations(c.Spec.YugabyteClusterParameters.Annotations), e.Spec.Annotations) {
		return true
	}
	if !reflect.DeepEqual(convertServer(c.Spec.YugabyteClusterParameters.Master), e.Spec.Master) {
		return true
	}
	if !reflect.DeepEqual(convertServer(c.Spec.YugabyteClusterParameters.TServer), e.Spec.TServer) {
		return true
	}
	return false
}

func convertServer(server v1alpha1.ServerSpec) rookv1alpha1.ServerSpec {
	return rookv1alpha1.ServerSpec{
		Replicas: server.Replicas,
		Network: rookv1alpha1.NetworkSpec{
			Ports: convertPorts(server.Network.Ports),
		},
		VolumeClaimTemplate: server.VolumeClaimTemplate,
	}
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
