/*
Copyright 2020 The Crossplane Authors.

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

package v1alpha1

import (
	v1 "k8s.io/api/core/v1"
)

// StorageScopeSpec defines scope or boundaries of storage that the cluster will
// use for its underlying storage.
type StorageScopeSpec struct {
	NodeCount int `json:"nodeCount,omitempty"`
	// PersistentVolumeClaims to use as storage
	VolumeClaimTemplates []v1.PersistentVolumeClaim `json:"volumeClaimTemplates,omitempty"`
}

// Annotations are a Crossplane representation of Rook Annotations.
type Annotations map[string]string
