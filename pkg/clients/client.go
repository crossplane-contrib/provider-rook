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

	runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane/apis/kubernetes/v1alpha1"

	"github.com/crossplane/provider-rook/apis/v1beta1"
)

const (
	errGetProviderConfig     = "cannot get referenced ProviderConfig"
	errGetProvider           = "cannot get referenced Provider"
	errGetSecret             = "cannot get referenced credentials secret"
	errNoRefGiven            = "neither providerConfigRef nor providerRef is given"
	errConstructClientConfig = "cannot construct a client config from data in the credentials secret"
	errConstructRestConfig   = "cannot construct a rest config from client config"
	errNewClient             = "cannot create a new controller-runtime client"
)

// NewClient returns a kubernetes client with the information in provider config
// reference of given managed resource. If the reference is a
// ProviderConfigReference, the Rook ProviderConfig is used. If the reference is
// a ProviderReference, the deprecated core Crossplane Kubernetes Provider is
// used.
func NewClient(ctx context.Context, kube client.Client, mg resource.Managed, scheme *runtime.Scheme) (client.Client, error) { // nolint:gocyclo
	pc := &v1beta1.ProviderConfig{}
	switch {
	case mg.GetProviderConfigReference() != nil && mg.GetProviderConfigReference().Name != "":
		nn := types.NamespacedName{Name: mg.GetProviderConfigReference().Name}
		if err := kube.Get(ctx, nn, pc); err != nil {
			return nil, errors.Wrap(err, errGetProviderConfig)
		}

		t := resource.NewProviderConfigUsageTracker(kube, &v1beta1.ProviderConfigUsage{})
		if err := t.Track(ctx, mg); err != nil {
			return nil, errors.Wrap(err, "cannot track ProviderConfigUsage")
		}
	case mg.GetProviderReference() != nil && mg.GetProviderReference().Name != "":
		nn := types.NamespacedName{Name: mg.GetProviderReference().Name}
		p := &v1alpha1.Provider{}
		if err := kube.Get(ctx, nn, p); err != nil {
			return nil, errors.Wrap(err, errGetProvider)
		}
		p.ObjectMeta.DeepCopyInto(&pc.ObjectMeta)
		pc.Spec.CredentialsSecretRef = &runtimev1alpha1.SecretKeySelector{
			SecretReference: runtimev1alpha1.SecretReference{
				Name:      p.Spec.Secret.Name,
				Namespace: p.Spec.Secret.Namespace,
			},
			Key: runtimev1alpha1.ResourceCredentialsSecretKubeconfigKey,
		}
	default:
		return nil, errors.New(errNoRefGiven)
	}

	s := &corev1.Secret{}
	nn := types.NamespacedName{Name: pc.Spec.CredentialsSecretRef.Name, Namespace: pc.Spec.CredentialsSecretRef.Namespace}
	if err := kube.Get(ctx, nn, s); err != nil {
		return nil, errors.Wrap(err, errGetSecret)
	}

	cfg, err := clientcmd.NewClientConfigFromBytes(s.Data[pc.Spec.CredentialsSecretRef.Key])
	if err != nil {
		return nil, errors.Wrap(err, errConstructClientConfig)
	}
	restCfg, err := cfg.ClientConfig()
	if err != nil {
		return nil, errors.Wrap(err, errConstructRestConfig)
	}

	kc, err := client.New(restCfg, client.Options{Scheme: scheme})
	if err != nil {
		return nil, errors.Wrap(err, errNewClient)
	}

	return kc, nil
}
