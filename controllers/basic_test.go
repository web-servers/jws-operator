package controllers

import (
	"context"
	"errors"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubectl/pkg/util/podutils"
	// podv1 "k8s.io/kubernetes/pkg/api/v1/pod"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	webserversv1alpha1 "github.com/web-servers/jws-operator/api/v1alpha1"
)

var (
	retryInterval = time.Second * 5
	// timeout       = time.Minute * 10
	timeout = time.Minute * 2
)

var _ = Describe("WebServer controller", func() {
	Context("First Test", func() {
		It("Basic test", func() {
			By("By creating a new WebServer")
			fmt.Printf("By creating a new WebServer\n")
			ctx := context.Background()
			webserver := &webserversv1alpha1.WebServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-webserver",
					Namespace: "default",
				},
				Spec: webserversv1alpha1.WebServerSpec{
					ApplicationName: "test-tomcat-demo",
					Replicas:        int32(2),
					WebImage: &webserversv1alpha1.WebImageSpec{
						ApplicationImage: "quay.io/jfclere/tomcat-demo",
					},
				},
			}
			Expect(k8sClient.Create(ctx, webserver)).Should(Succeed())

			// Check it is started.
			webserverLookupKey := types.NamespacedName{Name: "test-webserver", Namespace: "default"}
			createdWebserver := &webserversv1alpha1.WebServer{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, webserverLookupKey, createdWebserver)
				if err != nil {
					return false
				}
				return true
			}, time.Second*10, time.Millisecond*250).Should(BeTrue())
			fmt.Printf("new WebServer Name: %s Namespace: %s\n", createdWebserver.ObjectMeta.Name, createdWebserver.ObjectMeta.Namespace)

			// are the corresponding pods ready?
			Eventually(func() bool {
				err := waitUntilReady(ctx, createdWebserver)
				if err != nil {
					return false
				}
				return true
			}, timeout, retryInterval).Should(BeTrue())

			// remove the created webserver
			Expect(k8sClient.Delete(ctx, webserver)).Should(Succeed())

		})
	})
})

// generateLabelsForWebServer return a map of labels that are used for identification
//  of objects belonging to the particular WebServer instance
func generateLabelsForWebServer(webServer *webserversv1alpha1.WebServer) map[string]string {
	labels := map[string]string{
		"deploymentConfig": webServer.Spec.ApplicationName,
		"WebServer":        webServer.Name,
	}
	// labels["app.kubernetes.io/name"] = webServer.Name
	// labels["app.kubernetes.io/managed-by"] = os.Getenv("LABEL_APP_MANAGED_BY")
	// labels["app.openshift.io/runtime"] = os.Getenv("LABEL_APP_RUNTIME")
	if webServer.Labels != nil {
		for labelKey, labelValue := range webServer.Labels {
			labels[labelKey] = labelValue
		}
	}
	return labels
}

// arePodsReady checks that all the pods are ready
func arePodsReady(podList *corev1.PodList, replicas int32) bool {
	if int32(len(podList.Items)) != replicas {
		return false
	}
	for _, pod := range podList.Items {
		if !podutils.IsPodReady(&pod) {
			// if !corev1.IsPodReady(&pod) {
			return false
		}
	}
	return true
}

// waitUntilReady waits until the number of pods matches the WebServer Spec replica number.
func waitUntilReady(ctx context.Context, webServer *webserversv1alpha1.WebServer) (err error) {
	name := webServer.ObjectMeta.Name
	replicas := webServer.Spec.Replicas

	// t.Logf("Waiting until %[1]d/%[1]d pods for %s are ready", replicas, name)

	By("By checking pods")
	fmt.Printf("By checking pods in %s\n", webServer.Namespace)
	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(webServer.Namespace),
		client.MatchingLabels(generateLabelsForWebServer(webServer)),
	}
	err = k8sClient.List(ctx, podList, listOpts...)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// t.Logf("List of pods %s not found", name)
			fmt.Printf("List of pods %s not found\n", name)

			return nil
		}
		// t.Logf("Got error when getting pod list %s: %s", name, err)
		fmt.Printf("Got error when getting pod list %s: %s", name, err)
		// t.Fatal(err)
		return err
	}

	// Testing for Ready
	if arePodsReady(podList, replicas) {
		fmt.Printf("Done ready\n")
		// t.Logf("(%[1]d/%[1]d) pods are ready \n", replicas)
		return nil
	}

	// t.Logf("Waiting for full availability of %s pod list (%d/%d)\n", name, podList.Items, replicas)
	fmt.Printf("Waiting for full availability of %s pod list (%d/%d)\n", name, len(podList.Items), replicas)
	err = errors.New("Pods are not ready")
	return err

}
