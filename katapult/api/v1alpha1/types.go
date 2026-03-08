package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// @cpt-dod:cpt-katapult-dod-api-cli-crd-controller:p1

// VolumeTransferSpec defines the desired state of VolumeTransfer.
type VolumeTransferSpec struct {
	SourceCluster      string `json:"sourceCluster"`
	SourcePVC          string `json:"sourcePVC"`
	DestinationCluster string `json:"destinationCluster"`
	DestinationPVC     string `json:"destinationPVC"`
	Strategy           string `json:"strategy,omitempty"`
	AllowOverwrite     bool   `json:"allowOverwrite,omitempty"`
	RetryMax           int    `json:"retryMax,omitempty"`
}

// VolumeTransferStatus defines the observed state of VolumeTransfer.
type VolumeTransferStatus struct {
	TransferID       string             `json:"transferID,omitempty"`
	Phase            string             `json:"phase,omitempty"`
	BytesTransferred int64              `json:"bytesTransferred,omitempty"`
	BytesTotal       int64              `json:"bytesTotal,omitempty"`
	ChunksCompleted  int                `json:"chunksCompleted,omitempty"`
	ChunksTotal      int                `json:"chunksTotal,omitempty"`
	Conditions       []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Source",type=string,JSONPath=`.spec.sourceCluster`
// +kubebuilder:printcolumn:name="Destination",type=string,JSONPath=`.spec.destinationCluster`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=`.metadata.creationTimestamp`

// VolumeTransfer is the Schema for the volumetransfers API.
type VolumeTransfer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VolumeTransferSpec   `json:"spec,omitempty"`
	Status VolumeTransferStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// VolumeTransferList contains a list of VolumeTransfer.
type VolumeTransferList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VolumeTransfer `json:"items"`
}
