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
	"context"
	"testing"

	runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/claimbinding"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	databasev1alpha1 "github.com/crossplane/crossplane/apis/database/v1alpha1"
	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/types"

	"github.com/crossplane/provider-rook/apis/database/v1alpha1"
	corev1alpha1 "github.com/crossplane/provider-rook/apis/v1alpha1"
)

var _ claimbinding.ManagedConfigurator = claimbinding.ManagedConfiguratorFn(ConfigureCockroachCluster)

func TestConfigureCockroachCluster(t *testing.T) {
	type args struct {
		ctx context.Context
		cm  resource.Claim
		cs  resource.Class
		mg  resource.Managed
	}

	type want struct {
		mg  resource.Managed
		err error
	}

	claimUID := types.UID("definitely-a-uuid")
	providerName := "coolprovider"

	params := v1alpha1.CockroachClusterParameters{
		Name:        "cool-cockroach",
		Namespace:   "cool-cockroach-ns",
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
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"Successful": {
			args: args{
				cm: &databasev1alpha1.PostgreSQLInstance{
					ObjectMeta: metav1.ObjectMeta{UID: claimUID},
					Spec: databasev1alpha1.PostgreSQLInstanceSpec{
						ResourceClaimSpec: runtimev1alpha1.ResourceClaimSpec{
							ClassReference:                   &corev1.ObjectReference{},
							WriteConnectionSecretToReference: &runtimev1alpha1.LocalSecretReference{},
							ResourceReference:                nil,
						},
					},
				},
				cs: &v1alpha1.CockroachClusterClass{
					SpecTemplate: v1alpha1.CockroachClusterClassSpecTemplate{
						ClassSpecTemplate: runtimev1alpha1.ClassSpecTemplate{
							ProviderReference: runtimev1alpha1.Reference{Name: providerName},
							ReclaimPolicy:     runtimev1alpha1.ReclaimDelete,
						},
						CockroachClusterParameters: params,
					},
				},
				mg: &v1alpha1.CockroachCluster{},
			},
			want: want{
				mg: &v1alpha1.CockroachCluster{
					Spec: v1alpha1.CockroachClusterSpec{
						ResourceSpec: runtimev1alpha1.ResourceSpec{
							ProviderReference:                &runtimev1alpha1.Reference{Name: providerName},
							ReclaimPolicy:                    runtimev1alpha1.ReclaimDelete,
							WriteConnectionSecretToReference: &runtimev1alpha1.SecretReference{Name: string(claimUID)},
						},
						CockroachClusterParameters: params,
					},
				},
				err: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := ConfigureCockroachCluster(tc.args.ctx, tc.args.cm, tc.args.cs, tc.args.mg)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("ConfigureCockroachCluster(...): -want error, +got error:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.mg, tc.args.mg, test.EquateConditions()); diff != "" {
				t.Errorf("ConfigureCockroachCluster(...) Managed: -want, +got:\n%s", diff)
			}
		})
	}
}
