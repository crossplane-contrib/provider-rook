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

package cockroach

import (
	"testing"

	runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/google/go-cmp/cmp"
	rookv1alpha1 "github.com/rook/rook/pkg/apis/cockroachdb.rook.io/v1alpha1"
	rook "github.com/rook/rook/pkg/apis/rook.io/v1alpha2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/crossplane/provider-rook/apis/database/v1alpha1"
	corev1alpha1 "github.com/crossplane/provider-rook/apis/v1alpha1"
)

const (
	name      = "cool-name"
	namespace = "cool-namespace"

	providerName         = "cool-rook"
	connectionSecretName = "cool-connection-secret"
)

type cockroachClusterModifier func(*v1alpha1.CockroachCluster)

func withCockroachNodeCount(c int) cockroachClusterModifier {
	return func(i *v1alpha1.CockroachCluster) { i.Spec.CockroachClusterParameters.Storage.NodeCount = c }
}

func cockroachCluster(im ...cockroachClusterModifier) *v1alpha1.CockroachCluster {
	i := &v1alpha1.CockroachCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1alpha1.CockroachClusterSpec{
			ResourceSpec: runtimev1alpha1.ResourceSpec{
				ProviderReference:                &runtimev1alpha1.Reference{Name: providerName},
				WriteConnectionSecretToReference: &runtimev1alpha1.SecretReference{Name: connectionSecretName},
			},
			CockroachClusterParameters: v1alpha1.CockroachClusterParameters{
				Name:        name,
				Namespace:   namespace,
				Annotations: corev1alpha1.Annotations(map[string]string{"label": "value"}),
				Storage: corev1alpha1.StorageScopeSpec{
					NodeCount: 3,
					VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "rook-cockroachdb-test",
							},
							Spec: corev1.PersistentVolumeClaimSpec{
								AccessModes: []corev1.PersistentVolumeAccessMode{"ReadWriteOnce"},
								// Test does not check resource requirements due cmp pkg
								// inability to compare unexported fields.
								Resources: corev1.ResourceRequirements{},
							},
						},
					},
				},
				Network: v1alpha1.NetworkSpec{
					Ports: []v1alpha1.PortSpec{{
						Name: "cool--port",
						Port: int32(7001),
					}},
				},
				Secure:              false,
				CachePercent:        80,
				MaxSQLMemoryPercent: 80,
			},
		},
	}

	for _, m := range im {
		m(i)
	}

	return i
}

type rookCockroachClusterModifier func(*rookv1alpha1.Cluster)

func withNodeCount(i int) rookCockroachClusterModifier {
	return func(c *rookv1alpha1.Cluster) { c.Spec.Storage.NodeCount = i }
}

func rookCockroachCluster(im ...rookCockroachClusterModifier) *rookv1alpha1.Cluster {
	i := &rookv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: rookv1alpha1.ClusterSpec{
			Annotations: rook.Annotations(map[string]string{"label": "value"}),
			Storage: rook.StorageScopeSpec{
				NodeCount: 3,
				Selection: rook.Selection{
					VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "rook-cockroachdb-test",
							},
							Spec: corev1.PersistentVolumeClaimSpec{
								AccessModes: []corev1.PersistentVolumeAccessMode{"ReadWriteOnce"},
								// Test does not check resource requirements due cmp pkg
								// inability to compare unexported fields.
								Resources: corev1.ResourceRequirements{},
							},
						},
					},
				},
			},
			Network: rookv1alpha1.NetworkSpec{
				Ports: []rookv1alpha1.PortSpec{{
					Name: "cool--port",
					Port: int32(7001),
				}},
			},
			Secure:              false,
			CachePercent:        80,
			MaxSQLMemoryPercent: 80,
		},
	}

	for _, m := range im {
		m(i)
	}

	return i
}

type crossPortsModifier func([]v1alpha1.PortSpec)

func withCrossPortsName(n string) crossPortsModifier {
	return func(p []v1alpha1.PortSpec) { p[0].Name = n }
}

func crossPorts(pm ...crossPortsModifier) []v1alpha1.PortSpec {
	p := []v1alpha1.PortSpec{{
		Name: "cool-tserver-port",
		Port: int32(7001),
	}}

	for _, m := range pm {
		m(p)
	}

	return p
}

type rookPortsModifier func([]rookv1alpha1.PortSpec)

func withRookPortsName(n string) rookPortsModifier {
	return func(p []rookv1alpha1.PortSpec) { p[0].Name = n }
}

func rookPorts(pm ...rookPortsModifier) []rookv1alpha1.PortSpec {
	p := []rookv1alpha1.PortSpec{{
		Name: "cool-tserver-port",
		Port: int32(7001),
	}}

	for _, m := range pm {
		m(p)
	}

	return p
}

func TestCrossToRook(t *testing.T) {
	cases := map[string]struct {
		c    *v1alpha1.CockroachCluster
		want *rookv1alpha1.Cluster
	}{
		"Successful": {
			c:    cockroachCluster(),
			want: rookCockroachCluster(),
		},
		"SuccessfulWithModifier": {
			c:    cockroachCluster(withCockroachNodeCount(5)),
			want: rookCockroachCluster(withNodeCount(5)),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := CrossToRook(tc.c)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("CrossToRook(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestNeedsUpdate(t *testing.T) {
	cases := map[string]struct {
		c    *v1alpha1.CockroachCluster
		r    *rookv1alpha1.Cluster
		want bool
	}{
		"NoUpdateNeeded": {
			c:    cockroachCluster(),
			r:    rookCockroachCluster(),
			want: false,
		},
		"UpdateNeeded": {
			c:    cockroachCluster(),
			r:    rookCockroachCluster(withNodeCount(5)),
			want: true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := NeedsUpdate(tc.c, tc.r)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("NeedsUpdate(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestConvertPorts(t *testing.T) {
	n := "cool-port"

	cases := map[string]struct {
		c    []v1alpha1.PortSpec
		want []rookv1alpha1.PortSpec
	}{
		"Successful": {
			c:    crossPorts(),
			want: rookPorts(),
		},
		"SuccessfulWithModifier": {
			c:    crossPorts(withCrossPortsName(n)),
			want: rookPorts(withRookPortsName(n)),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := convertPorts(tc.c)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("convertPorts(...): -want, +got:\n%s", diff)
			}
		})
	}
}
