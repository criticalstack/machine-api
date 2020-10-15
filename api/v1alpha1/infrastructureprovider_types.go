/*
Copyright 2020 Critical Stack, LLC

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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// InfrastructureProviderSpec defines the desired state of InfrastructureProvider
type InfrastructureProviderSpec struct {
	InfrastructureRef corev1.ObjectReference `json:"infrastructureRef"`
}

// InfrastructureProviderSpec defines the desired state of InfrastructureProvider
type InfrastructureProviderStatus struct {
	Ready bool `json:"ready"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Provider Type",type="string",JSONPath=".spec.infrastructureRef.kind",description="InfrastructureProvider kind"
// +kubebuilder:printcolumn:name="Provider Namespace",type="string",JSONPath=".spec.infrastructureRef.namespace",description="InfrastructureProvider namespace"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.ready",description="Provider is ready"
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// InfrastructureProvider is the Schema for the infrastructureproviders API
type InfrastructureProvider struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InfrastructureProviderSpec   `json:"spec,omitempty"`
	Status InfrastructureProviderStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// InfrastructureProviderList contains a list of InfrastructureProvider
type InfrastructureProviderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []InfrastructureProvider `json:"items"`
}

func init() {
	SchemeBuilder.Register(&InfrastructureProvider{}, &InfrastructureProviderList{})
}
