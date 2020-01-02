/*
Copyright 2020 The Crossplane Authors.

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
	"net/url"

	runtimev1alpha1 "github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NewClient returns a kubernetes client given a secret with connection
// information.
func NewClient(ctx context.Context, s *corev1.Secret, scheme *runtime.Scheme) (client.Client, error) {

	u, err := url.Parse(string(s.Data[runtimev1alpha1.ResourceCredentialsSecretEndpointKey]))
	if err != nil {
		return nil, errors.Wrapf(err, "cannot parse Kubernetes endpoint as URL")
	}

	config := &rest.Config{
		Host:     u.String(),
		Username: string(s.Data[runtimev1alpha1.ResourceCredentialsSecretUserKey]),
		Password: string(s.Data[runtimev1alpha1.ResourceCredentialsSecretPasswordKey]),
		TLSClientConfig: rest.TLSClientConfig{
			// This field's godoc claims clients will use 'the hostname used to
			// contact the server' when it is left unset. In practice clients
			// appear to use the URL, including scheme and port.
			ServerName: u.Hostname(),
			CAData:     s.Data[runtimev1alpha1.ResourceCredentialsSecretCAKey],
			CertData:   s.Data[runtimev1alpha1.ResourceCredentialsSecretClientCertKey],
			KeyData:    s.Data[runtimev1alpha1.ResourceCredentialsSecretClientKeyKey],
		},
		BearerToken: string(s.Data[runtimev1alpha1.ResourceCredentialsSecretTokenKey]),
	}

	kc, err := client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		return nil, errors.Wrap(err, "cannot create Kubernetes client")
	}

	return kc, nil
}
