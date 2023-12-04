package framework

import (
	webserversv1alpha1 "github.com/web-servers/jws-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func makeSecureWebserver(namespace string, name string, imageStreamName string, imageStreamNamespace string, replicas int32, defaultIngressDomain string, usesessionclustering bool) *webserversv1alpha1.WebServer {
	host := "hosttest-" + namespace + "."
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
			ApplicationName:      name,
			Replicas:             replicas,
			UseSessionClustering: usesessionclustering,
			RouteHostname:        "tls:" + host + defaultIngressDomain,
			TLSSecret:            "test-tls-secret",
			WebImageStream: &webserversv1alpha1.WebImageStreamSpec{
				ImageStreamName:      imageStreamName,
				ImageStreamNamespace: imageStreamNamespace,
			},
		},
	}
}

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

// makeApplicationImageSourcesWebServer creates a WebServer using an application iamge and sources
func makeApplicationImageSourcesWebServer(namespace string, name string, image string, sourceRepositoryURL string, sourceRepositoryRef string, pushedimage string, pushsecret string, warname string, imagebuilder string, replicas int32) *webserversv1alpha1.WebServer {
	webServer := makeApplicationImageWebServer(namespace, name, image, replicas)
	webServer.Spec.UseSessionClustering = true
	webServer.Spec.WebImage = &webserversv1alpha1.WebImageSpec{
		ApplicationImage: image,
		WebApp: &webserversv1alpha1.WebAppSpec{
			Name:                     warname,
			SourceRepositoryURL:      sourceRepositoryURL,
			SourceRepositoryRef:      sourceRepositoryRef,
			WebAppWarImagePushSecret: pushsecret,
			WebAppWarImage:           pushedimage,
			Builder: &webserversv1alpha1.BuilderSpec{
				Image: imagebuilder,
			},
		},
	}
	return webServer
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

// makeImageStreamSourcesWebServer creates a WebServer using an ImageStream and sources
func makeImageStreamSourcesWebServer(namespace string, name string, imageStreamName string, imageStreamNamespace string, sourceRepositoryURL string, sourceRepositoryRef string, replicas int32) *webserversv1alpha1.WebServer {
	webServer := makeImageStreamWebServer(namespace, name, imageStreamName, imageStreamNamespace, replicas)
	webServer.Spec.UseSessionClustering = true
	webServer.Spec.WebImageStream = &webserversv1alpha1.WebImageStreamSpec{
		ImageStreamName:      imageStreamName,
		ImageStreamNamespace: imageStreamNamespace,
		WebSources: &webserversv1alpha1.WebSourcesSpec{
			SourceRepositoryURL: sourceRepositoryURL,
			SourceRepositoryRef: sourceRepositoryRef,
		},
	}
	return webServer
}
