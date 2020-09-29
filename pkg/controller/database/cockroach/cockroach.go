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

	"github.com/crossplane/provider-rook/pkg/clients"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/pkg/errors"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	rookv1alpha1 "github.com/rook/rook/pkg/apis/cockroachdb.rook.io/v1alpha1"

	runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/crossplane/provider-rook/apis/database/v1alpha1"
	"github.com/crossplane/provider-rook/pkg/clients/database/cockroach"
)

// Error strings.
const (
	errNewCockroachClient     = "cannot create new Kubernetes client"
	errNotCockroachCluster    = "managed resource is not an Cockroach cluster"
	errGetCockroachCluster    = "cannot get Cockroach cluster in target Kubernetes cluster"
	errCreateCockroachCluster = "cannot create Cockroach cluster in target Kubernetes cluster"
	errUpdateCockroachCluster = "cannot update Cockroach cluster in target Kubernetes cluster"
	errDeleteCockroachCluster = "cannot delete Cockroach cluster in target Kubernetes cluster"
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
		Complete(managed.NewReconciler(mgr,
			resource.ManagedKind(v1alpha1.CockroachClusterGroupVersionKind),
			managed.WithExternalConnecter(&connecter{client: mgr.GetClient()})))
}

type connecter struct {
	client client.Client
}

func (c *connecter) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	scheme := runtime.NewScheme()
	scheme.AddKnownTypes(rookv1alpha1.SchemeGroupVersion,
		&rookv1alpha1.Cluster{},
		&rookv1alpha1.ClusterList{},
	)
	metav1.AddToGroupVersion(scheme, rookv1alpha1.SchemeGroupVersion)

	cl, err := clients.NewClient(ctx, c.client, mg, scheme)
	return &external{client: cl}, errors.Wrap(err, errNewCockroachClient)
}

type external struct {
	client client.Client
}

func (e *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	c, ok := mg.(*v1alpha1.CockroachCluster)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotCockroachCluster)
	}

	key := types.NamespacedName{
		Name:      c.Spec.CockroachClusterParameters.Name,
		Namespace: c.Spec.CockroachClusterParameters.Namespace,
	}

	external := &rookv1alpha1.Cluster{}

	err := e.client.Get(ctx, key, external)
	if kerrors.IsNotFound(err) {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errGetCockroachCluster)
	}

	// If we are able to get the resource Cluster instance, we will consider
	// it available. If a status is added to the Cluster CRD in the future we
	// should check it to set conditions.
	c.Status.SetConditions(runtimev1alpha1.Available())

	o := managed.ExternalObservation{
		ResourceExists:    true,
		ConnectionDetails: managed.ConnectionDetails{},
	}

	return o, nil

}

func (e *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	c, ok := mg.(*v1alpha1.CockroachCluster)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotCockroachCluster)
	}

	c.Status.SetConditions(runtimev1alpha1.Creating())

	err := e.client.Create(ctx, cockroach.CrossToRook(c))
	return managed.ExternalCreation{}, errors.Wrap(err, errCreateCockroachCluster)
}

func (e *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	c, ok := mg.(*v1alpha1.CockroachCluster)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotCockroachCluster)
	}

	key := types.NamespacedName{
		Name:      c.Spec.CockroachClusterParameters.Name,
		Namespace: c.Spec.CockroachClusterParameters.Namespace,
	}

	external := &rookv1alpha1.Cluster{}

	if err := e.client.Get(ctx, key, external); err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errGetCockroachCluster)
	}

	if !cockroach.NeedsUpdate(c, external) {
		return managed.ExternalUpdate{}, nil
	}

	update := cockroach.CrossToRook(c)
	update.ResourceVersion = external.ResourceVersion
	err := e.client.Update(ctx, update)
	return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateCockroachCluster)
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
