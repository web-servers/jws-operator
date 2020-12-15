package framework

import (
	jwsv1alpha1 "github.com/web-servers/jws-operator/pkg/apis/webservers/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MakeBasicJWSServer creates a basic JWSServer resource
func MakeBasicJWSServer(ns, name, applicationImage string, size int32) *jwsv1alpha1.WebServer {
	return &jwsv1alpha1.WebServer{
		TypeMeta: metav1.TypeMeta{
			Kind:       "WebServer",
			APIVersion: "web.servers.org/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: jwsv1alpha1.WebServerSpec{
			Replicas:        size,
			ApplicationName: name,
			WebImage: &jwsv1alpha1.WebImageSpec{
				ApplicationImage: applicationImage,
			},
		},
	}
}
