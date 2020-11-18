package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// JbossWebServerSpec defines the desired state of JbossWebServer
type JbossWebServerSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

	// ApplicationImage is the name of the application image to be deployed
	ApplicationName string `json:"applicationName"`
	// Replicas is the desired number of replicas for the application
	// +kubebuilder:validation:Minimum=0
	Replicas int32 `json:"replicas"`
	// Image information
	JbossWebImage *JbossWebImageSpec `json:"JbossWebImage,omitempty"`
	// ImageStream information
	JbossWebImageStream *JbossWebImageStreamSpec `json:"JbossWebImageStream,omitempty"`
	// Sources information
	JbossWebSources *JbossWebSourcesSpec `json:"JbossWebSources,omitempty"`
	// Health checks information
	JbossWebServerHealthCheck *JbossWebServerHealthCheckSpec `json:"JbossWebServerHealthCheck,omitempty"`
}

// Image somewhere.
type JbossWebImageSpec struct {
	// ApplicationImage is the name of the application image to be deployed
	ApplicationImage string `json:"applicationImage"`
}

// ImageStream description
type JbossWebImageStreamSpec struct {
	// ImageStream containing our images
	ImageStreamName string `json:"imageStreamName"`
	// Space where the ImageStream is located
	ImageStreamNamespace string `json:"imageStreamNamespace"`
}

// Sources description
type JbossWebSourcesSpec struct {
	// URL for the repository of the application sources
	SourceRepositoryUrl string `json:"sourceRepositoryUrl"`
	// Branch in the source repository
	SourceRepositoryRef string `json:"sourceRepositoryRef"`
	// sub directory in the source repository
	ContextDir string `json:"contextDir"`
	// Sub not mandatory sources related parameters
	JbossWebSourcesParams *JbossWebSourcesParamsSpec `json:"JbossWebSourcesParams,omitempty"`
}

// Sources no mandatory
type JbossWebSourcesParamsSpec struct {
	// URL to a maven repository
	MavenMirrorUrl string `json:"mavenMirrorUrl,omitempty"`
	// Directory where the jar/war are created.
	ArtifactDir string `json:"artifactDir,omitempty"`
	// Secret for generic web hook
	GenericWebhookSecret string `json:"genericWebhookSecret,omitempty"`
	// Secret for Github web hook
	GithubWebhookSecret string `json:"githubWebhookSecret,omitempty"`
}

type JbossWebServerHealthCheckSpec struct {
	// String for the readyness health check logic
	ServerReadinessScript string `json:"serverReadinessScript"`
	// String for the alive health check logic
	ServerLivenessScript string `json:"serverLivenessScript,omitempty"`
	// Username and Password are for pre 5.4 images
	JbossWebServer53HealthCheck *JbossWebServer53HealthCheckSpec `json:"JbossWebServer53HealthCheck,omitempty"`
}
type JbossWebServer53HealthCheckSpec struct {
	// Admin User Name for the tomcat-users.xml
	JwsAdminUsername string `json:"jwsAdminUsername"`
	// Password for the Admin User in the tomcat-users.xml
	JwsAdminPassword string `json:"jwsAdminPassword"`
}

// JbossWebServerStatus defines the observed state of JbossWebServer
// +k8s:openapi-gen=true
type JbossWebServerStatus struct {
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

// PodStatus defines the observed state of pods running the JbossWebServer application
// +k8s:openapi-gen=true
type PodStatus struct {
	Name  string `json:"name"`
	PodIP string `json:"podIP"`
	// Represent the state of the Pod, it is used especially during scale down.
	// +kubebuilder:validation:Enum=ACTIVE;PENDING;FAILED
	State string `json:"state"`
}

// JbossWebServer is the Schema for the jbosswebservers API
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=jbosswebservers,scope=Namespaced
type JbossWebServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   JbossWebServerSpec   `json:"spec,omitempty"`
	Status JbossWebServerStatus `json:"status,omitempty"`
}

// JbossWebServerList contains a list of JbossWebServer
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type JbossWebServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []JbossWebServer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&JbossWebServer{}, &JbossWebServerList{})
}
