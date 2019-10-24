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
	"fmt"
	"strings"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	runtimev1alpha1 "github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane-runtime/pkg/meta"
	"github.com/crossplaneio/crossplane-runtime/pkg/resource"
	kubev1alpha1 "github.com/crossplaneio/crossplane/apis/kubernetes/v1alpha1"
	rookv1alpha1 "github.com/rook/rook/pkg/apis/cockroachdb.rook.io/v1alpha1"

	"github.com/crossplaneio/stack-rook/apis/database/v1alpha1"
	"github.com/crossplaneio/stack-rook/pkg/clients/database/cockroach"
)

// Error strings.
const (
	errGetCockroachProvider       = "cannot get Provider"
	errGetCockroachProviderSecret = "cannot get Provider Secret"
	errNewCockroachClient         = "cannot create new Kubernetes client"
	errNotCockroachCluster        = "managed resource is not an Cockroach cluster"
	errGetCockroachCluster        = "cannot get Cockroach cluster in target Kubernetes cluster"
	errCreateCockroachCluster     = "cannot create Cockroach cluster in target Kubernetes cluster"
	errUpdateCockroachCluster     = "cannot update Cockroach cluster in target Kubernetes cluster"
	errDeleteCockroachCluster     = "cannot delete Cockroach cluster in target Kubernetes cluster"
)

// Controller is responsible for adding the CockroachCluster
// controller and its corresponding reconciler to the manager with any runtime configuration.
type Controller struct{}

// SetupWithManager creates a new CockroachCluster Controller and adds it to the
// Manager with default RBAC. The Manager will set fields on the Controller and
// start it when the Manager is Started.
func (c *Controller) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named(strings.ToLower(fmt.Sprintf("%s.%s", v1alpha1.CockroachClusterKind, v1alpha1.Group))).
		For(&v1alpha1.CockroachCluster{}).
		Complete(resource.NewManagedReconciler(mgr,
			resource.ManagedKind(v1alpha1.CockroachClusterGroupVersionKind),
			resource.WithExternalConnecter(&connecter{client: mgr.GetClient(), newClient: cockroach.NewClient})))
}

type connecter struct {
	client    client.Client
	newClient func(ctx context.Context, secret *corev1.Secret) (client.Client, error)
}

func (c *connecter) Connect(ctx context.Context, mg resource.Managed) (resource.ExternalClient, error) {
	i, ok := mg.(*v1alpha1.CockroachCluster)
	if !ok {
		return nil, errors.New(errNotCockroachCluster)
	}

	p := &kubev1alpha1.Provider{}
	n := meta.NamespacedNameOf(i.Spec.ProviderReference)
	if err := c.client.Get(ctx, n, p); err != nil {
		return nil, errors.Wrap(err, errGetCockroachProvider)
	}

	s := &corev1.Secret{}
	n = types.NamespacedName{Namespace: p.Spec.Secret.Namespace, Name: p.Spec.Secret.Name}
	if err := c.client.Get(ctx, n, s); err != nil {
		return nil, errors.Wrap(err, errGetCockroachProviderSecret)
	}

	client, err := c.newClient(ctx, s)
	return &external{client: client}, errors.Wrap(err, errNewCockroachClient)
}

type external struct {
	client client.Client
}

func (e *external) Observe(ctx context.Context, mg resource.Managed) (resource.ExternalObservation, error) {
	c, ok := mg.(*v1alpha1.CockroachCluster)
	if !ok {
		return resource.ExternalObservation{}, errors.New(errNotCockroachCluster)
	}

	key := types.NamespacedName{
		Name:      c.Spec.CockroachClusterParameters.Name,
		Namespace: c.Spec.CockroachClusterParameters.Namespace,
	}

	external := &rookv1alpha1.Cluster{}

	err := e.client.Get(ctx, key, external)
	if kerrors.IsNotFound(err) {
		return resource.ExternalObservation{ResourceExists: false}, nil
	}
	if err != nil {
		return resource.ExternalObservation{}, errors.Wrap(err, errGetCockroachCluster)
	}

	// If we are able to get the resource Cluster instance, we will consider
	// it available. If a status is added to the Cluster CRD in the future we
	// should check it to set conditions.
	c.Status.SetConditions(runtimev1alpha1.Available())
	resource.SetBindable(c)

	o := resource.ExternalObservation{
		ResourceExists:    true,
		ConnectionDetails: resource.ConnectionDetails{},
	}

	return o, nil

}

func (e *external) Create(ctx context.Context, mg resource.Managed) (resource.ExternalCreation, error) {
	c, ok := mg.(*v1alpha1.CockroachCluster)
	if !ok {
		return resource.ExternalCreation{}, errors.New(errNotCockroachCluster)
	}

	c.Status.SetConditions(runtimev1alpha1.Creating())

	err := e.client.Create(ctx, cockroach.CrossToRook(c))
	return resource.ExternalCreation{}, errors.Wrap(err, errCreateCockroachCluster)
}

func (e *external) Update(ctx context.Context, mg resource.Managed) (resource.ExternalUpdate, error) {
	c, ok := mg.(*v1alpha1.CockroachCluster)
	if !ok {
		return resource.ExternalUpdate{}, errors.New(errNotCockroachCluster)
	}

	key := types.NamespacedName{
		Name:      c.Spec.CockroachClusterParameters.Name,
		Namespace: c.Spec.CockroachClusterParameters.Namespace,
	}

	external := &rookv1alpha1.Cluster{}

	if err := e.client.Get(ctx, key, external); err != nil {
		return resource.ExternalUpdate{}, errors.Wrap(err, errGetCockroachCluster)
	}

	if !cockroach.NeedsUpdate(c, external) {
		return resource.ExternalUpdate{}, nil
	}

	update := cockroach.CrossToRook(c)
	update.ResourceVersion = external.ResourceVersion
	err := e.client.Update(ctx, update)
	return resource.ExternalUpdate{}, errors.Wrap(err, errUpdateCockroachCluster)
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) error {
	c, ok := mg.(*v1alpha1.CockroachCluster)
	if !ok {
		return errors.New(errNotCockroachCluster)
	}

	c.SetConditions(runtimev1alpha1.Deleting())

	key := types.NamespacedName{
		Name:      c.Spec.CockroachClusterParameters.Name,
		Namespace: c.Spec.CockroachClusterParameters.Namespace,
	}

	external := &rookv1alpha1.Cluster{}

	if err := e.client.Get(ctx, key, external); err != nil {
		if kerrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrap(err, errGetCockroachCluster)
	}

	err := e.client.Delete(ctx, external)
	return errors.Wrap(err, errDeleteCockroachCluster)
}
