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

package cache

import (
	"context"
	"testing"

	redisv1 "cloud.google.com/go/redis/apiv1"
	"github.com/google/go-cmp/cmp"
	"github.com/googleapis/gax-go"
	"github.com/pkg/errors"
	redisv1pb "google.golang.org/genproto/googleapis/cloud/redis/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	runtimev1alpha1 "github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane-runtime/pkg/resource"
	"github.com/crossplaneio/crossplane-runtime/pkg/test"

	"github.com/crossplaneio/stack-gcp/apis/cache/v1alpha2"
	gcpv1alpha2 "github.com/crossplaneio/stack-gcp/apis/v1alpha2"
	"github.com/crossplaneio/stack-gcp/pkg/clients/cloudmemorystore"
	"github.com/crossplaneio/stack-gcp/pkg/clients/cloudmemorystore/fake"
)

const (
	namespace         = "cool-namespace"
	uid               = types.UID("definitely-a-uuid")
	region            = "us-cool1"
	project           = "coolProject"
	instanceName      = cloudmemorystore.NamePrefix + "-" + string(uid)
	qualifiedName     = "projects/" + project + "/locations/" + region + "/instances/" + instanceName
	authorizedNetwork = "default"
	memorySizeGB      = 1
	host              = "172.16.0.1"
	port              = 6379

	providerName       = "cool-gcp"
	providerSecretName = "cool-gcp-secret"
	providerSecretKey  = "credentials.json"
	providerSecretData = "definitelyjson"

	connectionSecretName = "cool-connection-secret"
)

var (
	errorBoom    = errors.New("boom")
	redisConfigs = map[string]string{"cool": "socool"}
)

type strange struct {
	resource.Managed
}

type instanceModifier func(*v1alpha2.CloudMemorystoreInstance)

func withConditions(c ...runtimev1alpha1.Condition) instanceModifier {
	return func(i *v1alpha2.CloudMemorystoreInstance) { i.Status.SetConditions(c...) }
}

func withBindingPhase(p runtimev1alpha1.BindingPhase) instanceModifier {
	return func(i *v1alpha2.CloudMemorystoreInstance) { i.Status.SetBindingPhase(p) }
}

func withState(s string) instanceModifier {
	return func(i *v1alpha2.CloudMemorystoreInstance) { i.Status.State = s }
}

func withProviderID(id string) instanceModifier {
	return func(i *v1alpha2.CloudMemorystoreInstance) { i.Status.ProviderID = id }
}

func withEndpoint(e string) instanceModifier {
	return func(i *v1alpha2.CloudMemorystoreInstance) { i.Status.Endpoint = e }
}

func withPort(p int) instanceModifier {
	return func(i *v1alpha2.CloudMemorystoreInstance) { i.Status.Port = p }
}

func instance(im ...instanceModifier) *v1alpha2.CloudMemorystoreInstance {
	i := &v1alpha2.CloudMemorystoreInstance{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:  namespace,
			Name:       instanceName,
			UID:        uid,
			Finalizers: []string{},
		},
		Spec: v1alpha2.CloudMemorystoreInstanceSpec{
			ResourceSpec: runtimev1alpha1.ResourceSpec{
				ProviderReference:                &corev1.ObjectReference{Namespace: namespace, Name: providerName},
				WriteConnectionSecretToReference: corev1.LocalObjectReference{Name: connectionSecretName},
			},
			CloudMemorystoreInstanceParameters: v1alpha2.CloudMemorystoreInstanceParameters{
				MemorySizeGB:      memorySizeGB,
				RedisConfigs:      redisConfigs,
				AuthorizedNetwork: authorizedNetwork,
			},
		},
	}

	for _, m := range im {
		m(i)
	}

	return i
}

var _ resource.ExternalClient = &external{}
var _ resource.ExternalConnecter = &connecter{}

func TestConnect(t *testing.T) {
	provider := gcpv1alpha2.Provider{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: providerName},
		Spec: gcpv1alpha2.ProviderSpec{
			ProjectID: project,
			Secret: corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: providerSecretName},
				Key:                  providerSecretKey,
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
			conn: &connecter{
				client: &test.MockClient{MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
					switch key {
					case client.ObjectKey{Namespace: namespace, Name: providerName}:
						*obj.(*gcpv1alpha2.Provider) = provider
					case client.ObjectKey{Namespace: namespace, Name: providerSecretName}:
						*obj.(*corev1.Secret) = secret
					}
					return nil
				}},
				newCMS: func(_ context.Context, _ []byte) (cloudmemorystore.Client, error) { return nil, nil },
			},
			args: args{
				ctx: context.Background(),
				mg:  instance(),
			},
			want: want{
				err: nil,
			},
		},
		"NotCloudMemorystoreInstance": {
			conn: &connecter{},
			args: args{ctx: context.Background(), mg: &strange{}},
			want: want{err: errors.New(errNotInstance)},
		},
		"FailedToGetProvider": {
			conn: &connecter{
				client: &test.MockClient{MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
					return errorBoom
				}},
			},
			args: args{ctx: context.Background(), mg: instance()},
			want: want{err: errors.Wrap(errorBoom, errGetProvider)},
		},
		"FailedToGetProviderSecret": {
			conn: &connecter{
				client: &test.MockClient{MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
					switch key {
					case client.ObjectKey{Namespace: namespace, Name: providerName}:
						*obj.(*gcpv1alpha2.Provider) = provider
					case client.ObjectKey{Namespace: namespace, Name: providerSecretName}:
						return errorBoom
					}
					return nil
				}},
			},
			args: args{ctx: context.Background(), mg: instance()},
			want: want{err: errors.Wrap(errorBoom, errGetProviderSecret)},
		},
		"FailedToCreateCloudMemorystoreClient": {
			conn: &connecter{
				client: &test.MockClient{MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
					switch key {
					case client.ObjectKey{Namespace: namespace, Name: providerName}:
						*obj.(*gcpv1alpha2.Provider) = provider
					case client.ObjectKey{Namespace: namespace, Name: providerSecretName}:
						*obj.(*corev1.Secret) = secret
					}
					return nil
				}},
				newCMS: func(_ context.Context, _ []byte) (cloudmemorystore.Client, error) { return nil, errorBoom },
			},
			args: args{ctx: context.Background(), mg: instance()},
			want: want{err: errors.Wrap(errorBoom, errNewClient)},
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
		"ObservedInstanceAvailable": {
			client: &external{cms: &fake.MockClient{
				MockGetInstance: func(_ context.Context, _ *redisv1pb.GetInstanceRequest, _ ...gax.CallOption) (*redisv1pb.Instance, error) {
					return &redisv1pb.Instance{
						State: redisv1pb.Instance_READY,
						Host:  host,
						Port:  port,
						Name:  qualifiedName,
					}, nil
				}},
			},
			args: args{
				ctx: context.Background(),
				mg:  instance(),
			},
			want: want{
				mg: instance(
					withConditions(runtimev1alpha1.Available()),
					withBindingPhase(runtimev1alpha1.BindingPhaseUnbound),
					withState(v1alpha2.StateReady),
					withEndpoint(host),
					withPort(port),
					withProviderID(qualifiedName)),
				observation: resource.ExternalObservation{
					ResourceExists: true,
					ConnectionDetails: resource.ConnectionDetails{
						runtimev1alpha1.ResourceCredentialsSecretEndpointKey: []byte(host),
					},
				},
			},
		},
		"ObservedInstanceCreating": {
			client: &external{cms: &fake.MockClient{
				MockGetInstance: func(_ context.Context, _ *redisv1pb.GetInstanceRequest, _ ...gax.CallOption) (*redisv1pb.Instance, error) {
					return &redisv1pb.Instance{
						State: redisv1pb.Instance_CREATING,
						Name:  qualifiedName,
					}, nil
				}},
			},
			args: args{
				ctx: context.Background(),
				mg:  instance(),
			},
			want: want{
				mg: instance(
					withConditions(runtimev1alpha1.Creating()),
					withState(v1alpha2.StateCreating),
					withProviderID(qualifiedName)),
				observation: resource.ExternalObservation{
					ResourceExists:    true,
					ConnectionDetails: resource.ConnectionDetails{},
				},
			},
		},
		"ObservedInstanceDeleting": {
			client: &external{cms: &fake.MockClient{
				MockGetInstance: func(_ context.Context, _ *redisv1pb.GetInstanceRequest, _ ...gax.CallOption) (*redisv1pb.Instance, error) {
					return &redisv1pb.Instance{
						State: redisv1pb.Instance_DELETING,
						Name:  qualifiedName,
					}, nil
				}},
			},
			args: args{
				ctx: context.Background(),
				mg:  instance(),
			},
			want: want{
				mg: instance(
					withConditions(runtimev1alpha1.Deleting()),
					withState(v1alpha2.StateDeleting),
					withProviderID(qualifiedName)),
				observation: resource.ExternalObservation{
					ResourceExists:    true,
					ConnectionDetails: resource.ConnectionDetails{},
				},
			},
		},
		"ObservedInstanceDoesNotExist": {
			client: &external{cms: &fake.MockClient{
				MockGetInstance: func(_ context.Context, _ *redisv1pb.GetInstanceRequest, _ ...gax.CallOption) (*redisv1pb.Instance, error) {
					return nil, status.Error(codes.NotFound, "wat")
				}},
			},
			args: args{
				ctx: context.Background(),
				mg:  instance(),
			},
			want: want{
				mg:          instance(),
				observation: resource.ExternalObservation{ResourceExists: false},
			},
		},
		"NotCloudMemorystoreInstance": {
			client: &external{},
			args: args{
				ctx: context.Background(),
				mg:  &strange{},
			},
			want: want{
				mg:  &strange{},
				err: errors.New(errNotInstance),
			},
		},
		"FailedToGetInstance": {
			client: &external{cms: &fake.MockClient{
				MockGetInstance: func(_ context.Context, _ *redisv1pb.GetInstanceRequest, _ ...gax.CallOption) (*redisv1pb.Instance, error) {
					return nil, errorBoom
				}},
			},
			args: args{
				ctx: context.Background(),
				mg:  instance(),
			},
			want: want{
				mg:  instance(),
				err: errors.Wrap(errorBoom, errGetInstance),
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
		"CreatedInstance": {
			client: &external{cms: &fake.MockClient{
				MockCreateInstance: func(_ context.Context, _ *redisv1pb.CreateInstanceRequest, _ ...gax.CallOption) (*redisv1.CreateInstanceOperation, error) {
					return nil, nil
				}},
			},
			args: args{
				ctx: context.Background(),
				mg:  instance(),
			},
			want: want{
				mg: instance(withConditions(runtimev1alpha1.Creating())),
			},
		},
		"NotCloudMemorystoreInstance": {
			client: &external{},
			args: args{
				ctx: context.Background(),
				mg:  &strange{},
			},
			want: want{
				mg:  &strange{},
				err: errors.New(errNotInstance),
			},
		},
		"FailedToCreateInstance": {
			client: &external{cms: &fake.MockClient{
				MockCreateInstance: func(_ context.Context, _ *redisv1pb.CreateInstanceRequest, _ ...gax.CallOption) (*redisv1.CreateInstanceOperation, error) {
					return nil, errorBoom
				},
			}},

			args: args{
				ctx: context.Background(),
				mg:  instance(),
			},
			want: want{
				mg:  instance(withConditions(runtimev1alpha1.Creating())),
				err: errors.Wrap(errorBoom, errCreateInstance),
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
			client: &external{cms: &fake.MockClient{
				MockGetInstance: func(_ context.Context, _ *redisv1pb.GetInstanceRequest, _ ...gax.CallOption) (*redisv1pb.Instance, error) {
					return &redisv1pb.Instance{}, nil
				},
				MockUpdateInstance: func(_ context.Context, _ *redisv1pb.UpdateInstanceRequest, _ ...gax.CallOption) (*redisv1.UpdateInstanceOperation, error) {
					return nil, nil
				},
			}},
			args: args{
				ctx: context.Background(),
				mg:  instance(),
			},
			want: want{
				mg: instance(withConditions()),
			},
		},
		"UpdateNotRequired": {
			client: &external{cms: &fake.MockClient{
				MockGetInstance: func(_ context.Context, _ *redisv1pb.GetInstanceRequest, _ ...gax.CallOption) (*redisv1pb.Instance, error) {
					return &redisv1pb.Instance{
						MemorySizeGb: memorySizeGB,
						RedisConfigs: redisConfigs,
					}, nil
				},
			}},
			args: args{
				ctx: context.Background(),
				mg:  instance(),
			},
			want: want{
				mg: instance(withConditions()),
			},
		},
		"NotCloudMemorystoreInstance": {
			client: &external{},
			args: args{
				ctx: context.Background(),
				mg:  &strange{},
			},
			want: want{
				mg:  &strange{},
				err: errors.New(errNotInstance),
			},
		},
		"FailedToGetInstance": {
			client: &external{cms: &fake.MockClient{
				MockGetInstance: func(_ context.Context, _ *redisv1pb.GetInstanceRequest, _ ...gax.CallOption) (*redisv1pb.Instance, error) {
					return nil, errorBoom
				}},
			},
			args: args{
				ctx: context.Background(),
				mg:  instance(),
			},
			want: want{
				mg:  instance(),
				err: errors.Wrap(errorBoom, errGetInstance),
			},
		},
		"FailedToUpdateInstance": {
			client: &external{cms: &fake.MockClient{
				MockGetInstance: func(_ context.Context, _ *redisv1pb.GetInstanceRequest, _ ...gax.CallOption) (*redisv1pb.Instance, error) {
					return &redisv1pb.Instance{}, nil
				},
				MockUpdateInstance: func(_ context.Context, _ *redisv1pb.UpdateInstanceRequest, _ ...gax.CallOption) (*redisv1.UpdateInstanceOperation, error) {
					return nil, errorBoom
				},
			}},

			args: args{
				ctx: context.Background(),
				mg:  instance(),
			},
			want: want{
				mg:  instance(),
				err: errors.Wrap(errorBoom, errUpdateInstance),
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
		"DeletedInstance": {
			client: &external{cms: &fake.MockClient{
				MockDeleteInstance: func(_ context.Context, _ *redisv1pb.DeleteInstanceRequest, _ ...gax.CallOption) (*redisv1.DeleteInstanceOperation, error) {
					return nil, nil
				}},
			},
			args: args{
				ctx: context.Background(),
				mg:  instance(),
			},
			want: want{
				mg: instance(withConditions(runtimev1alpha1.Deleting())),
			},
		},
		"NotCloudMemorystoreInstance": {
			client: &external{},
			args: args{
				ctx: context.Background(),
				mg:  &strange{},
			},
			want: want{
				mg:  &strange{},
				err: errors.New(errNotInstance),
			},
		},
		"FailedToDeleteInstance": {
			client: &external{cms: &fake.MockClient{
				MockDeleteInstance: func(_ context.Context, _ *redisv1pb.DeleteInstanceRequest, _ ...gax.CallOption) (*redisv1.DeleteInstanceOperation, error) {
					return nil, errorBoom
				},
			}},

			args: args{
				ctx: context.Background(),
				mg:  instance(),
			},
			want: want{
				mg:  instance(withConditions(runtimev1alpha1.Deleting())),
				err: errors.Wrap(errorBoom, errDeleteInstance),
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
