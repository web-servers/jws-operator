// +build !unit

package e2e

import (
	"testing"

	framework "github.com/operator-framework/operator-sdk/pkg/test"
	"github.com/web-servers/jws-operator/pkg/apis"
	jwsv1alpha1 "github.com/web-servers/jws-operator/pkg/apis/webservers/v1alpha1"
	jwsframework "github.com/web-servers/jws-operator/test/framework"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestJWS17Server(t *testing.T) {
	jwsServerList := &jwsv1alpha1.WebServerList{
		TypeMeta: metav1.TypeMeta{
			Kind:       "WebServer",
			APIVersion: "web.servers.org/v1alpha1",
		},
	}
	err := framework.AddToFrameworkScheme(apis.AddToScheme, jwsServerList)
	if err != nil {
		t.Fatalf("failed to add custom resource scheme to framework: %v", err)
	}
	// run subtests
	t.Run("BasicTest", jwsBasicTest)
}

func jwsBasicTest(t *testing.T) {
	jwsframework.JWSBasicTest(t, "17.0")
}
