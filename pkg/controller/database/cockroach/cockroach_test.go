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
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	kubev1alpha1 "github.com/crossplane/crossplane/apis/kubernetes/v1alpha1"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	rookv1alpha1 "github.com/rook/rook/pkg/apis/cockroachdb.rook.io/v1alpha1"
	rook "github.com/rook/rook/pkg/apis/rook.io/v1alpha2"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/provider-rook/apis/database/v1alpha1"
	corev1alpha1 "github.com/crossplane/provider-rook/apis/v1alpha1"
)

const (
	name      = "cool-name"
	namespace = "cool-namespace"
	uid       = types.UID("definitely-a-uuid")

	providerName            = "cool-rook"
	providerSecretName      = "cool-rook-secret"
	providerSecretNamespace = "cool-rook-secret-namespace"
	providerSecretKey       = "credentials.json"
	providerSecretData      = "definitelyjson"

	connectionSecretName = "cool-connection-secret"
)

var errorBoom = errors.New("boom")
var errorCockroachNotFound = kerrors.NewNotFound(
	schema.GroupResource{
		Group:    "cockroachdb.rook.io",
		Resource: "Cluster"},
	"boom")

type cockroachStrange struct {
	resource.Managed
}

type cockroachClusterModifier func(*v1alpha1.CockroachCluster)

func withConditions(c ...runtimev1alpha1.Condition) cockroachClusterModifier {
	return func(i *v1alpha1.CockroachCluster) { i.Status.SetConditions(c...) }
}

func withBindingPhase(p runtimev1alpha1.BindingPhase) cockroachClusterModifier {
	return func(i *v1alpha1.CockroachCluster) { i.Status.SetBindingPhase(p) }
}

func cockroachCluster(im ...cockroachClusterModifier) *v1alpha1.CockroachCluster {
	i := &v1alpha1.CockroachCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:       name,
			UID:        uid,
			Finalizers: []string{},
		},
		Spec: v1alpha1.CockroachClusterSpec{
			ResourceSpec: runtimev1alpha1.ResourceSpec{
				ProviderReference:                &corev1.ObjectReference{Name: providerName},
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
			Name:       name,
			Namespace:  namespace,
			UID:        uid,
			Finalizers: []string{},
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

var _ managed.ExternalClient = &external{}
var _ managed.ExternalConnecter = &connecter{}

func TestConnectCockroach(t *testing.T) {
	provider := kubev1alpha1.Provider{
		ObjectMeta: metav1.ObjectMeta{Name: providerName},
		Spec: kubev1alpha1.ProviderSpec{
			Secret: runtimev1alpha1.SecretReference{
				Name:      providerSecretName,
				Namespace: providerSecretNamespace,
			},
		},
	}

	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: providerSecretNamespace, Name: providerSecretName},
		Data:       map[string][]byte{providerSecretKey: []byte(providerSecretData)},
	}

	type cockroachStrange struct {
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
		conn managed.ExternalConnecter
		args args
		want want
	}{
		"Connected": {
			conn: &connecter{
				client: &test.MockClient{MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
					switch key {
					case client.ObjectKey{Name: providerName}:
						*obj.(*kubev1alpha1.Provider) = provider
					case client.ObjectKey{Namespace: providerSecretNamespace, Name: providerSecretName}:
						*obj.(*corev1.Secret) = secret
					}
					return nil
				}},
				newClient: func(_ context.Context, _ *corev1.Secret) (client.Client, error) { return &test.MockClient{}, nil },
			},
			args: args{
				ctx: context.Background(),
				mg:  cockroachCluster(),
			},
			want: want{
				err: nil,
			},
		},
		"NotCockroachCluster": {
			conn: &connecter{},
			args: args{ctx: context.Background(), mg: &cockroachStrange{}},
			want: want{err: errors.New(errNotCockroachCluster)},
		},
		"FailedToGetProvider": {
			conn: &connecter{
				client: &test.MockClient{MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
					return errorBoom
				}},
			},
			args: args{ctx: context.Background(), mg: cockroachCluster()},
			want: want{err: errors.Wrap(errorBoom, errGetCockroachProvider)},
		},
		"FailedToGetProviderSecret": {
			conn: &connecter{
				client: &test.MockClient{MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
					switch key {
					case client.ObjectKey{Name: providerName}:
						*obj.(*kubev1alpha1.Provider) = provider
					case client.ObjectKey{Namespace: providerSecretNamespace, Name: providerSecretName}:
						return errorBoom
					}
					return nil
				}},
			},
			args: args{ctx: context.Background(), mg: cockroachCluster()},
			want: want{err: errors.Wrap(errorBoom, errGetCockroachProviderSecret)},
		},
		"FailedToCreateKubernetesClient": {
			conn: &connecter{
				client: &test.MockClient{MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
					switch key {
					case client.ObjectKey{Name: providerName}:
						*obj.(*kubev1alpha1.Provider) = provider
					case client.ObjectKey{Namespace: providerSecretNamespace, Name: providerSecretName}:
						*obj.(*corev1.Secret) = secret
					}
					return nil
				}},
				newClient: func(_ context.Context, _ *corev1.Secret) (client.Client, error) { return nil, errorBoom },
			},
			args: args{ctx: context.Background(), mg: cockroachCluster()},
			want: want{err: errors.Wrap(errorBoom, errNewCockroachClient)},
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

func TestObserveCockroach(t *testing.T) {
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
					if key == (client.ObjectKey{Name: name}) {
						*obj.(*rookv1alpha1.Cluster) = rookv1alpha1.Cluster{
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
				mg:  cockroachCluster(),
			},
			want: want{
				mg: cockroachCluster(
					withConditions(runtimev1alpha1.Available()),
					withBindingPhase(runtimev1alpha1.BindingPhaseUnbound)),
				observation: managed.ExternalObservation{
					ResourceExists:    true,
					ConnectionDetails: managed.ConnectionDetails{},
				},
			},
		},
		"ObservedClusterDoesNotExist": {
			client: &external{client: &test.MockClient{
				MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
					return errorCockroachNotFound
				}},
			},
			args: args{
				ctx: context.Background(),
				mg:  cockroachCluster(),
			},
			want: want{
				mg:          cockroachCluster(),
				observation: managed.ExternalObservation{ResourceExists: false},
			},
		},
		"NotCockroachCluster": {
			client: &external{},
			args: args{
				ctx: context.Background(),
				mg:  &cockroachStrange{},
			},
			want: want{
				mg:  &cockroachStrange{},
				err: errors.New(errNotCockroachCluster),
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

func TestCreateCockroach(t *testing.T) {
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
				mg:  cockroachCluster(),
			},
			want: want{
				mg: cockroachCluster(withConditions(runtimev1alpha1.Creating())),
			},
		},
		"NotCockroachCluster": {
			client: &external{},
			args: args{
				ctx: context.Background(),
				mg:  &cockroachStrange{},
			},
			want: want{
				mg:  &cockroachStrange{},
				err: errors.New(errNotCockroachCluster),
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
				mg:  cockroachCluster(),
			},
			want: want{
				mg:  cockroachCluster(withConditions(runtimev1alpha1.Creating())),
				err: errors.Wrap(errorBoom, errCreateCockroachCluster),
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

func TestUpdateCockroach(t *testing.T) {
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
						*obj.(*rookv1alpha1.Cluster) = *rookCockroachCluster(withNodeCount(4))
					}
					return nil
				},
				MockUpdate: func(_ context.Context, obj runtime.Object, _ ...client.UpdateOption) error {
					return nil
				},
			}},
			args: args{
				ctx: context.Background(),
				mg:  cockroachCluster(),
			},
			want: want{
				mg: cockroachCluster(),
			},
		},
		"UpdatedNotRequired": {
			client: &external{client: &test.MockClient{
				MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
					if key == (client.ObjectKey{Namespace: namespace, Name: name}) {
						*obj.(*rookv1alpha1.Cluster) = *rookCockroachCluster()
					}
					return nil
				},
			}},
			args: args{
				ctx: context.Background(),
				mg:  cockroachCluster(),
			},
			want: want{
				mg: cockroachCluster(),
			},
		},
		"NotCockroachCluster": {
			client: &external{},
			args: args{
				ctx: context.Background(),
				mg:  &cockroachStrange{},
			},
			want: want{
				mg:  &cockroachStrange{},
				err: errors.New(errNotCockroachCluster),
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
				mg:  cockroachCluster(),
			},
			want: want{
				mg:  cockroachCluster(),
				err: errors.Wrap(errorBoom, errGetCockroachCluster),
			},
		},
		"FailedToUpdateCluster": {
			client: &external{client: &test.MockClient{
				MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
					if key == (client.ObjectKey{Namespace: namespace, Name: name}) {
						*obj.(*rookv1alpha1.Cluster) = *rookCockroachCluster(withNodeCount(4))
					}
					return nil
				},
				MockUpdate: func(_ context.Context, obj runtime.Object, _ ...client.UpdateOption) error {
					return errorBoom
				},
			}},

			args: args{
				ctx: context.Background(),
				mg:  cockroachCluster(),
			},
			want: want{
				mg:  cockroachCluster(),
				err: errors.Wrap(errorBoom, errUpdateCockroachCluster),
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

func TestDeleteCockroach(t *testing.T) {
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
						*obj.(*rookv1alpha1.Cluster) = *rookCockroachCluster()
					}
					return nil
				},
				MockDelete: func(_ context.Context, obj runtime.Object, _ ...client.DeleteOption) error {
					return nil
				}},
			},
			args: args{
				ctx: context.Background(),
				mg:  cockroachCluster(),
			},
			want: want{
				mg: cockroachCluster(withConditions(runtimev1alpha1.Deleting())),
			},
		},
		"NotCockroachCluster": {
			client: &external{},
			args: args{
				ctx: context.Background(),
				mg:  &cockroachStrange{},
			},
			want: want{
				mg:  &cockroachStrange{},
				err: errors.New(errNotCockroachCluster),
			},
		},
		"FailedToDeleteCluster": {
			client: &external{client: &test.MockClient{
				MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
					if key == (client.ObjectKey{Namespace: namespace, Name: name}) {
						*obj.(*rookv1alpha1.Cluster) = *rookCockroachCluster()
					}
					return nil
				},
				MockDelete: func(_ context.Context, obj runtime.Object, _ ...client.DeleteOption) error {
					return errorBoom
				}},
			},

			args: args{
				ctx: context.Background(),
				mg:  cockroachCluster(),
			},
			want: want{
				mg:  cockroachCluster(withConditions(runtimev1alpha1.Deleting())),
				err: errors.Wrap(errorBoom, errDeleteCockroachCluster),
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
