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
	"github.com/redhat-cop/operator-utils/pkg/util/apis"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// NamespaceConfigSpec defines the desired state of NamespaceConfig
// There are two selectors: "labelSelector", "annotationSelector".
// Selectors are considered in AND, so if multiple are defined they must all be true for a Namespace to be selected.
type NamespaceConfigSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

	// LabelSelector selects Namespaces by label.
	// +kubebuilder:validation:Optional
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="urn:alm:descriptor:com.tectonic.ui:selector:"
	LabelSelector metav1.LabelSelector `json:"labelSelector,omitempty"`

	// AnnotationSelector selects Namespaces by annotation.
	// +kubebuilder:validation:Optional
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="urn:alm:descriptor:com.tectonic.ui:selector:"
	AnnotationSelector metav1.LabelSelector `json:"annotationSelector,omitempty"`

	// Templates these are the templates of the resources to be created when a selected namespace is created/updated
	// +kubebuilder:validation:Optional
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	Templates []apis.LockedResourceTemplate `json:"templates,omitempty"`
}

// NamespaceConfigStatus defines the observed state of NamespaceSConfig
type NamespaceConfigStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	apis.EnforcingReconcileStatus `json:",inline"`
}

func (m *NamespaceConfig) GetEnforcingReconcileStatus() apis.EnforcingReconcileStatus {
	return m.Status.EnforcingReconcileStatus
}

func (m *NamespaceConfig) SetEnforcingReconcileStatus(reconcileStatus apis.EnforcingReconcileStatus) {
	m.Status.EnforcingReconcileStatus = reconcileStatus
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// NamespaceConfig is the Schema for the namespaceconfigs API
// +kubebuilder:resource:path=namespaceconfigs,scope=Cluster
type NamespaceConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NamespaceConfigSpec   `json:"spec,omitempty"`
	Status NamespaceConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// NamespaceConfigList contains a list of NamespaceConfig
type NamespaceConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NamespaceConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NamespaceConfig{}, &NamespaceConfigList{})
}
