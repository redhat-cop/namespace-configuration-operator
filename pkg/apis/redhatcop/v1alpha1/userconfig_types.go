package v1alpha1

import (
	"github.com/redhat-cop/operator-utils/pkg/util/apis"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// UserConfigSpec defines the desired state of UserConfig
type UserConfigSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

	//IdentityExtraSelector allows you to specify a selector for the extra field of the user idenitities.
	//If one of the user identities matches the selector the user is selected
	//This condition is in OR with ProviderName
	// +kubebuilder:validation:Optional
	IdentityExtraSelector metav1.LabelSelector `json:"identityExtraSelector,omitempry"`

	//ProviderName allows you to specify an idenitity provider. If a user logged in with that provider it is selected.
	//This condition is in OR with IdentityExtraSelector
	ProviderName string `json:"providerName,omitempry"`

	// +kubebuilder:validation:Optional
	Templates []apis.LockedResourceTemplate `json:"templates,omitempry"`
}

// UserConfigStatus defines the observed state of UserConfig
type UserConfigStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

	apis.EnforcingReconcileStatus `json:",inline"`
}

func (m *UserConfig) GetEnforcingReconcileStatus() apis.EnforcingReconcileStatus {
	return m.Status.EnforcingReconcileStatus
}

func (m *UserConfig) SetEnforcingReconcileStatus(reconcileStatus apis.EnforcingReconcileStatus) {
	m.Status.EnforcingReconcileStatus = reconcileStatus
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// UserConfig is the Schema for the userconfigs API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=userconfigs,scope=Cluster
type UserConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   UserConfigSpec   `json:"spec,omitempty"`
	Status UserConfigStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// UserConfigList contains a list of UserConfig
type UserConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []UserConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&UserConfig{}, &UserConfigList{})
}
