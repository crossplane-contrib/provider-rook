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

package database

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	runtimev1alpha1 "github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane-runtime/pkg/resource"
	"github.com/crossplaneio/crossplane-runtime/pkg/test"
	databasev1alpha1 "github.com/crossplaneio/crossplane/apis/database/v1alpha1"

	"github.com/crossplaneio/stack-gcp/apis/database/v1alpha2"
)

var (
	_ resource.ManagedConfigurator = resource.ManagedConfiguratorFn(ConfigurePostgreSQLCloudsqlInstance)
	_ resource.ManagedConfigurator = resource.ManagedConfiguratorFn(ConfigureMyCloudsqlInstance)
)

func TestConfigurePostgreCloudsqlInstance(t *testing.T) {
	type args struct {
		ctx context.Context
		cm  resource.Claim
		cs  resource.NonPortableClass
		mg  resource.Managed
	}

	type want struct {
		mg  resource.Managed
		err error
	}

	claimUID := types.UID("definitely-a-uuid")
	providerName := "coolprovider"

	cases := map[string]struct {
		args args
		want want
	}{
		"Successful": {
			args: args{
				cm: &databasev1alpha1.PostgreSQLInstance{
					ObjectMeta: metav1.ObjectMeta{UID: claimUID},
					Spec:       databasev1alpha1.PostgreSQLInstanceSpec{EngineVersion: "9.6"},
				},
				cs: &v1alpha2.CloudsqlInstanceClass{
					SpecTemplate: v1alpha2.CloudsqlInstanceClassSpecTemplate{
						NonPortableClassSpecTemplate: runtimev1alpha1.NonPortableClassSpecTemplate{
							ProviderReference: &corev1.ObjectReference{Name: providerName},
							ReclaimPolicy:     runtimev1alpha1.ReclaimDelete,
						},
					},
				},
				mg: &v1alpha2.CloudsqlInstance{},
			},
			want: want{
				mg: &v1alpha2.CloudsqlInstance{
					Spec: v1alpha2.CloudsqlInstanceSpec{
						ResourceSpec: runtimev1alpha1.ResourceSpec{
							ReclaimPolicy:                    runtimev1alpha1.ReclaimDelete,
							WriteConnectionSecretToReference: corev1.LocalObjectReference{Name: string(claimUID)},
							ProviderReference:                &corev1.ObjectReference{Name: providerName},
						},
						CloudsqlInstanceParameters: v1alpha2.CloudsqlInstanceParameters{
							AuthorizedNetworks: []string{},
							DatabaseVersion:    "POSTGRES_9_6",
							Labels:             map[string]string{},
							StorageGB:          v1alpha2.DefaultStorageGB,
						},
					},
				},
				err: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := ConfigurePostgreSQLCloudsqlInstance(tc.args.ctx, tc.args.cm, tc.args.cs, tc.args.mg)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("ConfigurePostgreSQLCloudsqlInstance(...) Error  -want, +got: %s", diff)
			}
			if diff := cmp.Diff(tc.want.mg, tc.args.mg, test.EquateConditions()); diff != "" {
				t.Errorf("ConfigurePostgreSQLCloudsqlInstance(...) Managed -want, +got:\n%s", diff)
			}
		})
	}
}

func TestConfigureMyCloudsqlInstance(t *testing.T) {
	type args struct {
		ctx context.Context
		cm  resource.Claim
		cs  resource.NonPortableClass
		mg  resource.Managed
	}

	type want struct {
		mg  resource.Managed
		err error
	}

	claimUID := types.UID("definitely-a-uuid")
	providerName := "coolprovider"

	cases := map[string]struct {
		args args
		want want
	}{
		"Successful": {
			args: args{
				cm: &databasev1alpha1.MySQLInstance{
					ObjectMeta: metav1.ObjectMeta{UID: claimUID},
					Spec:       databasev1alpha1.MySQLInstanceSpec{EngineVersion: "5.6"},
				},
				cs: &v1alpha2.CloudsqlInstanceClass{
					SpecTemplate: v1alpha2.CloudsqlInstanceClassSpecTemplate{
						NonPortableClassSpecTemplate: runtimev1alpha1.NonPortableClassSpecTemplate{
							ProviderReference: &corev1.ObjectReference{Name: providerName},
							ReclaimPolicy:     runtimev1alpha1.ReclaimDelete,
						},
					},
				},
				mg: &v1alpha2.CloudsqlInstance{},
			},
			want: want{
				mg: &v1alpha2.CloudsqlInstance{
					Spec: v1alpha2.CloudsqlInstanceSpec{
						ResourceSpec: runtimev1alpha1.ResourceSpec{
							ReclaimPolicy:                    runtimev1alpha1.ReclaimDelete,
							WriteConnectionSecretToReference: corev1.LocalObjectReference{Name: string(claimUID)},
							ProviderReference:                &corev1.ObjectReference{Name: providerName},
						},
						CloudsqlInstanceParameters: v1alpha2.CloudsqlInstanceParameters{
							AuthorizedNetworks: []string{},
							DatabaseVersion:    "MYSQL_5_6",
							Labels:             map[string]string{},
							StorageGB:          v1alpha2.DefaultStorageGB,
						},
					},
				},
				err: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := ConfigureMyCloudsqlInstance(tc.args.ctx, tc.args.cm, tc.args.cs, tc.args.mg)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("ConfigureMyCloudsqlInstance(...) Error -want, +got: %s", diff)
			}
			if diff := cmp.Diff(tc.want.mg, tc.args.mg, test.EquateConditions()); diff != "" {
				t.Errorf("ConfigureMyCloudsqlInstance(...) Managed: -want, +got:\n%s", diff)
			}
		})
	}
}
