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

// UserConfigSpec defines the desired state of UserConfig
// There are four selectors: "labelSelector", "annotationSelector", "identityExtraFieldSelector" and "providerName".
// labelSelector and annoationSelector are matches against the User object
// identityExtraFieldSelector and providerName are matched against any of the Identities associated with User
// Selectors are considered in AND, so if multiple are defined tthey must all be true for a User to be selected.
type UserConfigSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

	// LabelSelector selects Users by label.
	// +kubebuilder:validation:Optional
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="urn:alm:descriptor:com.tectonic.ui:selector:"
	LabelSelector metav1.LabelSelector `json:"labelSelector,omitempty"`

	// AnnotationSelector selects Users by annotation.
	// +kubebuilder:validation:Optional
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="urn:alm:descriptor:com.tectonic.ui:selector:"
	AnnotationSelector metav1.LabelSelector `json:"annotationSelector,omitempty"`

	//IdentityExtraSelector allows you to specify a selector for the extra fields of the User's identities.
	//If one of the user identities matches the selector the User is selected
	//This condition is in OR with ProviderName
	// +kubebuilder:validation:Optional
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="urn:alm:descriptor:com.tectonic.ui:selector:"
	IdentityExtraFieldSelector metav1.LabelSelector `json:"identityExtraFieldSelector,omitempty"`

	//ProviderName allows you to specify an identity provider. If a user logged in with that provider it is selected.
	//This condition is in OR with IdentityExtraSelector
	// +kubebuilder:validation:Optional
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="urn:alm:descriptor:com.tectonic.ui:text"
	ProviderName string `json:"providerName,omitempty"`

	// Templates these are the templates of the resources to be created when a selected user is created/updated
	// +kubebuilder:validation:Optional
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	Templates []apis.LockedResourceTemplate `json:"templates,omitempty"`
}

// UserConfigStatus defines the observed state of UserConfig
type UserConfigStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	apis.EnforcingReconcileStatus `json:",inline"`
}

func (m *UserConfig) GetEnforcingReconcileStatus() apis.EnforcingReconcileStatus {
	return m.Status.EnforcingReconcileStatus
}

func (m *UserConfig) SetEnforcingReconcileStatus(reconcileStatus apis.EnforcingReconcileStatus) {
	m.Status.EnforcingReconcileStatus = reconcileStatus
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// UserConfig is the Schema for the userconfigs API
// +kubebuilder:resource:path=userconfigs,scope=Cluster
type UserConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   UserConfigSpec   `json:"spec,omitempty"`
	Status UserConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// UserConfigList contains a list of UserConfig
type UserConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []UserConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&UserConfig{}, &UserConfigList{})
}
