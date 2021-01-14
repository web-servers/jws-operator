// +build !unit

package e2e

import (
	"testing"

	framework "github.com/operator-framework/operator-sdk/pkg/test"
	"github.com/web-servers/jws-operator/pkg/apis"
	webserversv1alpha1 "github.com/web-servers/jws-operator/pkg/apis/webservers/v1alpha1"
	webserversframework "github.com/web-servers/jws-operator/test/framework"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestWebServer54(t *testing.T) {
	webServerList := &webserversv1alpha1.WebServerList{
		TypeMeta: metav1.TypeMeta{
			Kind:       "WebServer",
			APIVersion: "web.servers.org/v1alpha1",
		},
	}
	err := framework.AddToFrameworkScheme(apis.AddToScheme, webServerList)
	if err != nil {
		t.Fatalf("failed to add custom resource scheme to framework: %v", err)
	}
	// run subtests
	t.Run("BasicTest", webServerBasicTest)
	t.Run("ImageStreamTest", webServerImageStreamTest)
	t.Run("SourcesTest", webServerSourcesTest)
}

func webServerBasicTest(t *testing.T) {
	webserversframework.WebServerBasicTest(t, "quay.io/jfclere/jws-image:5.4", "/health")
}

func webServerImageStreamTest(t *testing.T) {
	webserversframework.WebServerImageStreamTest(t, "jboss-webserver54-openjdk8-tomcat9-ubi8-openshift", "/health")
}

func webServerSourcesTest(t *testing.T) {
	webserversframework.WebServerSourcesTest(t, "jboss-webserver54-openjdk8-tomcat9-ubi8-openshift", "https://github.com/jfclere/demo-webapp", "/demo-1.0/demo")
}
