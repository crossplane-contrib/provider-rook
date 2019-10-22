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
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	runtimev1alpha1 "github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane-runtime/pkg/meta"
	"github.com/crossplaneio/crossplane-runtime/pkg/resource"
	kubev1alpha1 "github.com/crossplaneio/crossplane/apis/kubernetes/v1alpha1"
	rookv1alpha1 "github.com/rook/rook/pkg/apis/yugabytedb.rook.io/v1alpha1"

	"github.com/crossplaneio/stack-rook/apis/database/v1alpha1"
	"github.com/crossplaneio/stack-rook/pkg/clients/database/yugabyte"
)

// Error strings.
const (
	errGetYugabyteProvider       = "cannot get Provider"
	errGetYugabyteProviderSecret = "cannot get Provider Secret"
	errNewYugabyteClient         = "cannot create new Kubernetes client"
	errNotYugabyteCluster        = "managed resource is not an Yugabyte cluster"
	errGetYugabyteCluster        = "cannot get Yugabyte cluster in target Kubernetes cluster"
	errCreateYugabyteCluster     = "cannot create Yugabyte cluster in target Kubernetes cluster"
	errUpdateYugabyteCluster     = "cannot update Yugabyte cluster in target Kubernetes cluster"
	errDeleteYugabyteCluster     = "cannot delete Yugabyte cluster in target Kubernetes cluster"
)

// YugabyteClusterController is responsible for adding the YugabyteCluster
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
			resource.WithExternalConnecter(&yugabyteConnecter{client: mgr.GetClient(), newClient: yugabyte.NewClient})))
}

type yugabyteConnecter struct {
	client    client.Client
	newClient func(ctx context.Context, secret *corev1.Secret) (client.Client, error)
}

func (c *yugabyteConnecter) Connect(ctx context.Context, mg resource.Managed) (resource.ExternalClient, error) {
	i, ok := mg.(*v1alpha1.YugabyteCluster)
	if !ok {
		return nil, errors.New(errNotYugabyteCluster)
	}

	p := &kubev1alpha1.Provider{}
	n := meta.NamespacedNameOf(i.Spec.ProviderReference)
	if err := c.client.Get(ctx, n, p); err != nil {
		return nil, errors.Wrap(err, errGetYugabyteProvider)
	}

	s := &corev1.Secret{}
	n = types.NamespacedName{Namespace: p.Namespace, Name: p.Spec.Secret.Name}
	if err := c.client.Get(ctx, n, s); err != nil {
		return nil, errors.Wrap(err, errGetYugabyteProviderSecret)
	}

	client, err := c.newClient(ctx, s)
	return &yugabyteExternal{client: client}, errors.Wrap(err, errNewYugabyteClient)
}

type yugabyteExternal struct {
	client client.Client
}

func (e *yugabyteExternal) Observe(ctx context.Context, mg resource.Managed) (resource.ExternalObservation, error) {
	c, ok := mg.(*v1alpha1.YugabyteCluster)
	if !ok {
		return resource.ExternalObservation{}, errors.New(errNotYugabyteCluster)
	}

	key := types.NamespacedName{
		Name:      c.Spec.YugabyteClusterParameters.Name,
		Namespace: c.Spec.YugabyteClusterParameters.Namespace,
	}

	external := &rookv1alpha1.YBCluster{}

	err := e.client.Get(ctx, key, external)
	if kerrors.IsNotFound(err) {
		return resource.ExternalObservation{ResourceExists: false}, nil
	}
	if err != nil {
		return resource.ExternalObservation{}, errors.Wrap(err, errGetYugabyteCluster)
	}

	// If we are able to get the resource YBCluster instance, we will consider
	// it available. If a status is added to the YBCluster CRD in the future we
	// should check it to set conditions.
	c.Status.SetConditions(runtimev1alpha1.Available())
	resource.SetBindable(c)

	o := resource.ExternalObservation{
		ResourceExists:    true,
		ConnectionDetails: resource.ConnectionDetails{},
	}

	return o, nil

}

func (e *yugabyteExternal) Create(ctx context.Context, mg resource.Managed) (resource.ExternalCreation, error) {
	c, ok := mg.(*v1alpha1.YugabyteCluster)
	if !ok {
		return resource.ExternalCreation{}, errors.New(errNotYugabyteCluster)
	}

	c.Status.SetConditions(runtimev1alpha1.Creating())

	err := e.client.Create(ctx, yugabyte.CrossToRook(c))
	return resource.ExternalCreation{}, errors.Wrap(err, errCreateYugabyteCluster)
}

func (e *yugabyteExternal) Update(ctx context.Context, mg resource.Managed) (resource.ExternalUpdate, error) {
	c, ok := mg.(*v1alpha1.YugabyteCluster)
	if !ok {
		return resource.ExternalUpdate{}, errors.New(errNotYugabyteCluster)
	}

	key := types.NamespacedName{
		Name:      c.Spec.YugabyteClusterParameters.Name,
		Namespace: c.Spec.YugabyteClusterParameters.Namespace,
	}

	external := &rookv1alpha1.YBCluster{}

	if err := e.client.Get(ctx, key, external); err != nil {
		return resource.ExternalUpdate{}, errors.Wrap(err, errGetYugabyteCluster)
	}

	if !yugabyte.NeedsUpdate(c, external) {
		return resource.ExternalUpdate{}, nil
	}

	update := yugabyte.CrossToRook(c)
	update.ResourceVersion = external.ResourceVersion
	err := e.client.Update(ctx, update)
	return resource.ExternalUpdate{}, errors.Wrap(err, errUpdateYugabyteCluster)
}

func (e *yugabyteExternal) Delete(ctx context.Context, mg resource.Managed) error {
	c, ok := mg.(*v1alpha1.YugabyteCluster)
	if !ok {
		return errors.New(errNotYugabyteCluster)
	}

	c.SetConditions(runtimev1alpha1.Deleting())

	key := types.NamespacedName{
		Name:      c.Spec.YugabyteClusterParameters.Name,
		Namespace: c.Spec.YugabyteClusterParameters.Namespace,
	}

	external := &rookv1alpha1.YBCluster{}

	if err := e.client.Get(ctx, key, external); err != nil {
		if kerrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrap(err, errGetYugabyteCluster)
	}

	err := e.client.Delete(ctx, external)
	return errors.Wrap(err, errDeleteYugabyteCluster)
}
