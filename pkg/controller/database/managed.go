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
	"fmt"
	"strings"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	runtimev1alpha1 "github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane-runtime/pkg/meta"
	"github.com/crossplaneio/crossplane-runtime/pkg/resource"
	kubev1alpha1 "github.com/crossplaneio/crossplane/apis/kubernetes/v1alpha1"
	rookv1alpha1 "github.com/rook/rook/pkg/apis/yugabytedb.rook.io/v1alpha1"

	"github.com/crossplaneio/stack-rook/apis/database/v1alpha1"
	"github.com/crossplaneio/stack-rook/pkg/clients"
	"github.com/crossplaneio/stack-rook/pkg/clients/database/yugabyte"
)

// Error strings.
const (
	errGetProvider       = "cannot get Provider"
	errGetProviderSecret = "cannot get Provider Secret"
	errNewClient         = "cannot create new Kubernetes client"
	errNotCluster        = "managed resource is not an Yugabyte cluster"
	errGetCluster        = "cannot get Yugabyte cluster in target Kubernetes cluster"
	errCreateCluster     = "cannot create Yugabyte cluster in target Kubernetes cluster"
	errUpdateCluster     = "cannot update Yugabyte cluster in target Kubernetes cluster"
	errDeleteCluster     = "cannot delete Yugabyte cluster in target Kubernetes cluster"
)

// YugabyteClusterController is responsible for adding the Cloud Memorystore
// controller and its corresponding reconciler to the manager with any runtime configuration.
type YugabyteClusterController struct{}

// SetupWithManager creates a new YugabyteCluster Controller and adds it to the
// Manager with default RBAC. The Manager will set fields on the Controller and
// start it when the Manager is Started.
func (c *YugabyteClusterController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named(strings.ToLower(fmt.Sprintf("%s.%s", v1alpha1.YugabyteClusterKind, v1alpha1.Group))).
		For(&v1alpha1.YugabyteCluster{}).
		Complete(resource.NewManagedReconciler(mgr,
			resource.ManagedKind(v1alpha1.YugabyteClusterGroupVersionKind),
			resource.WithExternalConnecter(&connecter{client: mgr.GetClient(), newClient: clients.NewClient})))
}

type connecter struct {
	client    client.Client
	newClient func(ctx context.Context, secret *corev1.Secret) (client.Client, error)
}

func (c *connecter) Connect(ctx context.Context, mg resource.Managed) (resource.ExternalClient, error) {
	i, ok := mg.(*v1alpha1.YugabyteCluster)
	if !ok {
		return nil, errors.New(errNotCluster)
	}

	p := &kubev1alpha1.Provider{}
	n := meta.NamespacedNameOf(i.Spec.ProviderReference)
	if err := c.client.Get(ctx, n, p); err != nil {
		return nil, errors.Wrap(err, errGetProvider)
	}

	s := &corev1.Secret{}
	n = types.NamespacedName{Namespace: p.Namespace, Name: p.Spec.Secret.Name}
	if err := c.client.Get(ctx, n, s); err != nil {
		return nil, errors.Wrap(err, errGetProviderSecret)
	}

	client, err := c.newClient(ctx, s)
	return &external{client: client}, errors.Wrap(err, errNewClient)
}

type external struct {
	client client.Client
}

func (e *external) Observe(ctx context.Context, mg resource.Managed) (resource.ExternalObservation, error) {
	c, ok := mg.(*v1alpha1.YugabyteCluster)
	if !ok {
		return resource.ExternalObservation{}, errors.New(errNotCluster)
	}

	key := types.NamespacedName{
		Name:      c.Name,
		Namespace: c.Namespace,
	}

	external := &rookv1alpha1.YBCluster{}

	if err := e.client.Get(ctx, key, external); err != nil {
		// TODO(hasheddan): check error to see if it is due to object not existing
		return resource.ExternalObservation{ResourceExists: false}, nil
	}

	c.Status.SetConditions(runtimev1alpha1.Available())
	resource.SetBindable(c)

	// TODO(hasheddan): what determines if needs update?

	o := resource.ExternalObservation{
		ResourceExists:    true,
		ConnectionDetails: resource.ConnectionDetails{},
	}

	return o, nil

}

func (e *external) Create(ctx context.Context, mg resource.Managed) (resource.ExternalCreation, error) {
	c, ok := mg.(*v1alpha1.YugabyteCluster)
	if !ok {
		return resource.ExternalCreation{}, errors.New(errNotCluster)
	}

	c.Status.SetConditions(runtimev1alpha1.Creating())

	err := e.client.Create(ctx, yugabyte.CrossToRook(c))
	return resource.ExternalCreation{}, errors.Wrap(err, errCreateCluster)
}

func (e *external) Update(ctx context.Context, mg resource.Managed) (resource.ExternalUpdate, error) {
	c, ok := mg.(*v1alpha1.YugabyteCluster)
	if !ok {
		return resource.ExternalUpdate{}, errors.New(errNotCluster)
	}

	key := types.NamespacedName{
		Name:      c.Name,
		Namespace: c.Namespace,
	}

	external := &rookv1alpha1.YBCluster{}

	if err := e.client.Get(ctx, key, external); err != nil {
		return resource.ExternalUpdate{}, errors.Wrap(err, errGetCluster)
	}

	err := e.client.Update(ctx, external)
	return resource.ExternalUpdate{}, errors.Wrap(err, errUpdateCluster)
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) error {
	c, ok := mg.(*v1alpha1.YugabyteCluster)
	if !ok {
		return errors.New(errNotCluster)
	}

	c.SetConditions(runtimev1alpha1.Deleting())

	key := types.NamespacedName{
		Name:      c.Name,
		Namespace: c.Namespace,
	}

	external := &rookv1alpha1.YBCluster{}

	if err := e.client.Get(ctx, key, external); err != nil {
		return errors.Wrap(err, errGetCluster)
	}

	err := e.client.Delete(ctx, external)
	return errors.Wrap(err, errDeleteCluster)
}
