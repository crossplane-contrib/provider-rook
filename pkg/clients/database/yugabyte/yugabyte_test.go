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
	"testing"

	runtimev1alpha1 "github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
	"github.com/google/go-cmp/cmp"
	rookv1alpha1 "github.com/rook/rook/pkg/apis/yugabytedb.rook.io/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/crossplaneio/stack-rook/apis/database/v1alpha1"
)

const (
	name      = "cool-name"
	namespace = "cool-namespace"

	providerName = "cool-rook"

	connectionSecretName = "cool-connection-secret"
)

type yugabyteClusterModifier func(*v1alpha1.YugabyteCluster)

func yugabyteWithMasterReplicas(r int32) yugabyteClusterModifier {
	return func(i *v1alpha1.YugabyteCluster) { i.Spec.YugabyteClusterParameters.Master.Replicas = r }
}

func yugabyteCluster(im ...yugabyteClusterModifier) *v1alpha1.YugabyteCluster {
	i := &v1alpha1.YugabyteCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1alpha1.YugabyteClusterSpec{
			ResourceSpec: runtimev1alpha1.ResourceSpec{
				ProviderReference:                &corev1.ObjectReference{Name: providerName},
				WriteConnectionSecretToReference: &runtimev1alpha1.SecretReference{Name: connectionSecretName},
			},
			YugabyteClusterParameters: v1alpha1.YugabyteClusterParameters{
				Name:      name,
				Namespace: namespace,
				Master: v1alpha1.ServerSpec{
					Replicas: int32(3),
					Network: v1alpha1.NetworkSpec{
						Ports: []v1alpha1.PortSpec{{
							Name: "cool-master-port",
							Port: int32(7000),
						}},
					},
				},
				TServer: v1alpha1.ServerSpec{
					Replicas: int32(3),
					Network: v1alpha1.NetworkSpec{
						Ports: []v1alpha1.PortSpec{{
							Name: "cool-tserver-port",
							Port: int32(7001),
						}},
					},
				},
			},
		},
	}

	for _, m := range im {
		m(i)
	}

	return i
}

type rookYugabyteClusterModifier func(*rookv1alpha1.YBCluster)

func withMasterReplicas(i int32) rookYugabyteClusterModifier {
	return func(c *rookv1alpha1.YBCluster) { c.Spec.Master.Replicas = i }
}

func rookYugabyteCluster(im ...rookYugabyteClusterModifier) *rookv1alpha1.YBCluster {
	i := &rookv1alpha1.YBCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: rookv1alpha1.YBClusterSpec{
			Master: rookv1alpha1.ServerSpec{
				Replicas: int32(3),
				Network: rookv1alpha1.NetworkSpec{
					Ports: []rookv1alpha1.PortSpec{{
						Name: "cool-master-port",
						Port: int32(7000),
					}},
				},
			},
			TServer: rookv1alpha1.ServerSpec{
				Replicas: int32(3),
				Network: rookv1alpha1.NetworkSpec{
					Ports: []rookv1alpha1.PortSpec{{
						Name: "cool-tserver-port",
						Port: int32(7001),
					}},
				},
			},
		},
	}

	for _, m := range im {
		m(i)
	}

	return i
}

type crossServerModifier func(v1alpha1.ServerSpec)

func withCrossServerReplicas(i int32) crossServerModifier {
	return func(s v1alpha1.ServerSpec) { s.Replicas = i }
}

func crossServer(sm ...crossServerModifier) v1alpha1.ServerSpec {
	s := v1alpha1.ServerSpec{
		Replicas: int32(3),
		Network: v1alpha1.NetworkSpec{
			Ports: []v1alpha1.PortSpec{{
				Name: "cool-tserver-port",
				Port: int32(7001),
			}},
		},
	}

	for _, m := range sm {
		m(s)
	}

	return s
}

type rookServerModifier func(rookv1alpha1.ServerSpec)

func withRookServerReplicas(i int32) rookServerModifier {
	return func(s rookv1alpha1.ServerSpec) { s.Replicas = i }
}

func rookServer(sm ...rookServerModifier) rookv1alpha1.ServerSpec {
	s := rookv1alpha1.ServerSpec{
		Replicas: int32(3),
		Network: rookv1alpha1.NetworkSpec{
			Ports: []rookv1alpha1.PortSpec{{
				Name: "cool-tserver-port",
				Port: int32(7001),
			}},
		},
	}

	for _, m := range sm {
		m(s)
	}

	return s
}

func TestCrossToRook(t *testing.T) {
	mr := int32(5)

	cases := map[string]struct {
		c    *v1alpha1.YugabyteCluster
		want *rookv1alpha1.YBCluster
	}{
		"Successful": {
			c:    yugabyteCluster(),
			want: rookYugabyteCluster(),
		},
		"SuccessfulWithModifier": {
			c:    yugabyteCluster(yugabyteWithMasterReplicas(mr)),
			want: rookYugabyteCluster(withMasterReplicas(mr)),
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
	mr := int32(5)

	cases := map[string]struct {
		c    *v1alpha1.YugabyteCluster
		r    *rookv1alpha1.YBCluster
		want bool
	}{
		"NoUpdateNeeded": {
			c:    yugabyteCluster(),
			r:    rookYugabyteCluster(),
			want: false,
		},
		"UpdateNeeded": {
			c:    yugabyteCluster(),
			r:    rookYugabyteCluster(withMasterReplicas(mr)),
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

func TestConvertServer(t *testing.T) {
	r := int32(5)

	cases := map[string]struct {
		c    v1alpha1.ServerSpec
		want rookv1alpha1.ServerSpec
	}{
		"Successful": {
			c:    crossServer(),
			want: rookServer(),
		},
		"SuccessfulWithModifier": {
			c:    crossServer(withCrossServerReplicas(r)),
			want: rookServer(withRookServerReplicas(r)),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := convertServer(tc.c)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("convertServer(...): -want, +got:\n%s", diff)
			}
		})
	}
}
