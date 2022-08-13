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
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// ObjectAction defines actions applicable to Object
type ObjectAction string

// A ManagementPolicy determines what should happen to the underlying external
// resource when a managed resource is created, updated, deleted, or observed.
// +kubebuilder:validation:Enum=Default;ObserveCreateUpdate;ObserveDelete;Observe
type ManagementPolicy string

const (
	// Default means the provider can fully manage the resource.
	Default ManagementPolicy = "Default"
	// ObserveCreateUpdate means the provider can observe, create, or update
	// the resource, but can not delete it.
	ObserveCreateUpdate ManagementPolicy = "ObserveCreateUpdate"
	// ObserveDelete means the provider can observe or delete the resource, but
	// can not create and update it.
	ObserveDelete ManagementPolicy = "ObserveDelete"
	// Observe means the provider can only observe the resource.
	Observe ManagementPolicy = "Observe"

	// ObjectActionCreate means to create an Object
	ObjectActionCreate ObjectAction = "Create"
	// ObjectActionUpdate means to update an Object
	ObjectActionUpdate ObjectAction = "Update"
	// ObjectActionDelete means to delete an Object
	ObjectActionDelete ObjectAction = "Delete"
)

// DependsOn refers to an object by Name, Kind, APIVersion, etc. It is used to
// reference other Object or arbitrary Kubernetes resource which is either
// cluster or namespace scoped.
type DependsOn struct {
	// APIVersion of the referenced object.
	// +kubebuilder:default=kubernetes.crossplane.io/v1alpha1
	// +optional
	APIVersion string `json:"apiVersion,omitempty"`
	// Kind of the referenced object.
	// +kubebuilder:default=Object
	// +optional
	Kind string `json:"kind,omitempty"`
	// Name of the referenced object.
	Name string `json:"name"`
	// Namespace of the referenced object.
	// +optional
	Namespace string `json:"namespace,omitempty"`
	// BlockOwnerDeletion flag to support cascading forward deletion
	// +optional
	BlockOwnerDeletion *bool `json:"blockOwnerDeletion,omitempty"`
}

// PatchesFrom refers to an object by Name, Kind, APIVersion, etc., and patch
// fields from this object.
type PatchesFrom struct {
	DependsOn `json:",inline"`
	// FieldPath is the path of the field on the resource whose value is to be
	// used as input.
	FieldPath *string `json:"fieldPath"`
}

// Reference refers to an Object or arbitrary Kubernetes resource and optionally
// patch values from that resource to the current Object.
type Reference struct {
	// DependsOn is used to declare dependency on other Object or arbitrary
	// Kubernetes resource.
	// +optional
	*DependsOn `json:"dependsOn,omitempty"`
	// PatchesFrom is used to declare dependency on other Object or arbitrary
	// Kubernetes resource, and also patch fields from this object.
	// +optional
	*PatchesFrom `json:"patchesFrom,omitempty"`
	// ToFieldPath is the path of the field on the resource whose value will
	// be changed with the result of transforms. Leave empty if you'd like to
	// propagate to the same path as patchesFrom.fieldPath.
	// +optional
	ToFieldPath *string `json:"toFieldPath,omitempty"`
}

// ObjectParameters are the configurable fields of a Object.
type ObjectParameters struct {
	// Raw JSON representation of the kubernetes object to be created.
	// +kubebuilder:validation:EmbeddedResource
	// +kubebuilder:pruning:PreserveUnknownFields
	Manifest runtime.RawExtension `json:"manifest"`
}

// ObjectObservation are the observable fields of a Object.
type ObjectObservation struct {
	// Raw JSON representation of the remote object.
	// +kubebuilder:validation:EmbeddedResource
	// +kubebuilder:pruning:PreserveUnknownFields
	Manifest runtime.RawExtension `json:"manifest,omitempty"`
}

// A ObjectSpec defines the desired state of a Object.
type ObjectSpec struct {
	xpv1.ResourceSpec `json:",inline"`
	// +kubebuilder:default=Default
	ManagementPolicy `json:"managementPolicy,omitempty"`
	References       []Reference      `json:"references,omitempty"`
	ForProvider      ObjectParameters `json:"forProvider"`
}

// A ObjectStatus represents the observed state of a Object.
type ObjectStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          ObjectObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// A Object is an provider Kubernetes API type
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Cluster,categories={crossplane,managed,kubernetes}
type Object struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ObjectSpec   `json:"spec"`
	Status ObjectStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ObjectList contains a list of Object
type ObjectList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Object `json:"items"`
}

// ApplyFromFieldPathPatch patches the "to" resource, using a source field
// on the "from" resource.
func (r *Reference) ApplyFromFieldPathPatch(from, to runtime.Object) error {
	// Default to patch the same field on the "to" resource.
	if r.ToFieldPath == nil {
		r.ToFieldPath = r.PatchesFrom.FieldPath
	}

	paved, err := fieldpath.PaveObject(from)
	if err != nil {
		return err
	}

	out, err := paved.GetValue(*r.PatchesFrom.FieldPath)
	if err != nil {
		return err
	}

	return patchFieldValueToObject(*r.ToFieldPath, out, to)
}

// patchFieldValueToObject, given a path, value and "to" object, will
// apply the value to the "to" object at the given path, returning
// any errors as they occur.
func patchFieldValueToObject(path string, value interface{}, to runtime.Object) error {
	paved, err := fieldpath.PaveObject(to)
	if err != nil {
		return err
	}

	err = paved.SetValue("spec.forProvider.manifest."+path, value)
	if err != nil {
		return err
	}

	return runtime.DefaultUnstructuredConverter.FromUnstructured(paved.UnstructuredContent(), to)
}

// IsActionAllowed determines if action is allowed to be performed on Object
func (p *ManagementPolicy) IsActionAllowed(action ObjectAction) bool {
	if action == ObjectActionCreate || action == ObjectActionUpdate {
		return *p == Default || *p == ObserveCreateUpdate
	}

	// ObjectActionDelete
	return *p == Default || *p == ObserveDelete
}
