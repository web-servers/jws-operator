package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// JBossWebServerSpec defines the desired state of JBossWebServer
type JBossWebServerSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

	ApplicationName      string `json:"applicationName"`
	HostnameHttp         string `json:"hostnameHttp"`
	SourceRepositoryUrl  string `json:"sourceRepositoryUrl"`
	SourceRepositoryRef  string `json:"sourceRepositoryRef"`
	ContextDir           string `json:"contextDir"`
	JwsAdminUsername     string `json:"jwsAdminUsername"`
	JwsAdminPassword     string `json:"jwsAdminPassword"`
	GithubWebhookSecret  string `json:"githubWebhookSecret"`
	GenericWebhookSecret string `json:"genericWebhookSecret"`
	ImageStreamNamespace string `json:"imageStreamNamespace"`
	ImageStreamName      string `json:"imageStreamName"`
	MavenMirrorUrl       string `json:"mavenMirrorUrl"`
	ArtifactDir          string `json:"artifactDir"`
	Replicas             int32  `json:"replicas"`
}

// JBossWebServerStatus defines the observed state of JBossWebServer
type JBossWebServerStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// JBossWebServer is the Schema for the jbosswebservers API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=jbosswebservers,scope=Namespaced
type JBossWebServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   JBossWebServerSpec   `json:"spec,omitempty"`
	Status JBossWebServerStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// JBossWebServerList contains a list of JBossWebServer
type JBossWebServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []JBossWebServer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&JBossWebServer{}, &JBossWebServerList{})
}
