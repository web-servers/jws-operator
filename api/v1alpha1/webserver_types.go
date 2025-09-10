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
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Application Name",order=1
	ApplicationName string `json:"applicationName"`
	// The desired number of replicas for the application
	// +kubebuilder:validation:Minimum=0
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Replicas",order=2,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:podCount"}
	Replicas int32 `json:"replicas"`
	// Use session clustering
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Enable Session Clustering in Tomcat",order=3,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	UseSessionClustering bool `json:"useSessionClustering,omitempty"`
	// Use Insights client (works only with JWS 6.1+ images)
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Enable Red Hat Insights",order=4,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	UseInsightsClient bool `json:"useInsightsClient,omitempty"`
	// (Deployment method 1) Application image
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Web Image",order=5
	WebImage *WebImageSpec `json:"webImage,omitempty"`
	// (Deployment method 2) Imagestream
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Web Image Stream",order=6
	WebImageStream *WebImageStreamSpec `json:"webImageStream,omitempty"`
	// TLS configuration for the WebServer
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="TLS Configuration",order=7
	TLSConfig TLSConfig `json:"tlsConfig,omitempty"`
	// Environment variables for the WebServer
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Environment Variables",order=8
	EnvironmentVariables []corev1.EnvVar `json:"environmentVariables,omitempty"`
	// Persistent logs configuration
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Persistent Logs",order=9
	PersistentLogsConfig PersistentLogs `json:"persistentLogs,omitempty"`
	// Configuration of the resources used by the WebServer, e.g. CPU and memory
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Pod Resources",order=10,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:resourceRequirements"}
	PodResources corev1.ResourceRequirements `json:"podResources,omitempty"`
	// Security context defines the security capabilities required to run the application
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Security Context",order=11
	SecurityContext *corev1.SecurityContext `json:"securityContext,omitempty"`
	// Specifications of volumes which will be mounted
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Volume Specifications",order=12
	Volume *VolumeSpec `json:"volumeSpec,omitempty"`
	// IsNotJWS boolean that specifies if the image is JWS or not.
	IsNotJWS bool `json:"isNotJWS,omitempty"`
}

// Volume specification
type VolumeSpec struct {
	// Names of persistent volume claims which will be mounted to /volumes
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Persistent Volume Claims",order=1
	PersistentVolumeClaims []string `json:"persistentVolumeClaims,omitempty"`
	// Names of secrets which will be mounted to /secrets
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Secrets",order=2
	Secrets []string `json:"secrets,omitempty"`
	// Names of config maps which will be mounted to /configmaps
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Config Maps",order=3
	ConfigMaps []string `json:"configMaps,omitempty"`
	// Volume Claim Templates for stateful applications
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Volume Claim Templates",order=4
	VolumeClaimTemplates []corev1.PersistentVolumeClaimSpec `json:"volumeClaimTemplates,omitempty"`
}

// (Deployment method 1) Application image
type WebImageSpec struct {
	// The name of the application image to be deployed
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Application Image",order=1
	ApplicationImage string `json:"applicationImage"`
	// secret to pull from the docker repository
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Image Pull Secret",order=2
	ImagePullSecret string `json:"imagePullSecret,omitempty"`
	// The source code for a webapp to be built and deployed
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Web App",order=3
	WebApp *WebAppSpec `json:"webApp,omitempty"`
	// Pod health checks information
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Web Server Health Check",order=4
	WebServerHealthCheck *WebServerHealthCheckSpec `json:"webServerHealthCheck,omitempty"`
}

// WebApp contains all the information required to build and deploy a web application
type WebAppSpec struct {
	// Name of the web application (default: ROOT.war)
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Name",order=1
	Name string `json:"name,omitempty"`
	// URL for the repository of the application sources
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Source Repository URL",order=2
	SourceRepositoryURL string `json:"sourceRepositoryURL"`
	// Branch in the source repository
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Source Repository Reference",order=3
	SourceRepositoryRef string `json:"sourceRepositoryRef,omitempty"`
	// Subdirectory in the source repository
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Source Repository Context Directory",order=4
	SourceRepositoryContextDir string `json:"contextDir,omitempty"`
	// Docker repository to push the built image
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Web Application War Image",order=5
	WebAppWarImage string `json:"webAppWarImage"`
	// secret to push to the docker repository
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Web Application War Image Push Secret",order=6
	WebAppWarImagePushSecret string `json:"webAppWarImagePushSecret"`
	// The information required to build the application
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Builder",order=7
	Builder *BuilderSpec `json:"builder"`
}

// Builder contains all the information required to build the web application
type BuilderSpec struct {
	// Image of the container where the web application will be built
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Image",order=1
	Image string `json:"image"`
	// The script that the BuilderImage will use to build the application war and move it to /mnt
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Application Build Script",order=2
	ApplicationBuildScript string `json:"applicationBuildScript,omitempty"`
}

// (Deployment method 2) Imagestream
type WebImageStreamSpec struct {
	// The imagestream containing the image to be deployed
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Image Stream Name",order=1
	ImageStreamName string `json:"imageStreamName"`
	// The namespace where the image stream is located
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Image Stream Namespace",order=2
	ImageStreamNamespace string `json:"imageStreamNamespace"`
	// (Optional) Source code information
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Web Sources",order=3
	WebSources *WebSourcesSpec `json:"webSources,omitempty"`
	// Pod health checks information
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Web Server Health Check",order=4
	WebServerHealthCheck *WebServerHealthCheckSpec `json:"webServerHealthCheck,omitempty"`
}

// TLS settings
type TLSConfig struct {
	// TLSSecret secret containing server.cert the server certificate, server.key the server key and optional ca.cert the CA cert of the client certificates
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="TLS Secret",order=1
	TLSSecret string `json:"tlsSecret,omitempty"`
	// TLSPassword passphrase for the key in the client.key
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="TLS Password",order=2,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:password"}
	TLSPassword string `json:"tlsPassword,omitempty"`
	// certificateVerification for tomcat configuration: required/optional or empty.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Certificate Verification",order=3
	CertificateVerification string `json:"certificateVerification,omitempty"`
	// Route behaviour:[tls]hostname/NONE or empty.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Route Hostname",order=4
	RouteHostname string `json:"routeHostname,omitempty"`
}

type PersistentLogs struct {
	//If true operator will log tomcat's catalina logs
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Catalina Logs",order=1,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	CatalinaLogs bool `json:"catalinaLogs,omitempty"`
	//If true operator will log tomcat's access logs
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Access Logs",order=2,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	AccessLogs bool `json:"enableAccessLogs,omitempty"`
	// VolumeName is the name of pv we eant to bound
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Volume Name",order=3
	VolumeName string `json:"volumeName,omitempty"`
	// StorageClass name of the storage class we want to use for the bound
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Storage Class",order=4
	StorageClass string `json:"storageClass,omitempty"`
}

// (Optional) Source code information
type WebSourcesSpec struct {
	// URL for the repository of the application sources
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Source Repository URL",order=1
	SourceRepositoryURL string `json:"sourceRepositoryUrl"`
	// Branch in the source repository
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Source Repository Reference",order=2
	SourceRepositoryRef string `json:"sourceRepositoryRef,omitempty"`
	// Subdirectory in the source repository
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Context Directory",order=3
	ContextDir string `json:"contextDir,omitempty"`
	// (Optional) Sources related parameters
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Web Sources Parameters",order=4
	WebSourcesParams *WebSourcesParamsSpec `json:"webSourcesParams,omitempty"`
	// Webhook secrets configuration
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Webhook Secrets",order=5
	WebhookSecrets *WebhookSecrets `json:"webhookSecrets,omitempty"`
}

// (Optional) Sources related parameters
type WebSourcesParamsSpec struct {
	// URL to a maven repository
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Maven Mirror URL",order=1
	MavenMirrorURL string `json:"mavenMirrorUrl,omitempty"`
	// Directory where the jar/war is created
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Artifact Directory",order=2
	ArtifactDir string `json:"artifactDir,omitempty"`
	// (Deprecated - Use WebhookSecrets instead) Secret string for a generic web hook
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Generic Webhook Secret",order=3
	GenericWebhookSecret string `json:"genericWebhookSecret,omitempty"`
	// (Deprecated - Use WebhookSecrets instead) Secret string for a Github web hook
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Github Webhook Secret",order=4
	GithubWebhookSecret string `json:"githubWebhookSecret,omitempty"`
}

type WebhookSecrets struct {
	// Secret for generic webhook
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Generic",order=1
	Generic string `json:"generic,omitempty"`
	// Secret for Github webhook
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Github",order=2
	Github string `json:"github,omitempty"`
	// Secret for Gitlab webhook
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Gitlab",order=3
	Gitlab string `json:"gitlab,omitempty"`
}

type WebServerHealthCheckSpec struct {
	// String for the pod readiness health check logic
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Server Readiness Script",order=1
	ServerReadinessScript string `json:"serverReadinessScript"`
	// String for the pod liveness health check logic
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Server Liveness Script",order=2
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
	Selector string `json:"selector,omitempty"`
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
