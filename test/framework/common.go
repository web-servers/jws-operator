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
	timeout              = time.Minute * 5
	cleanupRetryInterval = time.Second * 1
	cleanupTimeout       = time.Second * 5
)

var name string
var namespace string

func init() {
	name = "example-webserver-" + strconv.FormatInt(time.Now().UnixNano(), 10)
}

func webServerTestSetup(t *testing.T) (*test.Context, *test.Framework) {
	testContext := test.NewContext(t)
	err := testContext.InitializeClusterResources(&test.CleanupOptions{TestContext: testContext, Timeout: cleanupTimeout, RetryInterval: cleanupRetryInterval})
	if err != nil {
		defer testContext.Cleanup()
		t.Fatalf("Failed to initialize cluster resources: %v", err)
	}
	t.Log("Initialized cluster resources")
	namespace, err = testContext.GetOperatorNamespace()
	if err != nil {
		defer testContext.Cleanup()
		t.Fatalf("Failed to get namespace for testing context '%v': %v", testContext, err)
	}
	t.Logf("Testing in namespace %s", namespace)
	// get global test variables
	framework := test.Global
	return testContext, framework
}

// WebServerBasicTest runs basic operator tests
func WebServerBasicTest(t *testing.T, imageName string, testURI string) {
	testContext, framework := webServerTestSetup(t)
	defer testContext.Cleanup()

	webServer := makeBasicWebServer(namespace, name, imageName, 1)

	err := webServerScaleTest(t, framework, testContext, webServer, testURI)
	if err != nil {
		t.Fatal(err)
	}
}

// WebServerChangeApplicationImageTest test if the application image is updated correctly
func WebServerUpdateApplicationImageTest(t *testing.T, imageName string, newImageName string, testURI string) {
	testContext, framework := webServerTestSetup(t)
	defer testContext.Cleanup()

	webServer := makeBasicWebServer(namespace, name, imageName, 1)

	err := webServerApplicationImageUpdateTest(t, framework, testContext, webServer, newImageName, testURI)
	if err != nil {
		t.Fatal(err)
	}
}

// WebServermageStreamTest runs Image Stream operator tests
func WebServerImageStreamTest(t *testing.T, imageStreamName string, testURI string) {
	testContext, framework := webServerTestSetup(t)
	defer testContext.Cleanup()

	webServer := makeImageStreamWebServer(namespace, name, imageStreamName, namespace, 1)

	err := webServerScaleTest(t, framework, testContext, webServer, testURI)
	if err != nil {
		t.Fatal(err)
	}

}

func WebServerSourcesTest(t *testing.T, imageStreamName string, gitURL string, testURI string) {
	testContext, framework := webServerTestSetup(t)
	defer testContext.Cleanup()

	webServer := makeSourcesWebServer(namespace, name, imageStreamName, namespace, gitURL, 1)

	err := webServerScaleTest(t, framework, testContext, webServer, testURI)
	if err != nil {
		t.Fatal(err)
	}
}

func webServerScaleTest(t *testing.T, framework *test.Framework, testContext *test.Context, webServer *webserversv1alpha1.WebServer, testURI string) error {

	err := deployWebServer(framework, testContext, t, webServer)
	if err != nil {
		return err
	}

	// scale up test.
	err = webServerScale(t, framework, testContext, webServer, testURI, 4)
	if err != nil {
		return err
	}

	// scale down test.
	err = webServerScale(t, framework, testContext, webServer, testURI, 1)
	if err != nil {
		return err
	}

	return nil
}

func webServerScale(t *testing.T, framework *test.Framework, testContext *test.Context, webServer *webserversv1alpha1.WebServer, testURI string, newReplicasValue int32) error {

	err := framework.Client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, webServer)
	if err != nil {
		return err
	}

	webServer.Spec.Replicas = newReplicasValue

	err = updateWebServer(framework, t, webServer, name, namespace)
	if err != nil {
		return err
	}

	t.Logf("Updated application %s replica size to %d\n", name, webServer.Spec.Replicas)

	_, err = testRouteWebServer(framework, t, name, namespace, testURI, false, nil)
	if err != nil {
		return err
	}
	return nil
}

func webServerApplicationImageUpdateTest(t *testing.T, framework *test.Framework, testContext *test.Context, webServer *webserversv1alpha1.WebServer, newImageName string, testURI string) error {

	err := deployWebServer(framework, testContext, t, webServer)
	if err != nil {
		t.Fatal(err)
	}

	// WebServer resource needs to be refreshed before being updated
	// to avoid "the object has been modified" errors
	err = framework.Client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, webServer)
	if err != nil {
		return err
	}

	webServer.Spec.WebImage.ApplicationImage = newImageName

	err = updateWebServer(framework, t, webServer, name, namespace)
	if err != nil {
		return err
	}

	t.Logf("Updated application image of WebServer %s to %s\n", name, newImageName)

	foundDeployment := &kbappsv1.Deployment{}
	err = framework.Client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, foundDeployment)
	if err != nil {
		t.Errorf("Failed to get Deployment\n")
	}

	foundImage := foundDeployment.Spec.Template.Spec.Containers[0].Image
	if foundImage != newImageName {
		t.Errorf("Found %s as application image; wanted %s", foundImage, newImageName)
	}

	return nil
}

func updateWebServer(framework *test.Framework, t *testing.T, webServer *webserversv1alpha1.WebServer, name string, namespace string) error {

	err := framework.Client.Update(context.TODO(), webServer)
	if err != nil {
		return err
	}

	t.Logf("WebServer %s updated\n", name)

	// Waits until the pods are deployed
	err = waitUntilReady(framework, t, webServer)
	if err != nil {
		return err
	}
	return nil
}

// deployWebServer deploys a WebServer resource and waits until the pods are online
func deployWebServer(framework *test.Framework, testContext *test.Context, t *testing.T, server *webserversv1alpha1.WebServer) error {
	// use Context's create helper to create the object and add a cleanup function for the new object
	err := framework.Client.Create(context.TODO(), server, &test.CleanupOptions{TestContext: testContext, Timeout: cleanupTimeout, RetryInterval: cleanupRetryInterval})
	if err != nil {
		return err
	}

	// removing finalizers explicitly otherwise the removal could hang
	testContext.AddCleanupFn(
		func() error {
			// Removing deployment for not putting finalizers back to the WebServer
			name := server.ObjectMeta.Name
			namespace := server.ObjectMeta.Namespace
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

	err = waitUntilReady(framework, t, server)
	if err != nil {
		return err
	}

	t.Logf("Application %s is deployed ", name)

	return nil
}

// waitUntilReady waits until the number of pods matches the server spec size.
func waitUntilReady(framework *test.Framework, t *testing.T, server *webserversv1alpha1.WebServer) error {
	name := server.ObjectMeta.Name
	size := server.Spec.Replicas

	t.Logf("Waiting until pods for %s are ready with size of %v", name, size)

	err := wait.Poll(retryInterval, timeout, func() (done bool, err error) {

		podList := &corev1.PodList{}
		listOpts := []client.ListOption{
			client.InNamespace(server.Namespace),
			client.MatchingLabels(LabelsForWeb(server)),
		}
		err = framework.Client.List(context.TODO(), podList, listOpts...)
		if err != nil {
			if apierrors.IsNotFound(err) {
				t.Logf("List of pods %s not found", name)

				return false, nil
			}
			t.Logf("Got error when getting pod list %s: %s", name, err)
			return false, err
		}

		// Testing for Ready
		if arePodsReady(podList, size) {
			return true, nil
		}

		// t.Logf("Waiting for full availability of %s pod list (%d/%d)\n", name, podList.Items, size)
		return false, nil
	})
	if err != nil {
		return err
	}
	t.Logf("pods available (%d/%d)\n", size, size)

	return nil
}


// Test the route
func testRouteWebServer(framework *test.Framework, t *testing.T, name string, namespace string, uri string, sticky bool, oldco *http.Cookie) (*http.Cookie, error) {

	webServer := &webserversv1alpha1.WebServer{}
	err := framework.Client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, webServer)
	if err != nil {
		return nil, err
	}
	t.Logf("route:  (%s)\n", webServer.Status.Hosts)
	url := "http://" + webServer.Status.Hosts[0] + uri
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	if oldco != nil {
		req.AddCookie(oldco)
		t.Logf("doing get:  (%s) cookie: (%s)\n", url, oldco.Raw)
	} else {
		t.Logf("doing get:  (%s)\n", url)
	}
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	body, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		return nil, err
	}
	if res.StatusCode != 200 {
		t.Logf("body: %s\n", body)
		return nil, errors.New(url + " returns: " + strconv.Itoa(res.StatusCode))
	}
	if sticky {
		// Do stickyness test.

		// read the SESSIONID cookie
		cookie := res.Cookies()
		sessionco := &http.Cookie{}
		sessionco = nil
		for _, co := range cookie {
			t.Logf("1-cookies: %s", co.Raw)
			if co.Name == "JSESSIONID" {
				sessionco = co
			}
		}
		if oldco != nil {
			if sessionco != nil {
				return nil, errors.New(url + " unexpected JSESSIONID cookies")
			}
			sessionco = oldco
		} else {
			if sessionco == nil {
				return nil, errors.New(url + " doesn't return JSESSIONID cookies")
			}
		}

		// Parse the response.
		var oldresult DemoResult
		json.Unmarshal(body, &oldresult)
		counter := 1
		if oldco != nil {
			// Read previous value and increase it.
			counter = oldresult.Counter
			counter++
		}
		t.Logf("%d - body: %s\n", counter, body)

		// Wait for the replication to take place... Probably something wrong there???
		time.Sleep(10 * time.Second)

		hostnames := make([]string, 0)
		hostnames = append(hostnames, oldresult.Hostname)
		for {
			// Do a another request.
			req, err := http.NewRequest("GET", url, nil)
			if err != nil {
				return nil, err
			}
			req.AddCookie(sessionco)
			client = &http.Client{}
			res, err = client.Do(req)
			if err != nil {
				return nil, err
			}
			newbody, err := ioutil.ReadAll(res.Body)
			res.Body.Close()
			if err != nil {
				return nil, err
			}
			if res.StatusCode != 200 {
				t.Logf("body: %s\n", newbody)
				return nil, errors.New(url + "second request returns: " + strconv.Itoa(res.StatusCode))
			}
			t.Logf("%d - body: %s\n", counter, newbody)
			cookie = res.Cookies()
			newsessionco := &http.Cookie{}
			newsessionco = nil
			for _, co := range cookie {
				t.Logf("2-cookies: %s", co.Raw)
				if co.Name == "JSESSIONID" {
					t.Logf("Found cookies: %s", co.Raw)
					newsessionco = co
				}
			}
			if newsessionco != nil {
				t.Logf("cookies new: %s old: %s", newsessionco.Raw, sessionco.Raw)
				return nil, errors.New(url + " Not sticky!!!")
			}

			// Check the counter in the body.
			var result DemoResult
			json.Unmarshal(newbody, &result)
			t.Logf("Demo counter: %d", result.Counter)
			if result.Counter != counter {
				return nil, errors.New(url + " NOTOK, counter should be " + strconv.Itoa(counter) + "... Not sticky!!!")
			}

			// And that pod name has changed...
			t.Logf("Demo POD: %s and %s", result.Hostname, strings.Join(hostnames, ","))
			found := false
			for _, hostname := range hostnames {
				t.Logf("Demo POD: %s and %s", result.Hostname, hostname)
				if hostname == result.Hostname {
					found = true
				}
			}
			if found {
				// We are on same pod... retry?...
				if webServer.Spec.Replicas == 1 {
					// Only one pod done...
					return sessionco, nil
				}
				t.Logf("%s NOTOK, on the same POD... Too sticky!!! retrying", url)
			} else {
				hostnames = append(hostnames, result.Hostname)
				if int32(len(hostnames)) == webServer.Spec.Replicas {
					return sessionco, nil
				}
			}
			counter++
			time.Sleep(10 * time.Second)
		}
	}
	return nil, nil
}


// Check that all the pods are ready
func arePodsReady(podList *corev1.PodList, size int32) bool {
	if int32(len(podList.Items)) != size {
		return false
	}
	for _, pod := range podList.Items {
		if !podv1.IsPodReady(&pod) {
			return false
		}
	}
	return true
}

// IsOperatorLocal returns true if the LOCAL_OPERATOR env var is set to true.
func IsOperatorLocal() bool {
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


// LabelsForWeb return a map of labels that are used for identification
//  of objects belonging to the particular WebServer instance
func LabelsForWeb(j *webserversv1alpha1.WebServer) map[string]string {
	labels := map[string]string{
		"deploymentConfig": j.Spec.ApplicationName,
		"WebServer":        j.Name,
	}
	// labels["app.kubernetes.io/name"] = j.Name
	// labels["app.kubernetes.io/managed-by"] = os.Getenv("LABEL_APP_MANAGED_BY")
	// labels["app.openshift.io/runtime"] = os.Getenv("LABEL_APP_RUNTIME")
	if j.Labels != nil {
		for labelKey, labelValue := range j.Labels {
			labels[labelKey] = labelValue
		}
	}
	return labels
}
