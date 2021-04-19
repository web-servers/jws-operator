package framework

import (
	webserversv1alpha1 "github.com/web-servers/jws-operator/pkg/apis/webservers/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MakeBasicWebServer creates a basic WebServer resource
func makeApplicationImageWebServer(namespace string, name string, applicationImage string, replicas int32) *webserversv1alpha1.WebServer {
	return &webserversv1alpha1.WebServer{
		TypeMeta: metav1.TypeMeta{
			Kind:       "WebServer",
			APIVersion: "web.servers.org/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: webserversv1alpha1.WebServerSpec{
			Replicas:        replicas,
			ApplicationName: name,
			WebImage: &webserversv1alpha1.WebImageSpec{
				ApplicationImage: applicationImage,
			},
		},
	}
}

// MakeImageStreamWebServer creates a WebServer using an ImageStream
func makeImageStreamWebServer(namespace string, name string, imageStreamName string, imageStreamNamespace string, replicas int32) *webserversv1alpha1.WebServer {
	return &webserversv1alpha1.WebServer{
		TypeMeta: metav1.TypeMeta{
			Kind:       "WebServer",
			APIVersion: "web.servers.org/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: webserversv1alpha1.WebServerSpec{
			Replicas:        replicas,
			ApplicationName: name,
			WebImageStream: &webserversv1alpha1.WebImageStreamSpec{
				ImageStreamName:      imageStreamName,
				ImageStreamNamespace: imageStreamNamespace,
			},
		},
	}
}

// makeSourcesWebServer creates a WebServer using an ImageStream and sources
func makeSourcesWebServer(namespace string, name string, imageStreamName string, imageStreamNamespace string, URL string, replicas int32) *webserversv1alpha1.WebServer {
	webServer := makeImageStreamWebServer(namespace, name, imageStreamName, imageStreamNamespace, replicas)
	webServer.Spec.UseSessionClustering = true
	webServer.Spec.WebImageStream = &webserversv1alpha1.WebImageStreamSpec{
		ImageStreamName:      imageStreamName,
		ImageStreamNamespace: imageStreamNamespace,
		WebSources: &webserversv1alpha1.WebSourcesSpec{
			SourceRepositoryURL: URL,
			SourceRepositoryRef: "master",
			ContextDir:          "/",
		},
	}
	return webServer
}
