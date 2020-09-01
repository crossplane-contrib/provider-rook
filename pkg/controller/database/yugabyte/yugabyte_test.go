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
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"

	runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	rookv1alpha1 "github.com/rook/rook/pkg/apis/yugabytedb.rook.io/v1alpha1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/provider-rook/apis/database/v1alpha1"
)

const (
	name      = "cool-name"
	namespace = "cool-namespace"
	uid       = types.UID("definitely-a-uuid")

	connectionSecretName = "cool-connection-secret"
)

var errorBoom = errors.New("boom")
var errorYugabyteNotFound = kerrors.NewNotFound(
	schema.GroupResource{
		Group:    "yugabytedb.rook.io",
		Resource: "YBCluster"},
	"boom")

type yugabyteStrange struct {
	resource.Managed
}

type yugabyteClusterModifier func(*v1alpha1.YugabyteCluster)

func yugabyteWithConditions(c ...runtimev1alpha1.Condition) yugabyteClusterModifier {
	return func(i *v1alpha1.YugabyteCluster) { i.Status.SetConditions(c...) }
}

func yugabyteWithBindingPhase(p runtimev1alpha1.BindingPhase) yugabyteClusterModifier {
	return func(i *v1alpha1.YugabyteCluster) { i.Status.SetBindingPhase(p) }
}

func yugabyteCluster(im ...yugabyteClusterModifier) *v1alpha1.YugabyteCluster {
	i := &v1alpha1.YugabyteCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:       name,
			UID:        uid,
			Finalizers: []string{},
		},
		Spec: v1alpha1.YugabyteClusterSpec{
			ResourceSpec: runtimev1alpha1.ResourceSpec{
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

var _ managed.ExternalClient = &external{}
var _ managed.ExternalConnecter = &connecter{}

func TestObserveYugabyte(t *testing.T) {
	type args struct {
		ctx context.Context
		mg  resource.Managed
	}
	type want struct {
		mg          resource.Managed
		observation managed.ExternalObservation
		err         error
	}

	cases := map[string]struct {
		client managed.ExternalClient
		args   args
		want   want
	}{
		"ObservedClusterAvailable": {
			client: &external{client: &test.MockClient{
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
				mg:  yugabyteCluster(),
			},
			want: want{
				mg: yugabyteCluster(
					yugabyteWithConditions(runtimev1alpha1.Available()),
					yugabyteWithBindingPhase(runtimev1alpha1.BindingPhaseUnbound)),
				observation: managed.ExternalObservation{
					ResourceExists:    true,
					ConnectionDetails: managed.ConnectionDetails{},
				},
			},
		},
		"ObservedClusterDoesNotExist": {
			client: &external{client: &test.MockClient{
				MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
					return errorYugabyteNotFound
				}},
			},
			args: args{
				ctx: context.Background(),
				mg:  yugabyteCluster(),
			},
			want: want{
				mg:          yugabyteCluster(),
				observation: managed.ExternalObservation{ResourceExists: false},
			},
		},
		"NotYugabyteCluster": {
			client: &external{},
			args: args{
				ctx: context.Background(),
				mg:  &yugabyteStrange{},
			},
			want: want{
				mg:  &yugabyteStrange{},
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

func TestCreateYugabyte(t *testing.T) {
	type args struct {
		ctx context.Context
		mg  resource.Managed
	}
	type want struct {
		mg       resource.Managed
		creation managed.ExternalCreation
		err      error
	}

	cases := map[string]struct {
		client managed.ExternalClient
		args   args
		want   want
	}{
		"CreatedCluster": {
			client: &external{client: &test.MockClient{
				MockCreate: func(_ context.Context, obj runtime.Object, _ ...client.CreateOption) error {
					return nil
				}},
			},
			args: args{
				ctx: context.Background(),
				mg:  yugabyteCluster(),
			},
			want: want{
				mg: yugabyteCluster(yugabyteWithConditions(runtimev1alpha1.Creating())),
			},
		},
		"NotYugabyteCluster": {
			client: &external{},
			args: args{
				ctx: context.Background(),
				mg:  &yugabyteStrange{},
			},
			want: want{
				mg:  &yugabyteStrange{},
				err: errors.New(errNotYugabyteCluster),
			},
		},
		"FailedToCreateCluster": {
			client: &external{client: &test.MockClient{
				MockCreate: func(_ context.Context, obj runtime.Object, _ ...client.CreateOption) error {
					return errorBoom
				}},
			},
			args: args{
				ctx: context.Background(),
				mg:  yugabyteCluster(),
			},
			want: want{
				mg:  yugabyteCluster(yugabyteWithConditions(runtimev1alpha1.Creating())),
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

func TestUpdateYugabyte(t *testing.T) {
	type args struct {
		ctx context.Context
		mg  resource.Managed
	}
	type want struct {
		mg     resource.Managed
		update managed.ExternalUpdate
		err    error
	}

	cases := map[string]struct {
		client managed.ExternalClient
		args   args
		want   want
	}{
		"UpdatedCluster": {
			client: &external{client: &test.MockClient{
				MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
					if key == (client.ObjectKey{Namespace: namespace, Name: name}) {
						*obj.(*rookv1alpha1.YBCluster) = *rookYugabyteCluster(withMasterReplicas(int32(4)))
					}
					return nil
				},
				MockUpdate: func(_ context.Context, obj runtime.Object, _ ...client.UpdateOption) error {
					return nil
				},
			}},
			args: args{
				ctx: context.Background(),
				mg:  yugabyteCluster(),
			},
			want: want{
				mg: yugabyteCluster(),
			},
		},
		"UpdatedNotRequired": {
			client: &external{client: &test.MockClient{
				MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
					if key == (client.ObjectKey{Namespace: namespace, Name: name}) {
						*obj.(*rookv1alpha1.YBCluster) = *rookYugabyteCluster()
					}
					return nil
				},
			}},
			args: args{
				ctx: context.Background(),
				mg:  yugabyteCluster(),
			},
			want: want{
				mg: yugabyteCluster(),
			},
		},
		"NotYugabyteCluster": {
			client: &external{},
			args: args{
				ctx: context.Background(),
				mg:  &yugabyteStrange{},
			},
			want: want{
				mg:  &yugabyteStrange{},
				err: errors.New(errNotYugabyteCluster),
			},
		},
		"FailedToGetCluster": {
			client: &external{client: &test.MockClient{
				MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
					return errorBoom
				}},
			},
			args: args{
				ctx: context.Background(),
				mg:  yugabyteCluster(),
			},
			want: want{
				mg:  yugabyteCluster(),
				err: errors.Wrap(errorBoom, errGetYugabyteCluster),
			},
		},
		"FailedToUpdateCluster": {
			client: &external{client: &test.MockClient{
				MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
					if key == (client.ObjectKey{Namespace: namespace, Name: name}) {
						*obj.(*rookv1alpha1.YBCluster) = *rookYugabyteCluster(withMasterReplicas(int32(4)))
					}
					return nil
				},
				MockUpdate: func(_ context.Context, obj runtime.Object, _ ...client.UpdateOption) error {
					return errorBoom
				},
			}},

			args: args{
				ctx: context.Background(),
				mg:  yugabyteCluster(),
			},
			want: want{
				mg:  yugabyteCluster(),
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

func TestDeleteYugabyte(t *testing.T) {
	type args struct {
		ctx context.Context
		mg  resource.Managed
	}
	type want struct {
		mg  resource.Managed
		err error
	}

	cases := map[string]struct {
		client managed.ExternalClient
		args   args
		want   want
	}{
		"DeletedCluster": {
			client: &external{client: &test.MockClient{
				MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
					if key == (client.ObjectKey{Namespace: namespace, Name: name}) {
						*obj.(*rookv1alpha1.YBCluster) = *rookYugabyteCluster()
					}
					return nil
				},
				MockDelete: func(_ context.Context, obj runtime.Object, _ ...client.DeleteOption) error {
					return nil
				}},
			},
			args: args{
				ctx: context.Background(),
				mg:  yugabyteCluster(),
			},
			want: want{
				mg: yugabyteCluster(yugabyteWithConditions(runtimev1alpha1.Deleting())),
			},
		},
		"NotYugabyteCluster": {
			client: &external{},
			args: args{
				ctx: context.Background(),
				mg:  &yugabyteStrange{},
			},
			want: want{
				mg:  &yugabyteStrange{},
				err: errors.New(errNotYugabyteCluster),
			},
		},
		"FailedToDeleteCluster": {
			client: &external{client: &test.MockClient{
				MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
					if key == (client.ObjectKey{Namespace: namespace, Name: name}) {
						*obj.(*rookv1alpha1.YBCluster) = *rookYugabyteCluster()
					}
					return nil
				},
				MockDelete: func(_ context.Context, obj runtime.Object, _ ...client.DeleteOption) error {
					return errorBoom
				}},
			},

			args: args{
				ctx: context.Background(),
				mg:  yugabyteCluster(),
			},
			want: want{
				mg:  yugabyteCluster(yugabyteWithConditions(runtimev1alpha1.Deleting())),
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
