package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

// @cpt-dod:cpt-katapult-dod-api-cli-crd-controller:p1

var (
	GroupVersion  = schema.GroupVersion{Group: "katapult.io", Version: "v1alpha1"}
	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}
	AddToScheme   = SchemeBuilder.AddToScheme
)

func init() {
	SchemeBuilder.Register(&VolumeTransfer{}, &VolumeTransferList{})
}
