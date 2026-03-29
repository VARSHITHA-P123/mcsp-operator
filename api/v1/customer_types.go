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

// CustomerSpec defines the desired state of Customer.
type CustomerSpec struct {
	// CustomerName is the name of the customer
	// This comes from the UI form via Gin backend
	CustomerName string `json:"customerName"`

	// TODO: Add more fields after confirming with teammates
	// Example: Replicas, Tier, etc.
}

// CustomerStatus defines the observed state of Customer.
type CustomerStatus struct {
	// Deployed indicates if the customer is deployed
	Deployed bool `json:"deployed,omitempty"`

	// Message gives the current status message
	Message string `json:"message,omitempty"`

	// URL is the live URL of the customer application
	// TODO: Confirm URL format with teammates
	URL string `json:"url,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Customer is the Schema for the customers API.
type Customer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CustomerSpec   `json:"spec,omitempty"`
	Status CustomerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// CustomerList contains a list of Customer.
type CustomerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Customer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Customer{}, &CustomerList{})
}
