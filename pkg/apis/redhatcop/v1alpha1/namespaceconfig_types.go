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

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NamespaceConfig is the Schema for the NamespaceSConfig API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=namespaceconfigs,scope=Cluster
// +operator-sdk:gen-csv:customresourcedefinitions.displayName="Namespace Config"
type NamespaceConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NamespaceConfigSpec   `json:"spec,omitempty"`
	Status NamespaceConfigStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NamespaceConfigList contains a list of NSConfig
type NamespaceConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NamespaceConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NamespaceConfig{}, &NamespaceConfigList{})
}
