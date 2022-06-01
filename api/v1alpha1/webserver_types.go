package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// WebServerSpec defines the desired state of WebServer
type WebServerSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

	// The base for the names of the deployed application resources
	// +kubebuilder:validation:Pattern=^[a-z]([-a-z0-9]*[a-z0-9])?$
	ApplicationName string `json:"applicationName"`
	// The desired number of replicas for the application
	// +kubebuilder:validation:Minimum=0
	Replicas int32 `json:"replicas"`
	// Use Session Clustering
	UseSessionClustering bool `json:"useSessionClustering,omitempty"`
	// Route behaviour:[TLS/tls]hostname/NONE or empty.
	RouteHostname string `json:"routeHostname,omitempty"`
	// IsNotJWS boolean that specifies if the image is JWS or not.
	IsNotJWS bool `json:"isNotJWS,omitempty"`
	// TLSSecret secret containing server.cert the server certificate, server.key the server key and optional ca.cert the CA cert of the client certificates
	TLSSecret string `json:"tlsSecret,omitempty"`
	// TLSPassword passphrase for the key in the client.key
	TLSPassword string `json:"tlsPassword,omitempty"`
	// (Deployment method 1) Application image
	WebImage *WebImageSpec `json:"webImage,omitempty"`
	// (Deployment method 2) Imagestream
	WebImageStream *WebImageStreamSpec `json:"webImageStream,omitempty"`
	// Configuration of the resources used by the WebServer, ie CPU and memory, use limits and requests
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
}

// (Deployment method 1) Application image
type WebImageSpec struct {
	// The name of the application image to be deployed
	ApplicationImage string `json:"applicationImage"`
	// secret to pull from the docker repository
	ImagePullSecret string `json:"imagePullSecret,omitempty"`
	// The source code for a webapp to be built and deployed
	WebApp *WebAppSpec `json:"webApp,omitempty"`
	// Pod health checks information
	WebServerHealthCheck *WebServerHealthCheckSpec `json:"webServerHealthCheck,omitempty"`
}

// WebApp contains all the information required to build and deploy a web application
type WebAppSpec struct {
	// Name of the web application (default: ROOT.war)
	Name string `json:"name,omitempty"`
	// URL for the repository of the application sources
	SourceRepositoryURL string `json:"sourceRepositoryURL"`
	// Branch in the source repository
	SourceRepositoryRef string `json:"sourceRepositoryRef,omitempty"`
	// Subdirectory in the source repository
	SourceRepositoryContextDir string `json:"contextDir,omitempty"`
	// Docker repository to push the built image
	WebAppWarImage string `json:"webAppWarImage"`
	// secret to push to the docker repository
	WebAppWarImagePushSecret string `json:"webAppWarImagePushSecret"`
	// The information required to build the application
	Builder *BuilderSpec `json:"builder"`
}

// Builder contains all the information required to build the web application
type BuilderSpec struct {
	// Image of the container where the web application will be built
	Image string `json:"image"`
	// The script that the BuilderImage will use to build the application war and move it to /mnt
	ApplicationBuildScript string `json:"applicationBuildScript,omitempty"`
}

// (Deployment method 2) Imagestream
type WebImageStreamSpec struct {
	// The imagestream containing the image to be deployed
	ImageStreamName string `json:"imageStreamName"`
	// The namespace where the image stream is located
	ImageStreamNamespace string `json:"imageStreamNamespace"`
	// (Optional) Source code information
	WebSources *WebSourcesSpec `json:"webSources,omitempty"`
	// Pod health checks information
	WebServerHealthCheck *WebServerHealthCheckSpec `json:"webServerHealthCheck,omitempty"`
}

// (Optional) Source code information
type WebSourcesSpec struct {
	// URL for the repository of the application sources
	SourceRepositoryURL string `json:"sourceRepositoryUrl"`
	// Branch in the source repository
	SourceRepositoryRef string `json:"sourceRepositoryRef,omitempty"`
	// Subdirectory in the source repository
	ContextDir string `json:"contextDir,omitempty"`
	// (Optional) Sources related parameters
	WebSourcesParams *WebSourcesParamsSpec `json:"webSourcesParams,omitempty"`
}

// (Optional) Sources related parameters
type WebSourcesParamsSpec struct {
	// URL to a maven repository
	MavenMirrorURL string `json:"mavenMirrorUrl,omitempty"`
	// Directory where the jar/war is created
	ArtifactDir string `json:"artifactDir,omitempty"`
	// Secret for a generic web hook
	GenericWebhookSecret string `json:"genericWebhookSecret,omitempty"`
	// Secret for a Github web hook
	GithubWebhookSecret string `json:"githubWebhookSecret,omitempty"`
}

type WebServerHealthCheckSpec struct {
	// String for the pod readiness health check logic
	ServerReadinessScript string `json:"serverReadinessScript"`
	// String for the pod liveness health check logic
	ServerLivenessScript string `json:"serverLivenessScript,omitempty"`
}

// WebServerStatus defines the observed state of WebServer
// +k8s:openapi-gen=true
type WebServerStatus struct {
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
	// selector for pods, used by HorizontalPodAutoscaler
	Selector string `json:"selector"`
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

// PodStatus defines the observed state of pods running the WebServer application
// +k8s:openapi-gen=true
type PodStatus struct {
	Name  string `json:"name"`
	PodIP string `json:"podIP"`
	// Represent the state of the Pod, it is used especially during scale down.
	// +kubebuilder:validation:Enum=ACTIVE;PENDING;FAILED
	State string `json:"state"`
}

// Web Server is the schema for the webservers API
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:subresource:scale:specpath=.spec.replicas,statuspath=.status.replicas,selectorpath=.status.selector
// +kubebuilder:resource:path=webservers,scope=Namespaced
type WebServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WebServerSpec   `json:"spec,omitempty"`
	Status WebServerStatus `json:"status,omitempty"`
}

// WebServerList contains a list of WebServer
// +kubebuilder:object:root=true
type WebServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WebServer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&WebServer{}, &WebServerList{})
}
