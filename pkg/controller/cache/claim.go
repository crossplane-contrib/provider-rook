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

package cache

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
	cachev1alpha1 "github.com/crossplaneio/crossplane/apis/cache/v1alpha1"

	"github.com/crossplaneio/stack-gcp/apis/cache/v1alpha2"
)

// CloudMemorystoreInstanceClaimController is responsible for adding the Cloud Memorystore
// claim controller and its corresponding reconciler to the manager with any runtime configuration.
type CloudMemorystoreInstanceClaimController struct{}

// SetupWithManager adds a controller that reconciles RedisCluster resource claims.
func (c *CloudMemorystoreInstanceClaimController) SetupWithManager(mgr ctrl.Manager) error {
	name := strings.ToLower(fmt.Sprintf("%s.%s.%s",
		cachev1alpha1.RedisClusterKind,
		v1alpha2.CloudMemorystoreInstanceKind,
		v1alpha2.Group))

	p := resource.NewPredicates(resource.AnyOf(
		resource.HasManagedResourceReferenceKind(resource.ManagedKind(v1alpha2.CloudMemorystoreInstanceGroupVersionKind)),
		resource.IsManagedKind(resource.ManagedKind(v1alpha2.CloudMemorystoreInstanceGroupVersionKind), mgr.GetScheme()),
		resource.HasIndirectClassReferenceKind(mgr.GetClient(), mgr.GetScheme(), resource.ClassKinds{
			Portable:    cachev1alpha1.RedisClusterClassGroupVersionKind,
			NonPortable: v1alpha2.CloudMemorystoreInstanceClassGroupVersionKind,
		})))

	r := resource.NewClaimReconciler(mgr,
		resource.ClaimKind(cachev1alpha1.RedisClusterGroupVersionKind),
		resource.ClassKinds{
			Portable:    cachev1alpha1.RedisClusterClassGroupVersionKind,
			NonPortable: v1alpha2.CloudMemorystoreInstanceClassGroupVersionKind,
		},
		resource.ManagedKind(v1alpha2.CloudMemorystoreInstanceGroupVersionKind),
		resource.WithManagedBinder(resource.NewAPIManagedStatusBinder(mgr.GetClient())),
		resource.WithManagedFinalizer(resource.NewAPIManagedStatusUnbinder(mgr.GetClient())),
		resource.WithManagedConfigurators(
			resource.ManagedConfiguratorFn(ConfigureCloudMemorystoreInstance),
			resource.NewObjectMetaConfigurator(mgr.GetScheme()),
		))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		Watches(&source.Kind{Type: &v1alpha2.CloudMemorystoreInstance{}}, &resource.EnqueueRequestForClaim{}).
		For(&cachev1alpha1.RedisCluster{}).
		WithEventFilter(p).
		Complete(r)
}

// ConfigureCloudMemorystoreInstance configures the supplied resource (presumed
// to be a CloudMemorystoreInstance) using the supplied resource claim (presumed
// to be a RedisCluster) and resource class.
func ConfigureCloudMemorystoreInstance(_ context.Context, cm resource.Claim, cs resource.NonPortableClass, mg resource.Managed) error {
	rc, cmok := cm.(*cachev1alpha1.RedisCluster)
	if !cmok {
		return errors.Errorf("expected resource claim %s to be %s", cm.GetName(), cachev1alpha1.RedisClusterGroupVersionKind)
	}

	rl, csok := cs.(*v1alpha2.CloudMemorystoreInstanceClass)
	if !csok {
		return errors.Errorf("expected resource class %s to be %s", cs.GetName(), v1alpha2.CloudMemorystoreInstanceClassGroupVersionKind)
	}

	i, mgok := mg.(*v1alpha2.CloudMemorystoreInstance)
	if !mgok {
		return errors.Errorf("expected managed resource %s to be %s", mg.GetName(), v1alpha2.CloudMemorystoreInstanceGroupVersionKind)
	}

	spec := &v1alpha2.CloudMemorystoreInstanceSpec{
		ResourceSpec: runtimev1alpha1.ResourceSpec{
			ReclaimPolicy: runtimev1alpha1.ReclaimRetain,
		},
		CloudMemorystoreInstanceParameters: rl.SpecTemplate.CloudMemorystoreInstanceParameters,
	}

	if rc.Spec.EngineVersion != "" {
		spec.RedisVersion = toGCPFormat(rc.Spec.EngineVersion)
	}

	spec.WriteConnectionSecretToReference = corev1.LocalObjectReference{Name: string(cm.GetUID())}
	spec.ProviderReference = rl.SpecTemplate.ProviderReference
	spec.ReclaimPolicy = rl.SpecTemplate.ReclaimPolicy

	i.Spec = *spec

	return nil
}

// toGCPFormat transforms a RedisClusterSpec EngineVersion to a
// CloudMemoryStoreInstanceSpec RedisVersion. The former uses major.minor
// (e.g. 3.2). The latter uses REDIS_MAJOR_MINOR (e.g. REDIS_3_2).
func toGCPFormat(version string) string {
	if version == "" {
		return ""
	}
	return fmt.Sprintf("REDIS_%s", strings.Replace(version, ".", "_", -1))
}
