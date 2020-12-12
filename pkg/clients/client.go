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

package clients

import (
	"context"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/crossplane/provider-rook/apis/v1beta1"
)

const (
	errGetProviderConfig     = "cannot get referenced ProviderConfig"
	errTrackUsage            = "cannot track ProviderConfig usage"
	errNoSecretRef           = "no connection secret reference was supplied"
	errGetSecret             = "cannot get referenced credentials secret"
	errNoRefGiven            = "neither providerConfigRef nor providerRef was supplied"
	errConstructClientConfig = "cannot construct a client config from data in the credentials secret"
	errConstructRestConfig   = "cannot construct a rest config from client config"
	errNewClient             = "cannot create a new controller-runtime client"

	errFmtUnsupportedCredSource = "unsupported credentials secret source %q"
)

// NewClient returns a kubernetes client with the information in provider config
// reference of given managed resource. If the reference is a
// ProviderConfigReference, the Rook ProviderConfig is used. If the reference is
// a ProviderReference, the deprecated core Crossplane Kubernetes Provider is
// used.
func NewClient(ctx context.Context, c client.Client, mg resource.Managed, s *runtime.Scheme) (client.Client, error) { // nolint:gocyclo
	switch {
	case mg.GetProviderConfigReference() != nil:
		return UseProviderConfig(ctx, c, mg, s)
	default:
		return nil, errors.New(errNoRefGiven)
	}
}

// UseProviderConfig to create a client.
func UseProviderConfig(ctx context.Context, c client.Client, mg resource.Managed, s *runtime.Scheme) (client.Client, error) {
	pc := &v1beta1.ProviderConfig{}
	if err := c.Get(ctx, types.NamespacedName{Name: mg.GetProviderConfigReference().Name}, pc); err != nil {
		return nil, errors.Wrap(err, errGetProviderConfig)
	}

	t := resource.NewProviderConfigUsageTracker(c, &v1beta1.ProviderConfigUsage{})
	if err := t.Track(ctx, mg); err != nil {
		return nil, errors.Wrap(err, errTrackUsage)
	}

	if s := pc.Spec.Credentials.Source; s != xpv1.CredentialsSourceSecret {
		return c, errors.Errorf(errFmtUnsupportedCredSource, s)
	}

	ref := pc.Spec.Credentials.SecretRef
	if ref == nil {
		return c, errors.New(errNoSecretRef)
	}

	secret := &corev1.Secret{}
	if err := c.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: ref.Namespace}, secret); err != nil {
		return nil, errors.Wrap(err, errGetSecret)
	}

	cfg, err := clientcmd.NewClientConfigFromBytes(secret.Data[ref.Key])
	if err != nil {
		return nil, errors.Wrap(err, errConstructClientConfig)
	}
	restCfg, err := cfg.ClientConfig()
	if err != nil {
		return nil, errors.Wrap(err, errConstructRestConfig)
	}

	kc, err := client.New(restCfg, client.Options{Scheme: s})
	if err != nil {
		return nil, errors.Wrap(err, errNewClient)
	}

	return kc, nil
}
