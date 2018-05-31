package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ImagePolicyRequirement is a specification for a ImagePolicyRequirement resource
type ImagePolicyRequirement struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ImagePolicyRequirementSpec `json:"spec"`
}

// PackageVulernerabilityRequirements is the requirements for package vulnz for an ImagePolicyRequirement
type PackageVulernerabilityRequirements struct {
	MaximumSeverity    string   `json:"maximumSeverity"`
	OnlyFixesAvailable bool     `json:"onlyFixesAvailable`
	Whitelist          []string `json:"whitelist"`
}

// ImagePolicyRequirementSpec is the spec for a ImagePolicyRequirement resource
type ImagePolicyRequirementSpec struct {
	PackageVulernerabilityRequirements PackageVulernerabilityRequirements `json:"packageVulnerabilityRequirements`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ImagePolicyRequirementList is a list of ImagePolicyRequirement resources
type ImagePolicyRequirementList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ImagePolicyRequirement `json:"items"`
}
