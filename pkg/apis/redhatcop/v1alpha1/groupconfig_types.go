package v1alpha1

import (
	"github.com/redhat-cop/operator-utils/pkg/util/apis"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// GroupConfigSpec defines the desired state of GroupConfig
// There are two selectors: "labelSelector", "annotationSelector".
// Selectors are considered in AND, so if multiple are defined they must all be true for a Group to be selected.
type GroupConfigSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

	// LabelSelector selects Groups by label.
	// +kubebuilder:validation:Optional
	LabelSelector metav1.LabelSelector `json:"labelSelector,omitempty"`

	// AnnotationSelector selects Groups by annotation.
	// +kubebuilder:validation:Optional
	AnnotationSelector metav1.LabelSelector `json:"annotationSelector,omitempty"`

	// +kubebuilder:validation:Optional
	Templates []apis.LockedResourceTemplate `json:"templates,omitempty"`
}

// GroupConfigStatus defines the observed state of GroupConfig
type GroupConfigStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

	apis.EnforcingReconcileStatus `json:",inline"`
}

func (m *GroupConfig) GetEnforcingReconcileStatus() apis.EnforcingReconcileStatus {
	return m.Status.EnforcingReconcileStatus
}

func (m *GroupConfig) SetEnforcingReconcileStatus(reconcileStatus apis.EnforcingReconcileStatus) {
	m.Status.EnforcingReconcileStatus = reconcileStatus
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GroupConfig is the Schema for the groupconfigs API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=groupconfigs,scope=Cluster
type GroupConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GroupConfigSpec   `json:"spec,omitempty"`
	Status GroupConfigStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GroupConfigList contains a list of GroupConfig
type GroupConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GroupConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GroupConfig{}, &GroupConfigList{})
}
