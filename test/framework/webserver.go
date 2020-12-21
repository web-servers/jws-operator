package framework

import (
	webserversv1alpha1 "github.com/web-servers/jws-operator/pkg/apis/webservers/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MakeBasicWebServer creates a basic WebServer resource
func MakeBasicWebServer(ns, name, applicationImage string, size int32) *webserversv1alpha1.WebServer {
	return &webserversv1alpha1.WebServer{
		TypeMeta: metav1.TypeMeta{
			Kind:       "WebServer",
			APIVersion: "web.servers.org/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: webserversv1alpha1.WebServerSpec{
			Replicas:        size,
			ApplicationName: name,
			WebImage: &webserversv1alpha1.WebImageSpec{
				ApplicationImage: applicationImage,
			},
		},
	}
}

// MakeImageStreamWebServer creates a WebServer using an ImageStream
func MakeImageStreamWebServer(ns, name, imageStreamName string, imageStreamNamespace string, size int32) *webserversv1alpha1.WebServer {
	return &webserversv1alpha1.WebServer{
		TypeMeta: metav1.TypeMeta{
			Kind:       "WebServer",
			APIVersion: "web.servers.org/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: webserversv1alpha1.WebServerSpec{
			Replicas:        size,
			ApplicationName: name,
			WebImageStream: &webserversv1alpha1.WebImageStreamSpec{
				ImageStreamName:      imageStreamName,
				ImageStreamNamespace: imageStreamNamespace,
			},
		},
	}
}

// MakeSourcesWebServer creates a WebServer using an ImageStream and sources
func MakeSourcesWebServer(ns, name, imageStreamName string, imageStreamNamespace string, url string, size int32) *webserversv1alpha1.WebServer {
	return &webserversv1alpha1.WebServer{
		TypeMeta: metav1.TypeMeta{
			Kind:       "WebServer",
			APIVersion: "web.servers.org/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: webserversv1alpha1.WebServerSpec{
			Replicas:        size,
			ApplicationName: name,
			WebImageStream: &webserversv1alpha1.WebImageStreamSpec{
				ImageStreamName:      imageStreamName,
				ImageStreamNamespace: imageStreamNamespace,
				WebSources: &webserversv1alpha1.WebSourcesSpec{
					SourceRepositoryUrl: url,
					SourceRepositoryRef: "master",
					ContextDir:          "/",
				},
			},
		},
	}
}
