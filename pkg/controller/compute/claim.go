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

package compute

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
	computev1alpha1 "github.com/crossplaneio/crossplane/apis/compute/v1alpha1"

	"github.com/crossplaneio/stack-gcp/apis/compute/v1alpha2"
)

// GKEClusterClaimController is responsible for adding the GKECluster
// claim controller and its corresponding reconciler to the manager with any runtime configuration.
type GKEClusterClaimController struct{}

// SetupWithManager adds a controller that reconciles KubernetesCluster resource claims.
func (c *GKEClusterClaimController) SetupWithManager(mgr ctrl.Manager) error {
	name := strings.ToLower(fmt.Sprintf("%s.%s.%s",
		computev1alpha1.KubernetesClusterKind,
		v1alpha2.GKEClusterClassKind,
		v1alpha2.Group))

	p := resource.NewPredicates(resource.AnyOf(
		resource.HasManagedResourceReferenceKind(resource.ManagedKind(v1alpha2.GKEClusterGroupVersionKind)),
		resource.IsManagedKind(resource.ManagedKind(v1alpha2.GKEClusterGroupVersionKind), mgr.GetScheme()),
		resource.HasIndirectClassReferenceKind(mgr.GetClient(), mgr.GetScheme(), resource.ClassKinds{
			Portable:    computev1alpha1.KubernetesClusterClassGroupVersionKind,
			NonPortable: v1alpha2.GKEClusterClassGroupVersionKind,
		})))

	r := resource.NewClaimReconciler(mgr,
		resource.ClaimKind(computev1alpha1.KubernetesClusterGroupVersionKind),
		resource.ClassKinds{
			Portable:    computev1alpha1.KubernetesClusterClassGroupVersionKind,
			NonPortable: v1alpha2.GKEClusterClassGroupVersionKind,
		},
		resource.ManagedKind(v1alpha2.GKEClusterGroupVersionKind),
		resource.WithManagedConfigurators(
			resource.ManagedConfiguratorFn(ConfigureGKECluster),
			resource.NewObjectMetaConfigurator(mgr.GetScheme()),
		))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		Watches(&source.Kind{Type: &v1alpha2.GKECluster{}}, &resource.EnqueueRequestForClaim{}).
		For(&computev1alpha1.KubernetesCluster{}).
		WithEventFilter(p).
		Complete(r)
}

// ConfigureGKECluster configures the supplied resource (presumed to be a
// GKECluster) using the supplied resource claim (presumed to be a
// KubernetesCluster) and resource class.
func ConfigureGKECluster(_ context.Context, cm resource.Claim, cs resource.NonPortableClass, mg resource.Managed) error {
	if _, cmok := cm.(*computev1alpha1.KubernetesCluster); !cmok {
		return errors.Errorf("expected resource claim %s to be %s", cm.GetName(), computev1alpha1.KubernetesClusterGroupVersionKind)
	}

	rs, csok := cs.(*v1alpha2.GKEClusterClass)
	if !csok {
		return errors.Errorf("expected resource class %s to be %s", cs.GetName(), v1alpha2.GKEClusterClassGroupVersionKind)
	}

	i, mgok := mg.(*v1alpha2.GKECluster)
	if !mgok {
		return errors.Errorf("expected managed resource %s to be %s", mg.GetName(), v1alpha2.GKEClusterGroupVersionKind)
	}

	spec := &v1alpha2.GKEClusterSpec{
		ResourceSpec: runtimev1alpha1.ResourceSpec{
			ReclaimPolicy: v1alpha2.DefaultReclaimPolicy,
		},
		GKEClusterParameters: rs.SpecTemplate.GKEClusterParameters,
	}

	// NOTE(hasheddan): consider moving defaulting to either CRD or managed reconciler level
	if spec.Labels == nil {
		spec.Labels = map[string]string{}
	}
	if spec.NumNodes == 0 {
		spec.NumNodes = v1alpha2.DefaultNumberOfNodes
	}
	if spec.Scopes == nil {
		spec.Scopes = []string{}
	}

	spec.WriteConnectionSecretToReference = corev1.LocalObjectReference{Name: string(cm.GetUID())}
	spec.ProviderReference = rs.SpecTemplate.ProviderReference
	spec.ReclaimPolicy = rs.SpecTemplate.ReclaimPolicy

	i.Spec = *spec

	return nil
}
