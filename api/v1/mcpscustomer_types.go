/*
Copyright 2026.
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
package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MCPSCustomerSpec defines the desired state of MCPSCustomer.
type MCPSCustomerSpec struct {
	// CustomerName is the name of the customer
	CustomerName string `json:"customerName"`
}

// MCPSCustomerStatus defines the observed state of MCPSCustomer.
type MCPSCustomerStatus struct {
	// Deployed indicates if the customer is deployed
	Deployed bool `json:"deployed,omitempty"`

	// Message gives the current status message
	Message string `json:"message,omitempty"`

	// URL is the live URL of the customer application
	URL string `json:"url,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// MCPSCustomer is the Schema for the mcpscustomers API.
type MCPSCustomer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MCPSCustomerSpec   `json:"spec,omitempty"`
	Status MCPSCustomerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MCPSCustomerList contains a list of MCPSCustomer.
type MCPSCustomerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MCPSCustomer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MCPSCustomer{}, &MCPSCustomerList{})
}
