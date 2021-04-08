package framework

import (
	"context"
	goctx "context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	framework "github.com/operator-framework/operator-sdk/pkg/test"
	webserversv1alpha1 "github.com/web-servers/jws-operator/pkg/apis/webservers/v1alpha1"
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

// WebServerBasicTest runs basic operator tests
func WebServerBasicTest(t *testing.T, imageName string, testUri string) {
	ctx, f := webServerTestSetup(t)
	defer ctx.Cleanup()

	if err := webServerBasicServerScaleTest(t, f, ctx, imageName, testUri, 1, 2); err != nil {
		t.Fatal(err)
	}
}

// WebServermageStreamTest runs Image Stream operator tests
func WebServerImageStreamTest(t *testing.T, imageStreamName string, testUri string) {
	ctx, f := webServerTestSetup(t)
	defer ctx.Cleanup()

	if err := webServerImageStreamServerScaleTest(t, f, ctx, imageStreamName, testUri, 1, 2); err != nil {
		t.Fatal(err)
	}
}

func WebServerSourcesTest(t *testing.T, imageStreamName string, gitUrl string, testUri string) {
	WebServerSourcesUpscaleTest(t, imageStreamName, gitUrl, testUri, 1, 4)
	WebServerSourcesDownscaleTest(t, imageStreamName, gitUrl, testUri, 4, 1)
}

func WebServerSourcesUpscaleTest(t *testing.T, imageStreamName string, gitUrl string, testUri string, init int32, final int32) {
	ctx, f := webServerTestSetup(t)
	defer ctx.Cleanup()

	// scale up test.
	if err := webServerSourcesServerScaleTest(t, f, ctx, imageStreamName, gitUrl, testUri, init, final); err != nil {
		t.Fatal(err)
	}
}

func WebServerSourcesDownscaleTest(t *testing.T, imageStreamName string, gitUrl string, testUri string, init int32, final int32) {
	ctx, f := webServerTestSetup(t)
	defer ctx.Cleanup()

	// scale down test.
	if err := webServerSourcesServerScaleTest(t, f, ctx, imageStreamName, gitUrl, testUri, init, final); err != nil {
		t.Fatal(err)
	}
}

func webServerTestSetup(t *testing.T) (*framework.Context, *framework.Framework) {
	ctx := framework.NewContext(t)
	err := ctx.InitializeClusterResources(&framework.CleanupOptions{TestContext: ctx, Timeout: cleanupTimeout, RetryInterval: cleanupRetryInterval})
	if err != nil {
		defer ctx.Cleanup()
		t.Fatalf("Failed to initialize cluster resources: %v", err)
	}
	t.Log("Initialized cluster resources")
	namespace, err := ctx.GetOperatorNamespace()
	if err != nil {
		defer ctx.Cleanup()
		t.Fatalf("Failed to get namespace for testing context '%v': %v", ctx, err)
	}
	t.Logf("Testing in namespace %s", namespace)
	// get global framework variables
	f := framework.Global
	return ctx, f
}

func webServerBasicServerScaleTest(t *testing.T, f *framework.Framework, ctx *framework.Context, imageName string, testUri string, init int32, final int32) error {
	namespace, err := ctx.GetOperatorNamespace()
	if err != nil {
		return fmt.Errorf("could not get namespace: %v", err)
	}

	name := "example-webserver-" + unixEpoch()
	// create webServer custom resource
	webServer := MakeBasicWebServer(namespace, name, imageName, init)
	err = CreateAndWaitUntilReady(f, ctx, t, webServer)
	if err != nil {
		return err
	}

	t.Logf("Application %s is deployed with %d instance\n", name, init)

	// update the size to 2
	err = ScaleAndWaitUntilReady(f, t, webServer, name, namespace, final)
	if err != nil {
		return err
	}

	_, err = TestRouteWebServer(f, t, name, namespace, testUri, false, nil)
	if err != nil {
		return err
	}
	return nil
}

func webServerImageStreamServerScaleTest(t *testing.T, f *framework.Framework, ctx *framework.Context, imageStreamName string, testURI string, init int32, final int32) error {
	namespace, err := ctx.GetOperatorNamespace()
	if err != nil {
		return fmt.Errorf("could not get namespace: %v", err)
	}

	name := "example-webserver-" + unixEpoch()
	// create the webServer custom resource
	webServer := MakeImageStreamWebServer(namespace, name, imageStreamName, namespace, init)
	err = CreateAndWaitUntilReady(f, ctx, t, webServer)
	if err != nil {
		return err
	}

	t.Logf("Application %s is deployed with %d instance\n", name, init)

	// update the size to 2
	err = ScaleAndWaitUntilReady(f, t, webServer, name, namespace, final)
	if err != nil {
		return err
	}

	_, err = TestRouteWebServer(f, t, name, namespace, testURI, false, nil)
	if err != nil {
		return err
	}
	return nil
}

func webServerSourcesServerScaleTest(t *testing.T, f *framework.Framework, ctx *framework.Context, imageStreamName string, gitUrl string, testUri string, init int32, final int32) error {
	namespace, err := ctx.GetOperatorNamespace()
	if err != nil {
		return fmt.Errorf("could not get namespace: %v", err)
	}

	name := "example-webserver-" + unixEpoch()
	// create the webServer custom resource
	webServer := MakeSourcesWebServer(namespace, name, imageStreamName, namespace, gitUrl, init)
	err = CreateAndWaitUntilReady(f, ctx, t, webServer)
	if err != nil {
		return err
	}

	t.Logf("Application %s is deployed with %d instance\n", name, init)

	// Wait until the replication is started.
	time.Sleep(40 * time.Second)

	// Test it.
	co, err := TestRouteWebServer(f, t, name, namespace, testUri, true, nil)
	if err != nil {
		return err
	}
	if co == nil {
		return errors.New("Missing JSESSIONID cookie")
	}

	// update the size to final
	err = ScaleAndWaitUntilReady(f, t, webServer, name, namespace, final)
	if err != nil {
		return err
	}

	// Retest it
	if final > init {
		// Wait until the replication is started on the new pods.
		time.Sleep(40 * time.Second)
	}
	_, err = TestRouteWebServer(f, t, name, namespace, testUri, true, co)
	if err != nil {
		return err
	}
	return nil
}

func ScaleAndWaitUntilReady(f *framework.Framework, t *testing.T, server *webserversv1alpha1.WebServer, name string, namespace string, size int32) error {
	context := goctx.TODO()

	err := f.Client.Get(context, types.NamespacedName{Name: name, Namespace: namespace}, server)
	if err != nil {
		return err
	}
	server.Spec.Replicas = size
	err = f.Client.Update(context, server)
	if err != nil {
		return err
	}
	t.Logf("Updated application %s size to %d\n", name, server.Spec.Replicas)

	// check that the resource have been updated
	err = WaitUntilReady(f, t, server)
	if err != nil {
		return err
	}
	return nil
}

func unixEpoch() string {
	return strconv.FormatInt(time.Now().UnixNano(), 10)
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

// CreateAndWaitUntilReady creates a WebServer resource and wait until it is ready
func CreateAndWaitUntilReady(f *framework.Framework, ctx *framework.Context, t *testing.T, server *webserversv1alpha1.WebServer) error {
	// use Context's create helper to create the object and add a cleanup function for the new object
	err := f.Client.Create(goctx.TODO(), server, &framework.CleanupOptions{TestContext: ctx, Timeout: cleanupTimeout, RetryInterval: cleanupRetryInterval})
	if err != nil {
		return err
	}

	// removing finalizers explicitly otherwise the removal could hang
	ctx.AddCleanupFn(
		func() error {
			// Removing deployment for not putting finalizers back to the WebServer
			name := server.ObjectMeta.Name
			namespace := server.ObjectMeta.Namespace
			deployment, err := f.KubeClient.AppsV1().Deployments(namespace).Get("jws-operator", metav1.GetOptions{})
			if err == nil && deployment != nil {
				t.Logf("Cleaning deployment '%v'\n", deployment.Name)
				f.Client.Delete(goctx.TODO(), deployment)
			}
			// Cleaning finalizer
			return wait.Poll(retryInterval, timeout, func() (done bool, err error) {
				foundServer := &webserversv1alpha1.WebServer{}
				namespacedName := types.NamespacedName{Name: name, Namespace: namespace}
				if errPoll := f.Client.Get(context.TODO(), namespacedName, foundServer); errPoll != nil {
					if apierrors.IsNotFound(errPoll) {
						t.Logf("No WebServer object '%v' to remove the finalizer at. Probably all cleanly finished before.\n", name)
						return true, nil
					}
					t.Logf("Cannot obtain object of the WebServer '%v', cause: %v\n", name, errPoll)
					return false, nil
				}
				foundServer.SetFinalizers([]string{})
				if errPoll := f.Client.Update(context.TODO(), foundServer); errPoll != nil {
					t.Logf("Cannot update WebServer '%v' with empty finalizers array, cause: %v\n", name, errPoll)
					return false, nil
				}
				t.Logf("Finalizer definition succesfully removed from the WebServer '%v'\n", name)
				return true, nil
			})
		},
	)

	err = WaitUntilReady(f, t, server)
	if err != nil {
		return err
	}
	return nil
}

// WaitUntilReady waits until the number of pods matches the server spec size.
func WaitUntilReady(f *framework.Framework, t *testing.T, server *webserversv1alpha1.WebServer) error {
	name := server.ObjectMeta.Name
	size := server.Spec.Replicas

	t.Logf("Waiting until pods for %s are ready with size of %v", name, size)

	err := wait.Poll(retryInterval, timeout, func() (done bool, err error) {

		podList := &corev1.PodList{}
		listOpts := []client.ListOption{
			client.InNamespace(server.Namespace),
			client.MatchingLabels(LabelsForWeb(server)),
		}
		err = f.Client.List(context.TODO(), podList, listOpts...)
		if err != nil {
			if apierrors.IsNotFound(err) {
				t.Logf("List of pods %s not found", name)

				return false, nil
			}
			t.Logf("Got error when getting pod list %s: %s", name, err)
			return false, err
		}

		// Testing for Ready
		if ArePodsReady(podList, size) {
			return true, nil
		}

		t.Logf("Waiting for full availability of %s pod list (%d/%d)\n", name, podList.Items, size)
		return false, nil
	})
	if err != nil {
		return err
	}
	t.Logf("pods available (%d/%d)\n", size, size)

	return nil
}

// Check that all the pods are ready
func ArePodsReady(podList *corev1.PodList, size int32) bool {
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

// Test the route
func TestRouteWebServer(f *framework.Framework, t *testing.T, name string, namespace string, uri string, sticky bool, oldco *http.Cookie) (*http.Cookie, error) {

	context := goctx.TODO()

	webServer := &webserversv1alpha1.WebServer{}
	err := f.Client.Get(context, types.NamespacedName{Name: name, Namespace: namespace}, webServer)
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
