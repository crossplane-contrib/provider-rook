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
	"fmt"

	"github.com/pkg/errors"
	rookv1alpha1 "github.com/rook/rook/pkg/apis/yugabytedb.rook.io/v1alpha1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/crossplane/provider-rook/apis/database/v1alpha1"
	"github.com/crossplane/provider-rook/pkg/clients"
	"github.com/crossplane/provider-rook/pkg/clients/database/yugabyte"
)

// Error strings.
const (
	errNewYugabyteClient     = "cannot create new Kubernetes client"
	errNotYugabyteCluster    = "managed resource is not an Yugabyte cluster"
	errGetYugabyteCluster    = "cannot get Yugabyte cluster in target Kubernetes cluster"
	errCreateYugabyteCluster = "cannot create Yugabyte cluster in target Kubernetes cluster"
	errUpdateYugabyteCluster = "cannot update Yugabyte cluster in target Kubernetes cluster"
	errDeleteYugabyteCluster = "cannot delete Yugabyte cluster in target Kubernetes cluster"
)

// Setup creates a new YugabyteCluster Controller and adds it to the Manager
// with default RBAC. The Manager will set fields on the Controller and start it
// when the Manager is Started.
func Setup(mgr ctrl.Manager, l logging.Logger) error {
	name := managed.ControllerName(fmt.Sprintf("%s.%s", v1alpha1.YugabyteClusterKind, v1alpha1.Group))
	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1alpha1.YugabyteCluster{}).
		Complete(managed.NewReconciler(mgr,
			resource.ManagedKind(v1alpha1.YugabyteClusterGroupVersionKind),
			managed.WithExternalConnecter(&connecter{client: mgr.GetClient()}),
			managed.WithLogger(l.WithValues("controller", name)),
			managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name)))))
}

type connecter struct {
	client client.Client
}

func (c *connecter) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	scheme := runtime.NewScheme()
	scheme.AddKnownTypes(rookv1alpha1.SchemeGroupVersion,
		&rookv1alpha1.YBCluster{},
		&rookv1alpha1.YBClusterList{},
	)

	metav1.AddToGroupVersion(scheme, rookv1alpha1.SchemeGroupVersion)

	cl, err := clients.NewClient(ctx, c.client, mg, scheme)
	return &external{client: cl}, errors.Wrap(err, errNewYugabyteClient)
}

type external struct {
	client client.Client
}

func (e *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	c, ok := mg.(*v1alpha1.YugabyteCluster)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotYugabyteCluster)
	}

	key := types.NamespacedName{
		Name:      c.Spec.YugabyteClusterParameters.Name,
		Namespace: c.Spec.YugabyteClusterParameters.Namespace,
	}

	external := &rookv1alpha1.YBCluster{}

	err := e.client.Get(ctx, key, external)
	if kerrors.IsNotFound(err) {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errGetYugabyteCluster)
	}

	// If we are able to get the resource YBCluster instance, we will consider
	// it available. If a status is added to the YBCluster CRD in the future we
	// should check it to set conditions.
	c.Status.SetConditions(xpv1.Available())

	o := managed.ExternalObservation{
		ResourceExists:    true,
		ConnectionDetails: managed.ConnectionDetails{},
	}

	return o, nil

}

func (e *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	c, ok := mg.(*v1alpha1.YugabyteCluster)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotYugabyteCluster)
	}

	c.Status.SetConditions(xpv1.Creating())

	err := e.client.Create(ctx, yugabyte.CrossToRook(c))
	return managed.ExternalCreation{}, errors.Wrap(err, errCreateYugabyteCluster)
}

func (e *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	c, ok := mg.(*v1alpha1.YugabyteCluster)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotYugabyteCluster)
	}

	key := types.NamespacedName{
		Name:      c.Spec.YugabyteClusterParameters.Name,
		Namespace: c.Spec.YugabyteClusterParameters.Namespace,
	}

	external := &rookv1alpha1.YBCluster{}

	if err := e.client.Get(ctx, key, external); err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errGetYugabyteCluster)
	}

	if !yugabyte.NeedsUpdate(c, external) {
		return managed.ExternalUpdate{}, nil
	}

	update := yugabyte.CrossToRook(c)
	update.ResourceVersion = external.ResourceVersion
	err := e.client.Update(ctx, update)
	return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateYugabyteCluster)
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) error {
	c, ok := mg.(*v1alpha1.YugabyteCluster)
	if !ok {
		return errors.New(errNotYugabyteCluster)
	}

	c.SetConditions(xpv1.Deleting())

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
