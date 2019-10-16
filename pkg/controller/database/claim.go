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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/crossplaneio/crossplane-runtime/pkg/resource"
	databasev1alpha1 "github.com/crossplaneio/crossplane/apis/database/v1alpha1"

	"github.com/crossplaneio/stack-rook/apis/database/v1alpha1"
)

// PostgreSQLInstanceYugabyteClaimController is responsible for adding the PostgreSQLInstance
// claim controller and its corresponding reconciler to the manager with any runtime configuration.
type PostgreSQLInstanceYugabyteClaimController struct{}

// SetupWithManager adds a controller that reconciles PostgreSQLInstance instance claims.
func (c *PostgreSQLInstanceYugabyteClaimController) SetupWithManager(mgr ctrl.Manager) error {
	name := strings.ToLower(fmt.Sprintf("%s.%s.%s",
		databasev1alpha1.PostgreSQLInstanceKind,
		v1alpha1.YugabyteClusterKind,
		v1alpha1.Group))

	p := resource.NewPredicates(resource.AnyOf(
		resource.HasManagedResourceReferenceKind(resource.ManagedKind(v1alpha1.YugabyteClusterGroupVersionKind)),
		resource.IsManagedKind(resource.ManagedKind(v1alpha1.YugabyteClusterGroupVersionKind), mgr.GetScheme()),
		resource.HasIndirectClassReferenceKind(mgr.GetClient(), mgr.GetScheme(), resource.ClassKinds{
			Portable:    databasev1alpha1.PostgreSQLInstanceClassGroupVersionKind,
			NonPortable: v1alpha1.YugabyteClusterClassGroupVersionKind,
		})))

	r := resource.NewClaimReconciler(mgr,
		resource.ClaimKind(databasev1alpha1.PostgreSQLInstanceGroupVersionKind),
		resource.ClassKinds{
			Portable:    databasev1alpha1.PostgreSQLInstanceClassGroupVersionKind,
			NonPortable: v1alpha1.YugabyteClusterClassGroupVersionKind,
		},
		resource.ManagedKind(v1alpha1.YugabyteClusterGroupVersionKind),
		resource.WithManagedBinder(resource.NewAPIManagedStatusBinder(mgr.GetClient())),
		resource.WithManagedFinalizer(resource.NewAPIManagedStatusUnbinder(mgr.GetClient())),
		resource.WithManagedConfigurators(
			resource.ManagedConfiguratorFn(ConfigureYugabyteCluster),
			resource.NewObjectMetaConfigurator(mgr.GetScheme()),
		))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		Watches(&source.Kind{Type: &v1alpha1.YugabyteCluster{}}, &resource.EnqueueRequestForClaim{}).
		For(&databasev1alpha1.PostgreSQLInstance{}).
		WithEventFilter(p).
		Complete(r)
}

// ConfigureYugabyteCluster configures the supplied instance (presumed
// to be a YugabyteCluster) using the supplied instance claim (presumed to be a
// PostgreSQLInstance) and instance class.
func ConfigureYugabyteCluster(_ context.Context, cm resource.Claim, cs resource.NonPortableClass, mg resource.Managed) error {
	_, cmok := cm.(*databasev1alpha1.PostgreSQLInstance)
	if !cmok {
		return errors.Errorf("expected resource claim %s to be %s", cm.GetName(), databasev1alpha1.PostgreSQLInstanceGroupVersionKind)
	}

	c, csok := cs.(*v1alpha1.YugabyteClusterClass)
	if !csok {
		return errors.Errorf("expected resource class %s to be %s", cs.GetName(), v1alpha1.YugabyteClusterClassGroupVersionKind)
	}

	m, mgok := mg.(*v1alpha1.YugabyteCluster)
	if !mgok {
		return errors.Errorf("expected managed instance %s to be %s", mg.GetName(), v1alpha1.YugabyteClusterGroupVersionKind)
	}

	m.Spec.WriteConnectionSecretToReference = corev1.LocalObjectReference{Name: string(cm.GetUID())}
	m.Spec.YugabyteClusterParameters = c.SpecTemplate.YugabyteClusterParameters
	m.Spec.ProviderReference = c.SpecTemplate.ProviderReference
	m.Spec.ReclaimPolicy = c.SpecTemplate.ReclaimPolicy

	return nil
}
