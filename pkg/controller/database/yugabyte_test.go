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

	runtimev1alpha1 "github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane-runtime/pkg/resource"
	"github.com/crossplaneio/crossplane-runtime/pkg/test"
	kubev1alpha1 "github.com/crossplaneio/crossplane/apis/kubernetes/v1alpha1"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	rookv1alpha1 "github.com/rook/rook/pkg/apis/yugabytedb.rook.io/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplaneio/stack-rook/apis/database/v1alpha1"
)

const (
	name      = "cool-name"
	namespace = "cool-namespace"
	uid       = types.UID("definitely-a-uuid")

	providerName       = "cool-rook"
	providerSecretName = "cool-rook-secret"
	providerSecretKey  = "credentials.json"
	providerSecretData = "definitelyjson"

	connectionSecretName = "cool-connection-secret"
)

var errorBoom = errors.New("boom")

type strange struct {
	resource.Managed
}

type clusterModifier func(*v1alpha1.YugabyteCluster)

func withConditions(c ...runtimev1alpha1.Condition) clusterModifier {
	return func(i *v1alpha1.YugabyteCluster) { i.Status.SetConditions(c...) }
}

func withBindingPhase(p runtimev1alpha1.BindingPhase) clusterModifier {
	return func(i *v1alpha1.YugabyteCluster) { i.Status.SetBindingPhase(p) }
}

func cluster(im ...clusterModifier) *v1alpha1.YugabyteCluster {
	i := &v1alpha1.YugabyteCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:  namespace,
			Name:       name,
			UID:        uid,
			Finalizers: []string{},
		},
		Spec: v1alpha1.YugabyteClusterSpec{
			ResourceSpec: runtimev1alpha1.ResourceSpec{
				ProviderReference:                &corev1.ObjectReference{Namespace: namespace, Name: providerName},
				WriteConnectionSecretToReference: corev1.LocalObjectReference{Name: connectionSecretName},
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

type rookClusterModifier func(*rookv1alpha1.YBCluster)

func withMasterReplicas(i int32) rookClusterModifier {
	return func(c *rookv1alpha1.YBCluster) { c.Spec.Master.Replicas = i }
}

func rookCluster(im ...rookClusterModifier) *rookv1alpha1.YBCluster {
	i := &rookv1alpha1.YBCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:  namespace,
			Name:       name,
			UID:        uid,
			Finalizers: []string{},
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

var _ resource.ExternalClient = &yugabyteExternal{}
var _ resource.ExternalConnecter = &yugabyteConnecter{}

func TestConnect(t *testing.T) {
	provider := kubev1alpha1.Provider{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: providerName},
		Spec: kubev1alpha1.ProviderSpec{
			Secret: corev1.LocalObjectReference{
				Name: providerSecretName,
			},
		},
	}

	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: providerSecretName},
		Data:       map[string][]byte{providerSecretKey: []byte(providerSecretData)},
	}

	type strange struct {
		resource.Managed
	}

	type args struct {
		ctx context.Context
		mg  resource.Managed
	}
	type want struct {
		err error
	}

	cases := map[string]struct {
		conn resource.ExternalConnecter
		args args
		want want
	}{
		"Connected": {
			conn: &yugabyteConnecter{
				client: &test.MockClient{MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
					switch key {
					case client.ObjectKey{Namespace: namespace, Name: providerName}:
						*obj.(*kubev1alpha1.Provider) = provider
					case client.ObjectKey{Namespace: namespace, Name: providerSecretName}:
						*obj.(*corev1.Secret) = secret
					}
					return nil
				}},
				newClient: func(_ context.Context, _ *corev1.Secret) (client.Client, error) { return nil, nil },
			},
			args: args{
				ctx: context.Background(),
				mg:  cluster(),
			},
			want: want{
				err: nil,
			},
		},
		"NotYugabyteCluster": {
			conn: &yugabyteConnecter{},
			args: args{ctx: context.Background(), mg: &strange{}},
			want: want{err: errors.New(errNotYugabyteCluster)},
		},
		"FailedToGetProvider": {
			conn: &yugabyteConnecter{
				client: &test.MockClient{MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
					return errorBoom
				}},
			},
			args: args{ctx: context.Background(), mg: cluster()},
			want: want{err: errors.Wrap(errorBoom, errGetYugabyteProvider)},
		},
		"FailedToGetProviderSecret": {
			conn: &yugabyteConnecter{
				client: &test.MockClient{MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
					switch key {
					case client.ObjectKey{Namespace: namespace, Name: providerName}:
						*obj.(*kubev1alpha1.Provider) = provider
					case client.ObjectKey{Namespace: namespace, Name: providerSecretName}:
						return errorBoom
					}
					return nil
				}},
			},
			args: args{ctx: context.Background(), mg: cluster()},
			want: want{err: errors.Wrap(errorBoom, errGetYugabyteProviderSecret)},
		},
		"FailedToCreateKubernetesClient": {
			conn: &yugabyteConnecter{
				client: &test.MockClient{MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
					switch key {
					case client.ObjectKey{Namespace: namespace, Name: providerName}:
						*obj.(*kubev1alpha1.Provider) = provider
					case client.ObjectKey{Namespace: namespace, Name: providerSecretName}:
						*obj.(*corev1.Secret) = secret
					}
					return nil
				}},
				newClient: func(_ context.Context, _ *corev1.Secret) (client.Client, error) { return nil, errorBoom },
			},
			args: args{ctx: context.Background(), mg: cluster()},
			want: want{err: errors.Wrap(errorBoom, errNewYugabyteClient)},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := tc.conn.Connect(tc.args.ctx, tc.args.mg)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("tc.conn.Connect(...): want error != got error:\n%s", diff)
			}
		})
	}
}

func TestObserve(t *testing.T) {
	type args struct {
		ctx context.Context
		mg  resource.Managed
	}
	type want struct {
		mg          resource.Managed
		observation resource.ExternalObservation
		err         error
	}

	cases := map[string]struct {
		client resource.ExternalClient
		args   args
		want   want
	}{
		"ObservedClusterAvailable": {
			client: &yugabyteExternal{client: &test.MockClient{
				MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
					if key == (client.ObjectKey{Namespace: namespace, Name: name}) {
						*obj.(*rookv1alpha1.YBCluster) = rookv1alpha1.YBCluster{
							ObjectMeta: metav1.ObjectMeta{
								Name:      name,
								Namespace: namespace,
							},
						}
					}
					return nil
				}},
			},
			args: args{
				ctx: context.Background(),
				mg:  cluster(),
			},
			want: want{
				mg: cluster(
					withConditions(runtimev1alpha1.Available()),
					withBindingPhase(runtimev1alpha1.BindingPhaseUnbound)),
				observation: resource.ExternalObservation{
					ResourceExists:    true,
					ConnectionDetails: resource.ConnectionDetails{},
				},
			},
		},
		"ObservedClusterDoesNotExist": {
			client: &yugabyteExternal{client: &test.MockClient{
				MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
					return errorBoom
				}},
			},
			args: args{
				ctx: context.Background(),
				mg:  cluster(),
			},
			want: want{
				mg:          cluster(),
				observation: resource.ExternalObservation{ResourceExists: false},
			},
		},
		"NotYugabyteCluster": {
			client: &yugabyteExternal{},
			args: args{
				ctx: context.Background(),
				mg:  &strange{},
			},
			want: want{
				mg:  &strange{},
				err: errors.New(errNotYugabyteCluster),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := tc.client.Observe(tc.args.ctx, tc.args.mg)

			if diff := cmp.Diff(tc.want.observation, got, test.EquateErrors()); diff != "" {
				t.Errorf("tc.client.Observe(): -want, +got:\n%s", diff)
			}

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("tc.client.Observe(): -want error, +got error:\n%s", diff)
			}

			if diff := cmp.Diff(tc.want.mg, tc.args.mg, test.EquateConditions()); diff != "" {
				t.Errorf("resource.Managed: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestCreate(t *testing.T) {
	type args struct {
		ctx context.Context
		mg  resource.Managed
	}
	type want struct {
		mg       resource.Managed
		creation resource.ExternalCreation
		err      error
	}

	cases := map[string]struct {
		client resource.ExternalClient
		args   args
		want   want
	}{
		"CreatedCluster": {
			client: &yugabyteExternal{client: &test.MockClient{
				MockCreate: func(_ context.Context, obj runtime.Object, _ ...client.CreateOption) error {
					return nil
				}},
			},
			args: args{
				ctx: context.Background(),
				mg:  cluster(),
			},
			want: want{
				mg: cluster(withConditions(runtimev1alpha1.Creating())),
			},
		},
		"NotYugabyteCluster": {
			client: &yugabyteExternal{},
			args: args{
				ctx: context.Background(),
				mg:  &strange{},
			},
			want: want{
				mg:  &strange{},
				err: errors.New(errNotYugabyteCluster),
			},
		},
		"FailedToCreateCluster": {
			client: &yugabyteExternal{client: &test.MockClient{
				MockCreate: func(_ context.Context, obj runtime.Object, _ ...client.CreateOption) error {
					return errorBoom
				}},
			},
			args: args{
				ctx: context.Background(),
				mg:  cluster(),
			},
			want: want{
				mg:  cluster(withConditions(runtimev1alpha1.Creating())),
				err: errors.Wrap(errorBoom, errCreateYugabyteCluster),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := tc.client.Create(tc.args.ctx, tc.args.mg)

			if diff := cmp.Diff(tc.want.creation, got, test.EquateErrors()); diff != "" {
				t.Errorf("tc.client.Create(): -want, +got:\n%s", diff)
			}

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("tc.client.Create(): -want error, +got error:\n%s", diff)
			}

			if diff := cmp.Diff(tc.want.mg, tc.args.mg, test.EquateConditions()); diff != "" {
				t.Errorf("resource.Managed: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestUpdate(t *testing.T) {
	type args struct {
		ctx context.Context
		mg  resource.Managed
	}
	type want struct {
		mg     resource.Managed
		update resource.ExternalUpdate
		err    error
	}

	cases := map[string]struct {
		client resource.ExternalClient
		args   args
		want   want
	}{
		"UpdatedInstance": {
			client: &yugabyteExternal{client: &test.MockClient{
				MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
					if key == (client.ObjectKey{Namespace: namespace, Name: name}) {
						*obj.(*rookv1alpha1.YBCluster) = *rookCluster(withMasterReplicas(int32(4)))
					}
					return nil
				},
				MockUpdate: func(_ context.Context, obj runtime.Object, _ ...client.UpdateOption) error {
					return nil
				},
			}},
			args: args{
				ctx: context.Background(),
				mg:  cluster(),
			},
			want: want{
				mg: cluster(),
			},
		},
		"UpdatedNotRequired": {
			client: &yugabyteExternal{client: &test.MockClient{
				MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
					if key == (client.ObjectKey{Namespace: namespace, Name: name}) {
						*obj.(*rookv1alpha1.YBCluster) = *rookCluster()
					}
					return nil
				},
			}},
			args: args{
				ctx: context.Background(),
				mg:  cluster(),
			},
			want: want{
				mg: cluster(),
			},
		},
		"NotYugabyteCluster": {
			client: &yugabyteExternal{},
			args: args{
				ctx: context.Background(),
				mg:  &strange{},
			},
			want: want{
				mg:  &strange{},
				err: errors.New(errNotYugabyteCluster),
			},
		},
		"FailedToGetCluster": {
			client: &yugabyteExternal{client: &test.MockClient{
				MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
					return errorBoom
				}},
			},
			args: args{
				ctx: context.Background(),
				mg:  cluster(),
			},
			want: want{
				mg:  cluster(),
				err: errors.Wrap(errorBoom, errGetYugabyteCluster),
			},
		},
		"FailedToUpdateCluster": {
			client: &yugabyteExternal{client: &test.MockClient{
				MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
					if key == (client.ObjectKey{Namespace: namespace, Name: name}) {
						*obj.(*rookv1alpha1.YBCluster) = *rookCluster(withMasterReplicas(int32(4)))
					}
					return nil
				},
				MockUpdate: func(_ context.Context, obj runtime.Object, _ ...client.UpdateOption) error {
					return errorBoom
				},
			}},

			args: args{
				ctx: context.Background(),
				mg:  cluster(),
			},
			want: want{
				mg:  cluster(),
				err: errors.Wrap(errorBoom, errUpdateYugabyteCluster),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := tc.client.Update(tc.args.ctx, tc.args.mg)

			if diff := cmp.Diff(tc.want.update, got, test.EquateErrors()); diff != "" {
				t.Errorf("tc.client.Update(): -want, +got:\n%s", diff)
			}

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("tc.client.Update(): -want error, +got error:\n%s", diff)
			}

			if diff := cmp.Diff(tc.want.mg, tc.args.mg, test.EquateConditions()); diff != "" {
				t.Errorf("resource.Managed: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestDelete(t *testing.T) {
	type args struct {
		ctx context.Context
		mg  resource.Managed
	}
	type want struct {
		mg  resource.Managed
		err error
	}

	cases := map[string]struct {
		client resource.ExternalClient
		args   args
		want   want
	}{
		"DeletedCluster": {
			client: &yugabyteExternal{client: &test.MockClient{
				MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
					if key == (client.ObjectKey{Namespace: namespace, Name: name}) {
						*obj.(*rookv1alpha1.YBCluster) = *rookCluster()
					}
					return nil
				},
				MockDelete: func(_ context.Context, obj runtime.Object, _ ...client.DeleteOption) error {
					return nil
				}},
			},
			args: args{
				ctx: context.Background(),
				mg:  cluster(),
			},
			want: want{
				mg: cluster(withConditions(runtimev1alpha1.Deleting())),
			},
		},
		"NotYugabyteCluster": {
			client: &yugabyteExternal{},
			args: args{
				ctx: context.Background(),
				mg:  &strange{},
			},
			want: want{
				mg:  &strange{},
				err: errors.New(errNotYugabyteCluster),
			},
		},
		"FailedToDeleteCluster": {
			client: &yugabyteExternal{client: &test.MockClient{
				MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
					if key == (client.ObjectKey{Namespace: namespace, Name: name}) {
						*obj.(*rookv1alpha1.YBCluster) = *rookCluster()
					}
					return nil
				},
				MockDelete: func(_ context.Context, obj runtime.Object, _ ...client.DeleteOption) error {
					return errorBoom
				}},
			},

			args: args{
				ctx: context.Background(),
				mg:  cluster(),
			},
			want: want{
				mg:  cluster(withConditions(runtimev1alpha1.Deleting())),
				err: errors.Wrap(errorBoom, errDeleteYugabyteCluster),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := tc.client.Delete(tc.args.ctx, tc.args.mg)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("tc.client.Delete(): -want error, +got error:\n%s", diff)
			}

			if diff := cmp.Diff(tc.want.mg, tc.args.mg, test.EquateConditions()); diff != "" {
				t.Errorf("resource.Managed: -want, +got:\n%s", diff)
			}
		})
	}
}
