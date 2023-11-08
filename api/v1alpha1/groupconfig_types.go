/*
Copyright 2020 Red Hat Community of Practice.

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
	apis "github.com/redhat-cop/operator-utils/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// GroupConfigSpec defines the desired state of GroupConfig
// There are two selectors: "labelSelector", "annotationSelector".
// Selectors are considered in AND, so if multiple are defined they must all be true for a Group to be selected.
type GroupConfigSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// LabelSelector selects Groups by label.
	// +kubebuilder:validation:Optional
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="urn:alm:descriptor:com.tectonic.ui:selector:"
	LabelSelector metav1.LabelSelector `json:"labelSelector,omitempty"`

	// AnnotationSelector selects Groups by annotation.
	// +kubebuilder:validation:Optional
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="urn:alm:descriptor:com.tectonic.ui:selector:"
	AnnotationSelector metav1.LabelSelector `json:"annotationSelector,omitempty"`

	// Templates these are the templates of the resources to be created when a selected groups is created/updated
	// +kubebuilder:validation:Optional
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	Templates []apis.LockedResourceTemplate `json:"templates,omitempty"`
}

// GroupConfigStatus defines the observed state of GroupConfig
type GroupConfigStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	apis.EnforcingReconcileStatus `json:",inline"`
}

func (m *GroupConfig) GetEnforcingReconcileStatus() apis.EnforcingReconcileStatus {
	return m.Status.EnforcingReconcileStatus
}

func (m *GroupConfig) SetEnforcingReconcileStatus(reconcileStatus apis.EnforcingReconcileStatus) {
	m.Status.EnforcingReconcileStatus = reconcileStatus
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// GroupConfig is the Schema for the groupconfigs API
// +kubebuilder:resource:path=groupconfigs,scope=Cluster
type GroupConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GroupConfigSpec   `json:"spec,omitempty"`
	Status GroupConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// GroupConfigList contains a list of GroupConfig
type GroupConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GroupConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GroupConfig{}, &GroupConfigList{})
}
