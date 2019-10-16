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

package storage

import (
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/source"

	runtimev1alpha1 "github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane-runtime/pkg/resource"
	storagev1alpha1 "github.com/crossplaneio/crossplane/apis/storage/v1alpha1"

	"github.com/crossplaneio/stack-gcp/apis/storage/v1alpha2"
)

// BucketClaimController is responsible for adding the Bucket claim controller and its
// corresponding reconciler to the manager with any runtime configuration.
type BucketClaimController struct{}

// SetupWithManager adds a controller that reconciles Bucket resource claims.
func (c *BucketClaimController) SetupWithManager(mgr ctrl.Manager) error {
	name := strings.ToLower(fmt.Sprintf("%s.%s.%s",
		storagev1alpha1.BucketKind,
		v1alpha2.BucketKind,
		v1alpha2.Group))

	p := resource.NewPredicates(resource.AnyOf(
		resource.HasManagedResourceReferenceKind(resource.ManagedKind(v1alpha2.BucketGroupVersionKind)),
		resource.IsManagedKind(resource.ManagedKind(v1alpha2.BucketGroupVersionKind), mgr.GetScheme()),
		resource.HasIndirectClassReferenceKind(mgr.GetClient(), mgr.GetScheme(), resource.ClassKinds{
			Portable:    storagev1alpha1.BucketClassGroupVersionKind,
			NonPortable: v1alpha2.BucketClassGroupVersionKind,
		})))

	r := resource.NewClaimReconciler(mgr,
		resource.ClaimKind(storagev1alpha1.BucketGroupVersionKind),
		resource.ClassKinds{
			Portable:    storagev1alpha1.BucketClassGroupVersionKind,
			NonPortable: v1alpha2.BucketClassGroupVersionKind,
		},
		resource.ManagedKind(v1alpha2.BucketGroupVersionKind),
		resource.WithManagedBinder(resource.NewAPIManagedStatusBinder(mgr.GetClient())),
		resource.WithManagedFinalizer(resource.NewAPIManagedStatusUnbinder(mgr.GetClient())),
		resource.WithManagedConfigurators(
			resource.ManagedConfiguratorFn(ConfigureBucket),
			resource.NewObjectMetaConfigurator(mgr.GetScheme()),
		))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		Watches(&source.Kind{Type: &v1alpha2.Bucket{}}, &resource.EnqueueRequestForClaim{}).
		For(&storagev1alpha1.Bucket{}).
		WithEventFilter(p).
		Complete(r)
}

// ConfigureBucket configures the supplied resource (presumed
// to be a Bucket) using the supplied resource claim (presumed
// to be a Bucket) and resource class.
func ConfigureBucket(_ context.Context, cm resource.Claim, cs resource.NonPortableClass, mg resource.Managed) error {
	bcm, cmok := cm.(*storagev1alpha1.Bucket)
	if !cmok {
		return errors.Errorf("expected resource claim %s to be %s", cm.GetName(), storagev1alpha1.BucketGroupVersionKind)
	}

	rs, csok := cs.(*v1alpha2.BucketClass)
	if !csok {
		return errors.Errorf("expected resource class %s to be %s", cs.GetName(), v1alpha2.BucketClassGroupVersionKind)
	}

	bmg, mgok := mg.(*v1alpha2.Bucket)
	if !mgok {
		return errors.Errorf("expected managed resource %s to be %s", mg.GetName(), v1alpha2.BucketGroupVersionKind)
	}

	spec := &v1alpha2.BucketSpec{
		ResourceSpec: runtimev1alpha1.ResourceSpec{
			ReclaimPolicy: runtimev1alpha1.ReclaimRetain,
		},
		BucketParameters: rs.SpecTemplate.BucketParameters,
	}

	// Set Name bucket name if Name value is provided by Bucket Claim spec
	if bcm.Spec.Name != "" {
		spec.NameFormat = bcm.Spec.Name
	}

	// Set PredefinedACL from bucketClaim claim only iff: claim has this value and
	// it is not defined in the resource class (i.e. not already in the spec)
	if bcm.Spec.PredefinedACL != nil && spec.PredefinedACL == "" {
		spec.PredefinedACL = string(*bcm.Spec.PredefinedACL)
	}

	spec.WriteConnectionSecretToReference = corev1.LocalObjectReference{Name: string(cm.GetUID())}
	spec.ProviderReference = rs.SpecTemplate.ProviderReference
	spec.ReclaimPolicy = rs.SpecTemplate.ReclaimPolicy

	bmg.Spec = *spec

	return nil
}
