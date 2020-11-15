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

	// ApplicationImage is the name of the application image to be deployed
	ApplicationImage    string `json:"applicationImage"`
	ApplicationName     string `json:"applicationName"`
	SourceRepositoryUrl string `json:"sourceRepositoryUrl"`
	SourceRepositoryRef string `json:"sourceRepositoryRef"`
	ContextDir          string `json:"contextDir"`
	// Username and Password are for pre 5.4 images
	JwsAdminUsername string `json:"jwsAdminUsername"`
	JwsAdminPassword string `json:"jwsAdminPassword"`
	// Corresponding Strings from the health check logics
	ServerReadinessScript string `json:"serverReadinessScript"`
	ServerLivenessScript  string `json:"serverLivenessScript"`
	GithubWebhookSecret   string `json:"githubWebhookSecret"`
	GenericWebhookSecret  string `json:"genericWebhookSecret"`
	ImageStreamNamespace  string `json:"imageStreamNamespace"`
	ImageStreamName       string `json:"imageStreamName"`
	MavenMirrorUrl        string `json:"mavenMirrorUrl"`
	ArtifactDir           string `json:"artifactDir"`
	// Replicas is the desired number of replicas for the application
	Replicas int32 `json:"replicas"`
}

// JBossWebServerStatus defines the observed state of JBossWebServer
// +k8s:openapi-gen=true
type JBossWebServerStatus struct {
	// Replicas is the actual number of replicas for the application
	Replicas int32 `json:"replicas"`
	// +listType=atomic
	Pods []PodStatus `json:"pods,omitempty"`
	// +listType=set
	Hosts []string `json:"hosts,omitempty"`
	// Represents the number of pods which are in scaledown process
	// what particular pod is scaling down can be verified by PodStatus
	//
	// Read-only.
	ScalingdownPods int32 `json:"scalingdownPods"`
}

const (
	// PodStateActive represents PodStatus.State when pod is active to serve requests
	// it's connected in the Service load balancer
	PodStateActive = "ACTIVE"
	// PodStatePending represents PodStatus.State when pod is pending
	PodStatePending = "PENDING"
	// PodStateFailed represents PodStatus.State when pod has failed
	PodStateFailed = "FAILED"
)

// PodStatus defines the observed state of pods running the JBossWebServer application
// +k8s:openapi-gen=true
type PodStatus struct {
	Name  string `json:"name"`
	PodIP string `json:"podIP"`
	// Represent the state of the Pod, it is used especially during scale down.
	// +kubebuilder:validation:Enum=ACTIVE;PENDING;FAILED
	State string `json:"state"`
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
