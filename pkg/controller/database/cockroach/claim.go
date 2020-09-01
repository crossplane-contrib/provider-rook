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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/source"

	runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/claimbinding"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/claimdefaulting"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/claimscheduling"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	databasev1alpha1 "github.com/crossplane/crossplane/apis/database/v1alpha1"

	"github.com/crossplane/provider-rook/apis/database/v1alpha1"
)

// A ClaimSchedulingController reconciles PostgreSQLInstance claims that include
// a class selector but omit their class and resource references by picking a
// random matching Rook Cockroach Cluster class, if any.
type ClaimSchedulingController struct{}

// SetupWithManager sets up the ClaimSchedulingController using the
// supplied manager.
func (c *ClaimSchedulingController) SetupWithManager(mgr ctrl.Manager) error {
	name := strings.ToLower(fmt.Sprintf("scheduler.%s.%s.%s",
		databasev1alpha1.PostgreSQLInstanceKind,
		v1alpha1.CockroachClusterKind,
		v1alpha1.Group))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&databasev1alpha1.PostgreSQLInstance{}).
		WithEventFilter(resource.NewPredicates(resource.AllOf(
			resource.HasClassSelector(),
			resource.HasNoClassReference(),
			resource.HasNoManagedResourceReference(),
		))).
		Complete(claimscheduling.NewReconciler(mgr,
			resource.ClaimKind(databasev1alpha1.PostgreSQLInstanceGroupVersionKind),
			resource.ClassKind(v1alpha1.CockroachClusterClassGroupVersionKind),
		))
}

// A ClaimDefaultingController reconciles Cluster claims that omit
// their resource ref, class ref, and class selector by choosing a default Rook Cockroach Cluster class if one exists.
type ClaimDefaultingController struct{}

// SetupWithManager sets up the ClaimDefaultingController using the
// supplied manager.
func (c *ClaimDefaultingController) SetupWithManager(mgr ctrl.Manager) error {
	name := strings.ToLower(fmt.Sprintf("defaulter.%s.%s.%s",
		databasev1alpha1.PostgreSQLInstanceKind,
		v1alpha1.CockroachClusterKind,
		v1alpha1.Group))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&databasev1alpha1.PostgreSQLInstance{}).
		WithEventFilter(resource.NewPredicates(resource.AllOf(
			resource.HasNoClassSelector(),
			resource.HasNoClassReference(),
			resource.HasNoManagedResourceReference(),
		))).
		Complete(claimdefaulting.NewReconciler(mgr,
			resource.ClaimKind(databasev1alpha1.PostgreSQLInstanceGroupVersionKind),
			resource.ClassKind(v1alpha1.CockroachClusterClassGroupVersionKind),
		))
}

// ClaimController is responsible for adding the PostgreSQLInstance
// claim controller and its corresponding reconciler to the manager with any runtime configuration.
type ClaimController struct{}

// SetupWithManager adds a controller that reconciles PostgreSQLInstance instance claims.
func (c *ClaimController) SetupWithManager(mgr ctrl.Manager) error {
	name := strings.ToLower(fmt.Sprintf("%s.%s.%s",
		databasev1alpha1.PostgreSQLInstanceKind,
		v1alpha1.CockroachClusterKind,
		v1alpha1.Group))

	p := resource.NewPredicates(resource.AnyOf(
		resource.HasClassReferenceKind(resource.ClassKind(v1alpha1.CockroachClusterClassGroupVersionKind)),
		resource.HasManagedResourceReferenceKind(resource.ManagedKind(v1alpha1.CockroachClusterGroupVersionKind)),
		resource.IsManagedKind(resource.ManagedKind(v1alpha1.CockroachClusterGroupVersionKind), mgr.GetScheme()),
	))

	r := claimbinding.NewReconciler(mgr,
		resource.ClaimKind(databasev1alpha1.PostgreSQLInstanceGroupVersionKind),
		resource.ClassKind(v1alpha1.CockroachClusterClassGroupVersionKind),
		resource.ManagedKind(v1alpha1.CockroachClusterGroupVersionKind),
		claimbinding.WithManagedConfigurators(
			claimbinding.ManagedConfiguratorFn(ConfigureCockroachCluster),
			claimbinding.ManagedConfiguratorFn(claimbinding.ConfigureReclaimPolicy),
			claimbinding.ManagedConfiguratorFn(claimbinding.ConfigureNames),
		))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		Watches(&source.Kind{Type: &v1alpha1.CockroachCluster{}}, &resource.EnqueueRequestForClaim{}).
		For(&databasev1alpha1.PostgreSQLInstance{}).
		WithEventFilter(p).
		Complete(r)
}

// ConfigureCockroachCluster configures the supplied instance (presumed
// to be a CockroachCluster) using the supplied instance claim (presumed to be a
// PostgreSQLInstance) and instance class.
func ConfigureCockroachCluster(_ context.Context, cm resource.Claim, cs resource.Class, mg resource.Managed) error {
	_, cmok := cm.(*databasev1alpha1.PostgreSQLInstance)
	if !cmok {
		return errors.Errorf("expected resource claim %s to be %s", cm.GetName(), databasev1alpha1.PostgreSQLInstanceGroupVersionKind)
	}

	c, csok := cs.(*v1alpha1.CockroachClusterClass)
	if !csok {
		return errors.Errorf("expected resource class %s to be %s", cs.GetName(), v1alpha1.CockroachClusterClassGroupVersionKind)
	}

	m, mgok := mg.(*v1alpha1.CockroachCluster)
	if !mgok {
		return errors.Errorf("expected managed instance %s to be %s", mg.GetName(), v1alpha1.CockroachClusterGroupVersionKind)
	}

	m.Spec.WriteConnectionSecretToReference = &runtimev1alpha1.SecretReference{
		Namespace: c.SpecTemplate.WriteConnectionSecretsToNamespace,
		Name:      string(cm.GetUID()),
	}
	m.Spec.CockroachClusterParameters = c.SpecTemplate.CockroachClusterParameters
	m.Spec.ProviderReference = c.SpecTemplate.ProviderReference.DeepCopy()
	m.Spec.ReclaimPolicy = c.SpecTemplate.ReclaimPolicy

	return nil
}
