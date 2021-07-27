package framework

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/operator-framework/operator-sdk/pkg/test"
	webserversv1alpha1 "github.com/web-servers/jws-operator/pkg/apis/webservers/v1alpha1"
	kbappsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	podv1 "k8s.io/kubernetes/pkg/api/v1/pod"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

/* Result for the demo webapp
{
  "counter": 2,
  "id": "2244ED88EBC16E2956F63107405D7CC9",
  "new": false,
  "server": "10.129.2.169",
  "hostname": "test-app-1-psvcz",
  "newtest": "2020"
}
*/
type DemoResult struct {
	Counter  int
	Id       string
	New      bool
	Server   string
	Hostname string
	Newtest  string
}

var (
	retryInterval        = time.Second * 5
	timeout              = time.Minute * 10
	cleanupRetryInterval = time.Second * 1
	cleanupTimeout       = time.Second * 5
)

var name string
var namespace string

// webServerTestSetup sets up the context and framework for the test
func webServerTestSetup(t *testing.T) (*test.Context, *test.Framework) {
	testContext := test.NewContext(t)
	err := testContext.InitializeClusterResources(&test.CleanupOptions{TestContext: testContext, Timeout: cleanupTimeout, RetryInterval: cleanupRetryInterval})
	if err != nil {
		defer testContext.Cleanup()
		t.Fatalf("Failed to initialize cluster resources: %v", err)
	}
	t.Log("Initialized cluster resources")
	namespace, err = testContext.GetOperatorNamespace()
	name = strings.ToLower(strings.Split(t.Name(), "/")[1])
	if err != nil {
		defer testContext.Cleanup()
		t.Fatalf("Failed to get namespace for testing context '%v': %v", testContext, err)
	}
	t.Logf("Testing in namespace %s", namespace)
	// get global test variables
	framework := test.Global
	return testContext, framework
}

// WebServerApplicationImageBasicTest tests the deployment of an application image operator
func WebServerApplicationImageBasicTest(t *testing.T, imageName string, testURI string) {
	testContext, framework := webServerTestSetup(t)

	webServer := makeApplicationImageWebServer(namespace, name, imageName, 1)

	webServerBasicTest(t, framework, testContext, webServer, testURI)

	testContext.Cleanup()
}

// WebServerApplicationImageScaleTest tests the scaling of an application image operator
func WebServerApplicationImageScaleTest(t *testing.T, imageName string, testURI string) {
	testContext, framework := webServerTestSetup(t)

	webServer := makeApplicationImageWebServer(namespace, name, imageName, 1)

	webServerScaleTest(t, framework, testContext, webServer, testURI)

	testContext.Cleanup()
}

// WebServerApplicationImageUpdateTest test the application image update feature of an application image operator
func WebServerApplicationImageUpdateTest(t *testing.T, imageName string, newImageName string, testURI string) {
	testContext, framework := webServerTestSetup(t)

	webServer := makeApplicationImageWebServer(namespace, name, imageName, 1)

	webServerApplicationImageUpdateTest(t, framework, testContext, webServer, newImageName, testURI)

	testContext.Cleanup()
}

// WebServerImageStreamBasicTest tests the deployment of an Image Stream operator
func WebServerImageStreamBasicTest(t *testing.T, imageStreamName string, testURI string) {
	testContext, framework := webServerTestSetup(t)

	webServer := makeImageStreamWebServer(namespace, name, imageStreamName, namespace, 1)

	webServerBasicTest(t, framework, testContext, webServer, testURI)

	testContext.Cleanup()
}

// WebServerImageStreamScaleTest tests the scaling of an Image Stream operator
func WebServerImageStreamScaleTest(t *testing.T, imageStreamName string, testURI string) {
	testContext, framework := webServerTestSetup(t)

	webServer := makeImageStreamWebServer(namespace, name, imageStreamName, namespace, 1)

	webServerScaleTest(t, framework, testContext, webServer, testURI)

	testContext.Cleanup()
}

// WebServerSourcesBasicTest tests the deployment of an Image Stream operator with sources
func WebServerSourcesBasicTest(t *testing.T, imageStreamName string, gitURL string, testURI string) {
	testContext, framework := webServerTestSetup(t)

	webServer := makeSourcesWebServer(namespace, name, imageStreamName, namespace, gitURL, 1)

	webServerBasicTest(t, framework, testContext, webServer, testURI)

	testContext.Cleanup()
}

// WebServerSourcesScaleTest tests the scaling of an Image Stream operator with sources
func WebServerSourcesScaleTest(t *testing.T, imageStreamName string, gitURL string, testURI string) {
	testContext, framework := webServerTestSetup(t)

	webServer := makeSourcesWebServer(namespace, name, imageStreamName, namespace, gitURL, 1)

	webServerScaleTest(t, framework, testContext, webServer, testURI)

	testContext.Cleanup()
}

// webServerBasicTest tests if the deployed pods of the operator are working
func webServerBasicTest(t *testing.T, framework *test.Framework, testContext *test.Context, webServer *webserversv1alpha1.WebServer, testURI string) {

	deployWebServer(framework, testContext, t, webServer)

	webServerRouteTest(framework, t, name, namespace, testURI, false, nil)

}

// webServerScaleTest tests if the deployed pods of the operator are working properly after scaling
func webServerScaleTest(t *testing.T, framework *test.Framework, testContext *test.Context, webServer *webserversv1alpha1.WebServer, testURI string) {

	deployWebServer(framework, testContext, t, webServer)

	// scale up test.
	webServerScale(t, framework, testContext, webServer, testURI, 4)

	webServerRouteTest(framework, t, name, namespace, testURI, false, nil)

	// scale down test.
	webServerScale(t, framework, testContext, webServer, testURI, 1)

	webServerRouteTest(framework, t, name, namespace, testURI, false, nil)
}

// webServerScale changes the replica number of the WebServer resource
func webServerScale(t *testing.T, framework *test.Framework, testContext *test.Context, webServer *webserversv1alpha1.WebServer, testURI string, newReplicasValue int32) {

	err := framework.Client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, webServer)
	if err != nil {
		t.Fatal(err)
	}

	webServer.Spec.Replicas = newReplicasValue

	updateWebServer(framework, t, webServer, name, namespace)

	t.Logf("Updated application %s number of replicas to %d\n", name, webServer.Spec.Replicas)

}

// webServerApplicationImageUpdateTest tests if the deployed pods of the operator are working properly after an application image update
func webServerApplicationImageUpdateTest(t *testing.T, framework *test.Framework, testContext *test.Context, webServer *webserversv1alpha1.WebServer, newImageName string, testURI string) {

	deployWebServer(framework, testContext, t, webServer)

	webServerApplicationImageUpdate(t, framework, testContext, webServer, newImageName, testURI)

	foundDeployment := &kbappsv1.Deployment{}
	err := framework.Client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, foundDeployment)
	if err != nil {
		t.Errorf("Failed to get Deployment\n")
	}

	foundImage := foundDeployment.Spec.Template.Spec.Containers[0].Image
	if foundImage != newImageName {
		t.Errorf("Found %s as application image; wanted %s", foundImage, newImageName)
	}

}

// webServerApplicationImageUpdate changes the application image of the WebServer resource
func webServerApplicationImageUpdate(t *testing.T, framework *test.Framework, testContext *test.Context, webServer *webserversv1alpha1.WebServer, newImageName string, testURI string) {

	// WebServer resource needs to be refreshed before being updated
	// to avoid "the object has been modified" errors
	err := framework.Client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, webServer)
	if err != nil {
		t.Fatal(err)
	}

	webServer.Spec.WebImage.ApplicationImage = newImageName

	updateWebServer(framework, t, webServer, name, namespace)

	t.Logf("Updated application image of WebServer %s to %s\n", name, newImageName)

}

// updateWebServer updates the WebServer resource and waits until the new deployment is ready
func updateWebServer(framework *test.Framework, t *testing.T, webServer *webserversv1alpha1.WebServer, name string, namespace string) {

	err := framework.Client.Update(context.TODO(), webServer)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("WebServer %s updated\n", name)

	// Waits until the pods are deployed
	waitUntilReady(framework, t, webServer)

}

// deployWebServer deploys a WebServer resource and waits until the pods are online
func deployWebServer(framework *test.Framework, testContext *test.Context, t *testing.T, webServer *webserversv1alpha1.WebServer) {
	// use Context's create helper to create the object and add a cleanup function for the new object
	err := framework.Client.Create(context.TODO(), webServer, &test.CleanupOptions{TestContext: testContext, Timeout: cleanupTimeout, RetryInterval: cleanupRetryInterval})
	if err != nil {
		t.Fatal(err)
	}

	// removing finalizers explicitly otherwise the removal could hang
	testContext.AddCleanupFn(
		func() error {
			// Removing deployment for not putting finalizers back to the WebServer
			name := webServer.ObjectMeta.Name
			namespace := webServer.ObjectMeta.Namespace
			deployment, err := framework.KubeClient.AppsV1().Deployments(namespace).Get("jws-operator", metav1.GetOptions{})
			if err == nil && deployment != nil {
				t.Logf("Cleaning deployment '%v'\n", deployment.Name)
				framework.Client.Delete(context.TODO(), deployment)
			}
			// Cleaning finalizer
			return wait.Poll(retryInterval, timeout, func() (done bool, err error) {
				foundServer := &webserversv1alpha1.WebServer{}
				namespacedName := types.NamespacedName{Name: name, Namespace: namespace}
				if errPoll := framework.Client.Get(context.TODO(), namespacedName, foundServer); errPoll != nil {
					if apierrors.IsNotFound(errPoll) {
						t.Logf("No WebServer object '%v' to remove the finalizer at. Probably all cleanly finished before.\n", name)
						return true, nil
					}
					t.Logf("Cannot obtain object of the WebServer '%v', cause: %v\n", name, errPoll)
					return false, nil
				}
				foundServer.SetFinalizers([]string{})
				if errPoll := framework.Client.Update(context.TODO(), foundServer); errPoll != nil {
					t.Logf("Cannot update WebServer '%v' with empty finalizers array, cause: %v\n", name, errPoll)
					return false, nil
				}
				t.Logf("Finalizer definition succesfully removed from the WebServer '%v'\n", name)
				return true, nil
			})
		},
	)

	waitUntilReady(framework, t, webServer)

	t.Logf("Application %s is deployed ", name)

}

// waitUntilReady waits until the number of pods matches the WebServer Spec replica number.
func waitUntilReady(framework *test.Framework, t *testing.T, webServer *webserversv1alpha1.WebServer) {
	name := webServer.ObjectMeta.Name
	replicas := webServer.Spec.Replicas

	t.Logf("Waiting until %[1]d/%[1]d pods for %s are ready", replicas, name)

	err := wait.Poll(retryInterval, timeout, func() (done bool, err error) {

		podList := &corev1.PodList{}
		listOpts := []client.ListOption{
			client.InNamespace(webServer.Namespace),
			client.MatchingLabels(generateLabelsForWebServer(webServer)),
		}
		err = framework.Client.List(context.TODO(), podList, listOpts...)
		if err != nil {
			if apierrors.IsNotFound(err) {
				t.Logf("List of pods %s not found", name)

				return false, nil
			}
			t.Logf("Got error when getting pod list %s: %s", name, err)
			t.Fatal(err)
		}

		// Testing for Ready
		if arePodsReady(podList, replicas) {
			return true, nil
		}

		// t.Logf("Waiting for full availability of %s pod list (%d/%d)\n", name, podList.Items, replicas)
		return false, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("(%[1]d/%[1]d) pods are ready \n", replicas)

}

// waitForRoute checks if a Route is available up to 5 times
func waitForRoute(t *testing.T, webServer *webserversv1alpha1.WebServer) {
	for i := 1; i < 7; i++ {
		if len(webServer.Status.Hosts) == 0 {
			t.Logf("WebServer.Status.Hosts is empty. Attempt %d/5\n", i)
			time.Sleep(1 * time.Second)
		} else {
			return
		}
	}
	t.Fatal("Route resource not found")
}

// webServerRouteTest tests the Route created for the operator pods
func webServerRouteTest(framework *test.Framework, t *testing.T, name string, namespace string, URI string, sticky bool, oldCookie *http.Cookie) *http.Cookie {

	webServer := &webserversv1alpha1.WebServer{}
	err := framework.Client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, webServer)
	if err != nil {
		t.Fatal(err)
	}

	waitForRoute(t, webServer)

	t.Logf("Route:  (%s)\n", webServer.Status.Hosts)
	URL := "http://" + webServer.Status.Hosts[0] + URI
	req, err := http.NewRequest("GET", URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	if oldCookie != nil {
		req.AddCookie(oldCookie)
		t.Logf("GET:  (%s) cookie: (%s)\n", URL, oldCookie.Raw)
	} else {
		t.Logf("GET:  (%s)\n", URL)
	}
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != 200 {
		t.Logf("body: %s\n", body)
		t.Fatal(errors.New(URL + " returns: " + strconv.Itoa(res.StatusCode)))
	}
	if sticky {
		// Do stickyness test.

		// read the SESSIONID cookie
		cookies := res.Cookies()
		sessionCookie := &http.Cookie{}
		sessionCookie = nil
		for _, cookie := range cookies {
			t.Logf("1-cookies: %s", cookie.Raw)
			if cookie.Name == "JSESSIONID" {
				sessionCookie = cookie
			}
		}
		if oldCookie != nil {
			if sessionCookie != nil {
				t.Fatal(errors.New(URL + " returns unexpected JSESSIONID cookies"))
			}
			sessionCookie = oldCookie
		} else {
			if sessionCookie == nil {
				t.Fatal(errors.New(URL + " doesn't return JSESSIONID cookies"))
			}
		}

		// Parse the response.
		var oldResult DemoResult
		json.Unmarshal(body, &oldResult)
		counter := 1
		if oldCookie != nil {
			// Read previous value and increase it.
			counter = oldResult.Counter
			counter++
		}
		t.Logf("%d - body: %s\n", counter, body)

		// Wait for the replication to take place... Probably something wrong there???
		time.Sleep(10 * time.Second)

		hostnames := make([]string, 0)
		hostnames = append(hostnames, oldResult.Hostname)
		for {
			// Do a another request.
			req, err := http.NewRequest("GET", URL, nil)
			if err != nil {
				t.Fatal(err)
			}
			req.AddCookie(sessionCookie)
			client = &http.Client{}
			res, err = client.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			newBody, err := ioutil.ReadAll(res.Body)
			res.Body.Close()
			if err != nil {
				t.Fatal(err)
			}
			if res.StatusCode != 200 {
				t.Logf("body: %s\n", newBody)
				t.Fatal(errors.New(URL + " second request returns: " + strconv.Itoa(res.StatusCode)))
			}
			t.Logf("%d - body: %s\n", counter, newBody)
			cookies = res.Cookies()
			newSessionCookie := &http.Cookie{}
			newSessionCookie = nil
			for _, cookie := range cookies {
				t.Logf("2-cookies: %s", cookie.Raw)
				if cookie.Name == "JSESSIONID" {
					t.Logf("Found cookies: %s", cookie.Raw)
					newSessionCookie = cookie
				}
			}
			if newSessionCookie != nil {
				t.Logf("Cookies new: %s old: %s", newSessionCookie.Raw, sessionCookie.Raw)
				t.Fatal(errors.New(URL + " Not sticky!!!"))
			}

			// Check the counter in the body.
			var result DemoResult
			json.Unmarshal(newBody, &result)
			t.Logf("Demo counter: %d", result.Counter)
			if result.Counter != counter {
				t.Fatal(errors.New(URL + " NOTOK, counter should be " + strconv.Itoa(counter) + "... Not sticky!!!"))
			}

			// And that pod name has changed...
			t.Logf("Demo pod: %s and %s", result.Hostname, strings.Join(hostnames, ","))
			found := false
			for _, hostname := range hostnames {
				t.Logf("Demo pod: %s and %s", result.Hostname, hostname)
				if hostname == result.Hostname {
					found = true
				}
			}
			if found {
				// We are on same pod... retry?...
				if webServer.Spec.Replicas == 1 {
					// Only one pod done...
					return sessionCookie
				}
				t.Logf("%s NOTOK, on the same pod... Too sticky!!! retrying", URL)
			} else {
				hostnames = append(hostnames, result.Hostname)
				if int32(len(hostnames)) == webServer.Spec.Replicas {
					return sessionCookie
				}
			}
			counter++
			time.Sleep(10 * time.Second)
		}
	}
	return nil
}

// arePodsReady checks that all the pods are ready
func arePodsReady(podList *corev1.PodList, replicas int32) bool {
	if int32(len(podList.Items)) != replicas {
		return false
	}
	for _, pod := range podList.Items {
		if !podv1.IsPodReady(&pod) {
			return false
		}
	}
	return true
}

// isOperatorLocal returns true if the LOCAL_OPERATOR env var is set to true.
func isOperatorLocal() bool {
	val, ok := os.LookupEnv("LOCAL_OPERATOR")
	if !ok {
		return false
	}
	local, err := strconv.ParseBool(val)
	if err != nil {
		return false
	}
	return local
}

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
