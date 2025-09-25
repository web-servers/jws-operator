package e2e

import (
	"fmt"
	"time"

	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	webserversv1alpha1 "github.com/web-servers/jws-operator/api/v1alpha1"
)

func createWebServer(webServer *webserversv1alpha1.WebServer) {
	// create the webserver
	Expect(k8sClient.Create(ctx, webServer)).Should(Succeed())
	createdWebserver := getWebServer(webServer.Name)
	fmt.Printf("new WebServer Name: %s Namespace: %s\n", createdWebserver.ObjectMeta.Name, createdWebserver.ObjectMeta.Namespace)
}

func deleteWebServer(webServer *webserversv1alpha1.WebServer) {
	k8sClient.Delete(ctx, webServer)
	webserverLookupKey := types.NamespacedName{Name: webServer.Name, Namespace: namespace}

	Eventually(func() bool {
		err := k8sClient.Get(ctx, webserverLookupKey, &webserversv1alpha1.WebServer{})
		return apierrors.IsNotFound(err)
	}, "2m", "5s").Should(BeTrue(), "the webserver should be deleted")
}

func getWebServer(name string) *webserversv1alpha1.WebServer {
	createdWebserver := &webserversv1alpha1.WebServer{}
	webserverLookupKey := types.NamespacedName{Name: name, Namespace: namespace}

	Eventually(func() bool {
		err := k8sClient.Get(ctx, webserverLookupKey, createdWebserver)
		if err != nil {
			return false
		}
		return true
	}, time.Second*10, time.Millisecond*250).Should(BeTrue())
	fmt.Printf("new WebServer Name: %s Namespace: %s\n", createdWebserver.ObjectMeta.Name, createdWebserver.ObjectMeta.Namespace)

	return createdWebserver
}

func updateWebServer(webServer *webserversv1alpha1.WebServer) {
	Eventually(func() bool {
		err := k8sClient.Update(ctx, webServer)
		if err != nil {
			return false
		}
		thetest.Logf("WebServer %s updated\n", webServer.Name)
		return true
	}, time.Second*10, time.Millisecond*250).Should(BeTrue())
}
