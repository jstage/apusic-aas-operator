/*


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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type SessionCache struct {
	Type string `json:"type"`
	Size int32  `json:"size,omitempty"`
}
type CacheStatus struct {
	SessionCache SessionCache `json:"sessionCache,omitempty"`
}

// ApusicAsSpec defines the desired state of ApusicAs
type ApusicAsSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Name             string       `json:"name,omitempty"`
	Replicas         int32        `json:"replicas,omitempty"`
	SessionCache     SessionCache `json:"sessionCache,omitempty"`
	LicenseConfigRef string       `json:"licenseConfigRef,omitempty"`
	OssSecertRef     string       `json:"ossSecertRef,omitempty"`
	OssUrl           string       `json:"ossUrl,omitempty"`
}

// ApusicAsStatus defines the observed state of ApusicAs
type ApusicAsStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Nodes       []string    `json:"nodes,omitempty"`
	CacheStatus CacheStatus `json:"cacheStatus,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// ApusicAs is the Schema for the apusicas API
type ApusicAs struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ApusicAsSpec   `json:"spec,omitempty"`
	Status ApusicAsStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ApusicAsList contains a list of ApusicAs
type ApusicAsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ApusicAs `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ApusicAs{}, &ApusicAsList{})
}
