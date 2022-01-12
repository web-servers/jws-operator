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

	// "github.com/operator-framework/operator-sdk/pkg/test"
	webserversv1alpha1 "github.com/web-servers/jws-operator/api/v1alpha1"
	kbappsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	// metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubectl/pkg/util/podutils"
	// podv1 "k8s.io/kubernetes/pkg/api/v1/pod"
	routev1 "github.com/openshift/api/route/v1"
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

// WebServerApplicationImageBasicTest tests the deployment of an application image operator
func WebServerApplicationImageBasicTest(clt client.Client, ctx context.Context, t *testing.T, namespace string, name string, image string, testURI string) (err error) {

	webServer := makeApplicationImageWebServer(namespace, name, image, 1)
	if strings.HasPrefix(image, "registry.redhat.io") {
		// We need a pull secret for the image.
		webServer.Spec.WebImage.ImagePullSecret = "secretfortests"
	}

	// cleanup
	defer func() {
		clt.Delete(context.Background(), webServer)
		time.Sleep(time.Second * 5)
	}()

	return webServerBasicTest(clt, ctx, t, webServer, testURI)

}

// WebServerApplicationImageScaleTest tests the scaling of an application image operator
func WebServerApplicationImageScaleTest(clt client.Client, ctx context.Context, t *testing.T, namespace string, name string, image string, testURI string) (err error) {

	webServer := makeApplicationImageWebServer(namespace, name, image, 1)

	// cleanup
	defer func() {
		clt.Delete(context.Background(), webServer)
		time.Sleep(time.Second * 5)
	}()

	return webServerScaleTest(clt, ctx, t, webServer, testURI)
}

// WebServerApplicationImageUpdateTest test the application image update feature of an application image operator
func WebServerApplicationImageUpdateTest(clt client.Client, ctx context.Context, t *testing.T, namespace string, name string, image string, newImageName string, testURI string) (err error) {

	webServer := makeApplicationImageWebServer(namespace, name, image, 1)

	// cleanup
	defer func() {
		clt.Delete(context.Background(), webServer)
		time.Sleep(time.Second * 5)
	}()

	return webServerApplicationImageUpdateTest(clt, ctx, t, webServer, newImageName, testURI)
}

// WebServerApplicationImageSourcesBasicTest tests the deployment of an application image with sources
// we use testURI.war instead of ROOT.war and the servlet is /demo there
func WebServerApplicationImageSourcesBasicTest(clt client.Client, ctx context.Context, t *testing.T, namespace string, name string, image string, sourceRepositoryURL string, sourceRepositoryRef string, pushedimage string, pushsecret string, imagebuilder string, testURI string) (err error) {

	warname := testURI + ".war"
	webServer := makeApplicationImageSourcesWebServer(namespace, name, image, sourceRepositoryURL, sourceRepositoryRef, pushedimage, pushsecret, warname, imagebuilder, 1)

	// cleanup
	defer func() {
		clt.Delete(context.Background(), webServer)
		time.Sleep(time.Second * 5)
	}()

	return webServerBasicTest(clt, ctx, t, webServer, "/"+testURI+"/demo")
}

// WebServerApplicationImageSourcesScriptBasicTest tests the deployment of an application image with sources
// we use testURI.war instead of ROOT.war and the servlet is /demo there
func WebServerApplicationImageSourcesScriptBasicTest(clt client.Client, ctx context.Context, t *testing.T, namespace string, name string, image string, sourceRepositoryURL string, sourceRepositoryRef string, pushedimage string, pushsecret string, imagebuilder string, testURI string) (err error) {

	warname := testURI + ".war"
	webServer := makeApplicationImageSourcesWebServer(namespace, name, image, sourceRepositoryURL, sourceRepositoryRef, pushedimage, pushsecret, warname, imagebuilder, 1)
	// Add the custom script
	webServer.Spec.WebImage.WebApp.Builder.ApplicationBuildScript = `#!/bin/sh
cd tmp
echo "my html is ugly" > index.html
mkdir WEB-INF
echo "<web-app>" > WEB-INF/web.xml
echo "   <servlet>" >> WEB-INF/web.xml
echo "        <servlet-name>default</servlet-name>" >> WEB-INF/web.xml
echo "        <servlet-class>org.apache.catalina.servlets.DefaultServlet</servlet-class>" >> WEB-INF/web.xml
echo "    </servlet>" >> WEB-INF/web.xml
echo "   <servlet-mapping>" >> WEB-INF/web.xml
echo "        <servlet-name>default</servlet-name>" >> WEB-INF/web.xml
echo "        <url-pattern>/</url-pattern>" >> WEB-INF/web.xml
echo "    </servlet-mapping>" >> WEB-INF/web.xml
echo "</web-app>" >> WEB-INF/web.xml
jar cvf ROOT.war index.html WEB-INF/web.xml
mkdir /tmp/deployments
cp ROOT.war /tmp/deployments/${webAppWarFileName}
HOME=/tmp
STORAGE_DRIVER=vfs buildah bud -f /Dockerfile.JWS -t ${webAppWarImage} --authfile /auth/.dockerconfigjson --build-arg webAppSourceImage=${webAppSourceImage}
STORAGE_DRIVER=vfs buildah push --authfile /auth/.dockerconfigjson ${webAppWarImage}
`

	// cleanup
	defer func() {
		clt.Delete(context.Background(), webServer)
		time.Sleep(time.Second * 5)
	}()

	err = webServerBasicTest(clt, ctx, t, webServer, "/"+testURI+"/index.html")
	if err != nil {
		return err
	}
	err = webServerTestFor(clt, ctx, t, webServer, "/"+testURI+"/index.html", "my html is ugly")
	if err != nil {
		t.Logf("WebServerApplicationImageSourcesScriptBasicTest application %s webServerTestFor FAILED\n", name)
		return err
	}

	// Get the current webserver
	err = clt.Get(ctx, types.NamespacedName{Name: webServer.ObjectMeta.Name, Namespace: webServer.ObjectMeta.Namespace}, webServer)
	if err != nil {
		t.Logf("WebServerApplicationImageSourcesScriptBasicTest application %s get failed: %s\n", name, err)
		return err
	}

	// Change the custom script
	webServer.Spec.WebImage.WebApp.Builder.ApplicationBuildScript = `#!/bin/sh
cd tmp
echo "my html is _VERY_ ugly" > index.html
mkdir WEB-INF
echo "<web-app>" > WEB-INF/web.xml
echo "   <servlet>" >> WEB-INF/web.xml
echo "        <servlet-name>default</servlet-name>" >> WEB-INF/web.xml
echo "        <servlet-class>org.apache.catalina.servlets.DefaultServlet</servlet-class>" >> WEB-INF/web.xml
echo "    </servlet>" >> WEB-INF/web.xml
echo "   <servlet-mapping>" >> WEB-INF/web.xml
echo "        <servlet-name>default</servlet-name>" >> WEB-INF/web.xml
echo "        <url-pattern>/</url-pattern>" >> WEB-INF/web.xml
echo "    </servlet-mapping>" >> WEB-INF/web.xml
echo "</web-app>" >> WEB-INF/web.xml
jar cvf ROOT.war index.html WEB-INF/web.xml
mkdir /tmp/deployments
cp ROOT.war /tmp/deployments/${webAppWarFileName}
HOME=/tmp
STORAGE_DRIVER=vfs buildah bud -f /Dockerfile.JWS -t ${webAppWarImage} --authfile /auth/.dockerconfigjson --build-arg webAppSourceImage=${webAppSourceImage}
STORAGE_DRIVER=vfs buildah push --authfile /auth/.dockerconfigjson ${webAppWarImage}
`
	err = clt.Update(context.Background(), webServer)
	if err != nil {
		t.Logf("WebServerApplicationImageSourcesScriptBasicTest application %s update failed: %s\n", name, err)
		return err
	}
	t.Logf("WebServerApplicationImageSourcesScriptBasicTest application %s updated\n", name)

	return webServerTestFor(clt, ctx, t, webServer, "/"+testURI+"/index.html", "my html is _VERY_ ugly")

}

// WebServerApplicationImageSourcesBasicTest tests the scaling of an application image with sources
// we use testURI.war instead of ROOT.war and the servlet is /demo there
func WebServerApplicationImageSourcesScaleTest(clt client.Client, ctx context.Context, t *testing.T, namespace string, name string, image string, sourceRepositoryURL string, sourceRepositoryRef string, pushedimage string, pushsecret string, imagebuilder string, testURI string) (err error) {

	warname := testURI + ".war"
	webServer := makeApplicationImageSourcesWebServer(namespace, name, image, sourceRepositoryURL, sourceRepositoryRef, pushedimage, pushsecret, warname, imagebuilder, 1)

	// cleanup
	defer func() {
		clt.Delete(context.Background(), webServer)
		time.Sleep(time.Second * 5)
	}()

	return webServerScaleTest(clt, ctx, t, webServer, "/"+testURI+"/demo")
}

// WebServerImageStreamBasicTest tests the deployment of an Image Stream operator
func WebServerImageStreamBasicTest(clt client.Client, ctx context.Context, t *testing.T, namespace string, name string, imageStreamName string, testURI string) (err error) {

	webServer := makeImageStreamWebServer(namespace, name, imageStreamName, namespace, 1)
	t.Logf("WebServerImageStreamBasicTest application %s number of replicas to %d\n", name, webServer.Spec.Replicas)
	t.Logf("WebServerImageStreamBasicTest application %s imagestream %s\n", name, webServer.Spec.WebImageStream.ImageStreamName)

	// cleanup
	defer func() {
		clt.Delete(context.Background(), webServer)
		time.Sleep(time.Second * 5)
	}()

	return webServerBasicTest(clt, ctx, t, webServer, testURI)
}

// WebServerImageStreamScaleTest tests the scaling of an Image Stream operator
func WebServerImageStreamScaleTest(clt client.Client, ctx context.Context, t *testing.T, namespace string, name string, imageStreamName string, testURI string) (err error) {

	webServer := makeImageStreamWebServer(namespace, name, imageStreamName, namespace, 1)

	// cleanup
	defer func() {
		clt.Delete(context.Background(), webServer)
		time.Sleep(time.Second * 5)
	}()

	return webServerScaleTest(clt, ctx, t, webServer, testURI)
}

// WebServerImageStreamSourcesBasicTest tests the deployment of an Image Stream operator with sources
func WebServerImageStreamSourcesBasicTest(clt client.Client, ctx context.Context, t *testing.T, namespace string, name string, imageStreamName string, sourceRepositoryURL string, sourceRepositoryRef string, testURI string) (err error) {

	webServer := makeImageStreamSourcesWebServer(namespace, name, imageStreamName, namespace, sourceRepositoryURL, sourceRepositoryRef, 1)

	// cleanup
	defer func() {
		clt.Delete(context.Background(), webServer)
		time.Sleep(time.Second * 5)
	}()

	return webServerBasicTest(clt, ctx, t, webServer, testURI)
}

// WebServerImageStreamSourcesScaleTest tests the scaling of an Image Stream operator with sources
func WebServerImageStreamSourcesScaleTest(clt client.Client, ctx context.Context, t *testing.T, namespace string, name string, imageStreamName string, sourceRepositoryURL string, sourceRepositoryRef string, testURI string) (err error) {

	webServer := makeImageStreamSourcesWebServer(namespace, name, imageStreamName, namespace, sourceRepositoryURL, sourceRepositoryRef, 1)

	// cleanup
	defer func() {
		clt.Delete(context.Background(), webServer)
		time.Sleep(time.Second * 5)
	}()

	return webServerScaleTest(clt, ctx, t, webServer, testURI)
}

// webServerBasicTest tests if the deployed pods of the operator are working
func webServerBasicTest(clt client.Client, ctx context.Context, t *testing.T, webServer *webserversv1alpha1.WebServer, testURI string) (err error) {

	err = deployWebServer(clt, ctx, t, webServer)
	if err != nil {
		return err
	}

	cookie, err := webServerRouteTest(clt, ctx, t, webServer, testURI, false, nil)

	if cookie == nil {
		// return errors.New("The cookie was nil!")
	}
	return err

}

// webServerScaleTest tests if the deployed pods of the operator are working properly after scaling
func webServerScaleTest(clt client.Client, ctx context.Context, t *testing.T, webServer *webserversv1alpha1.WebServer, testURI string) (err error) {

	err = deployWebServer(clt, ctx, t, webServer)
	if err != nil {
		return err
	}

	// scale up test.
	webServerScale(clt, ctx, t, webServer, testURI, 4)

	cookie, err := webServerRouteTest(clt, ctx, t, webServer, testURI, false, nil)
	if err != nil {
		return err
	}
	if cookie == nil {
	}

	// scale down test.
	webServerScale(clt, ctx, t, webServer, testURI, 1)

	cookie, err = webServerRouteTest(clt, ctx, t, webServer, testURI, false, nil)
	if err != nil {
		return err
	}
	if cookie == nil {
	}
	return nil
}

// webServerScale changes the replica number of the WebServer resource
func webServerScale(clt client.Client, ctx context.Context, t *testing.T, webServer *webserversv1alpha1.WebServer, testURI string, newReplicasValue int32) {

	err := clt.Get(ctx, types.NamespacedName{Name: webServer.ObjectMeta.Name, Namespace: webServer.ObjectMeta.Namespace}, webServer)
	if err != nil {
		t.Fatal(err)
	}

	webServer.Spec.Replicas = newReplicasValue

	updateWebServer(clt, ctx, t, webServer, name, namespace)

	t.Logf("Updated application %s number of replicas to %d\n", name, webServer.Spec.Replicas)

}

// webServerApplicationImageUpdateTest tests if the deployed pods of the operator are working properly after an application image update
func webServerApplicationImageUpdateTest(clt client.Client, ctx context.Context, t *testing.T, webServer *webserversv1alpha1.WebServer, newImageName string, testURI string) (err error) {

	deployWebServer(clt, ctx, t, webServer)

	webServerApplicationImageUpdate(clt, ctx, t, webServer, newImageName, testURI)

	foundDeployment := &kbappsv1.Deployment{}
	err = clt.Get(ctx, types.NamespacedName{Name: webServer.ObjectMeta.Name, Namespace: webServer.ObjectMeta.Namespace}, foundDeployment)
	if err != nil {
		t.Errorf("Failed to get Deployment\n")
		return err
	}

	foundImage := foundDeployment.Spec.Template.Spec.Containers[0].Image
	if foundImage != newImageName {
		/* TODO: The test needs to be more cleaver here... */
		t.Errorf("Found %s as application image; wanted %s", foundImage, newImageName)
		return err
		/* */
	}
	return nil

}

// webServerApplicationImageUpdate changes the application image of the WebServer resource
func webServerApplicationImageUpdate(clt client.Client, ctx context.Context, t *testing.T, webServer *webserversv1alpha1.WebServer, newImageName string, testURI string) {

	// WebServer resource needs to be refreshed before being updated
	// to avoid "the object has been modified" errors
	err := clt.Get(ctx, types.NamespacedName{Name: webServer.ObjectMeta.Name, Namespace: webServer.ObjectMeta.Namespace}, webServer)
	if err != nil {
		t.Fatal(err)
	}

	webServer.Spec.WebImage.ApplicationImage = newImageName

	updateWebServer(clt, ctx, t, webServer, name, namespace)

	t.Logf("Updated application image of WebServer %s to %s\n", name, newImageName)

}

// updateWebServer updates the WebServer resource and waits until the new deployment is ready
func updateWebServer(clt client.Client, ctx context.Context, t *testing.T, webServer *webserversv1alpha1.WebServer, name string, namespace string) {

	err := clt.Update(ctx, webServer)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("WebServer %s updated\n", name)

	// Waits until the pods are deployed
	waitUntilReady(clt, ctx, t, webServer)

}

// deployWebServer deploys a WebServer resource and waits until the pods are online
func deployWebServer(clt client.Client, ctx context.Context, t *testing.T, webServer *webserversv1alpha1.WebServer) (err error) {

	// Create the webserver
	t.Logf("Create webServer\n")
	err = clt.Create(ctx, webServer)
	if err != nil {
		t.Logf("Create webServer failed\n")
		t.Fatal(err)
		return err
	}

	// Wait for it to be ready
	err = waitUntilReady(clt, ctx, t, webServer)

	t.Logf("Application %s is deployed ", name)

	return err

}

// waitUntilReady waits until the number of pods matches the WebServer Spec replica number.
func WaitUntilReady(clt client.Client, ctx context.Context, t *testing.T, webServer *webserversv1alpha1.WebServer) (err error) {
	return waitUntilReady(clt, ctx, t, webServer)
}

func waitUntilReady(clt client.Client, ctx context.Context, t *testing.T, webServer *webserversv1alpha1.WebServer) (err error) {
	name := webServer.ObjectMeta.Name
	replicas := webServer.Spec.Replicas

	t.Logf("Waiting until %[1]d/%[1]d pods for %s are ready", replicas, name)

	err = wait.Poll(retryInterval, timeout, func() (done bool, err error) {

		podList := &corev1.PodList{}
		listOpts := []client.ListOption{
			client.InNamespace(webServer.Namespace),
			client.MatchingLabels(generateLabelsForWebServer(webServer)),
		}
		err = clt.List(ctx, podList, listOpts...)
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
			t.Logf("Waiting for full availability Done")
			return true, nil
		}

		t.Logf("Waiting for full availability of %s pod list (%d/%d)\n", name, int32(len(podList.Items)), replicas)
		return false, nil
	})
	if err != nil {
		t.Fatal(err)
		return err
	}
	t.Logf("(%[1]d/%[1]d) pods are ready \n", replicas)
	return nil
}

// webServerRouteTest tests the Route created for the operator pods
func webServerRouteTest(clt client.Client, ctx context.Context, t *testing.T, webServer *webserversv1alpha1.WebServer, URI string, sticky bool, oldCookie *http.Cookie) (sessionCookie *http.Cookie, err error) {

	curwebServer := &webserversv1alpha1.WebServer{}
	err = clt.Get(ctx, types.NamespacedName{Name: webServer.ObjectMeta.Name, Namespace: webServer.ObjectMeta.Namespace}, curwebServer)
	if err != nil {
		return nil, errors.New("Can't read webserver!")
	}
	URL := ""
	if os.Getenv("NODENAME") != "" {
		// here we need to use nodePort
		balancer := &corev1.Service{}
		err = clt.Get(ctx, types.NamespacedName{Name: webServer.Spec.ApplicationName + "-lb", Namespace: webServer.ObjectMeta.Namespace}, balancer)
		if err != nil {
			t.Logf("WebServer.Status.Hosts error!!!")
			return nil, errors.New("Can't read balancer!")
		}
		port := balancer.Spec.Ports[0].NodePort
		URL = "http://" + os.Getenv("NODENAME") + ":" + strconv.Itoa(int(port)) + URI
	} else {
		for i := 1; i < 20; i++ {
			err = clt.Get(ctx, types.NamespacedName{Name: webServer.ObjectMeta.Name, Namespace: webServer.ObjectMeta.Namespace}, curwebServer)
			if err != nil {
				t.Logf("WebServer.Status.Hosts error!!!")
				time.Sleep(10 * time.Second)
				continue
			}
			if len(curwebServer.Status.Hosts) == 0 {
				t.Logf("WebServer.Status.Hosts is empty. Attempt %d/20\n", i)
				time.Sleep(20 * time.Second)
			} else {
				break
			}
		}
		if err != nil {
			return nil, err
		}

		if len(curwebServer.Status.Hosts) == 0 {
			t.Logf("WebServer.Status.Hosts is empty\n")
			return nil, errors.New("Route is empty!")
		}
		t.Logf("Route:  (%s)\n", curwebServer.Status.Hosts)
		URL = "http://" + curwebServer.Status.Hosts[0] + URI
	}

	// Wait a little to avoid 503 codes.
	time.Sleep(10 * time.Second)

	req, err := http.NewRequest("GET", URL, nil)
	if err != nil {
		t.Logf("GET: (%s) FAILED\n", URL)
		return nil, err
	}
	if oldCookie != nil {
		req.AddCookie(oldCookie)
		t.Logf("GET: (%s) cookie: (%s)\n", URL, oldCookie.Raw)
	} else {
		t.Logf("GET:  (%s)\n", URL)
	}
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		// Probably the  dns information needs more time.
		t.Logf("GET: (%s) FAILED\n", URL)
		for i := 1; i < 60; i++ {
			time.Sleep(10 * time.Second)
			res, err = client.Do(req)
			if err == nil {
				break
			}
		}
		if err != nil {
			t.Logf("GET: (%s) FAILED 60 times\n", URL)
			return nil, err
		}
	}
	body, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		t.Logf("GET: (%s) FAILED no Body\n", URL)
		return nil, err
	}
	if res.StatusCode != 200 {
		t.Logf("FAIL status: %d body: %s\n", res.StatusCode, body)
		return nil, errors.New(URL + " returns: " + strconv.Itoa(res.StatusCode))
	}
	if sticky {
		// Do stickyness test.

		// read the SESSIONID cookie
		cookies := res.Cookies()
		sessionCookie = nil
		for _, cookie := range cookies {
			t.Logf("1-cookies: %s", cookie.Raw)
			if cookie.Name == "JSESSIONID" {
				sessionCookie = cookie
			}
		}
		if oldCookie != nil {
			if sessionCookie != nil {
				return nil, errors.New(URL + " returns unexpected JSESSIONID cookies")
			}
			sessionCookie = oldCookie
		} else {
			if sessionCookie == nil {
				return nil, errors.New(URL + " doesn't return JSESSIONID cookies")
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
				t.Logf("GET: (%s) FAILED\n", URL)
				return nil, err
			}
			req.AddCookie(sessionCookie)
			client = &http.Client{}
			res, err = client.Do(req)
			if err != nil {
				t.Logf("GET: (%s) FAILED\n", URL)
				return nil, err
			}
			newBody, err := ioutil.ReadAll(res.Body)
			res.Body.Close()
			if err != nil {
				t.Logf("GET: (%s) FAILED no Body\n", URL)
				return nil, err
			}
			if res.StatusCode != 200 {
				t.Logf("body: %s\n", newBody)
				return nil, errors.New(URL + " second request returns: " + strconv.Itoa(res.StatusCode))
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
				return nil, errors.New(URL + " Not sticky!!!")
			}

			// Check the counter in the body.
			var result DemoResult
			json.Unmarshal(newBody, &result)
			t.Logf("Demo counter: %d", result.Counter)
			if result.Counter != counter {
				return nil, errors.New(URL + " NOTOK, counter should be " + strconv.Itoa(counter) + "... Not sticky!!!")
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
				if curwebServer.Spec.Replicas == 1 {
					// Only one pod done...
					return sessionCookie, nil
				}
				t.Logf("%s NOTOK, on the same pod... Too sticky!!! retrying", URL)
			} else {
				hostnames = append(hostnames, result.Hostname)
				if int32(len(hostnames)) == curwebServer.Spec.Replicas {
					return sessionCookie, nil
				}
			}
			counter++
			time.Sleep(10 * time.Second)
		}
	}
	t.Logf("GET: (%s) Done\n", URL)
	return nil, nil
}

// arePodsReady checks that all the pods are ready
func arePodsReady(podList *corev1.PodList, replicas int32) bool {
	if int32(len(podList.Items)) != replicas {
		return false
	}
	for _, pod := range podList.Items {
		if !podutils.IsPodReady(&pod) {
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

// Pseudo random string
func UnixEpoch() string {
	return strconv.FormatInt(time.Now().UnixNano(), 10)
}

// Check for openshift looking for routes
func WebServerHaveRoutes(clt client.Client, ctx context.Context, t *testing.T) bool {
	routeList := &routev1.RouteList{}
	listOpts := []client.ListOption{}
	err := clt.List(ctx, routeList, listOpts...)
	if err != nil {
		t.Logf("webServerHaveRoutes error: %s", err)
		return false
	}
	t.Logf("webServerHaveRoutes found %d routes", int32(len(routeList.Items)))
	return true
}

// webServerTestFor tests the pod for a content in the URI
func webServerTestFor(clt client.Client, ctx context.Context, t *testing.T, webServer *webserversv1alpha1.WebServer, URI string, content string) (err error) {

	curwebServer := &webserversv1alpha1.WebServer{}
	err = clt.Get(ctx, types.NamespacedName{Name: webServer.ObjectMeta.Name, Namespace: webServer.ObjectMeta.Namespace}, curwebServer)
	if err != nil {
		return errors.New("Can't read webserver!")
	}
	URL := ""
	if os.Getenv("NODENAME") != "" {
		// here we need to use nodePort
		balancer := &corev1.Service{}
		err = clt.Get(ctx, types.NamespacedName{Name: webServer.Spec.ApplicationName + "-lb", Namespace: webServer.ObjectMeta.Namespace}, balancer)
		if err != nil {
			t.Logf("WebServer.Status.Hosts error!!!")
			return errors.New("Can't read balancer!")
		}
		port := balancer.Spec.Ports[0].NodePort
		URL = "http://" + os.Getenv("NODENAME") + ":" + strconv.Itoa(int(port)) + URI
	} else {
		for i := 1; i < 10; i++ {
			err = clt.Get(ctx, types.NamespacedName{Name: webServer.ObjectMeta.Name, Namespace: webServer.ObjectMeta.Namespace}, curwebServer)
			if err != nil {
				t.Logf("WebServer.Status.Hosts error!!!")
				time.Sleep(10 * time.Second)
				continue
			}
			if len(curwebServer.Status.Hosts) == 0 {
				t.Logf("WebServer.Status.Hosts is empty. Attempt %d/10\n", i)
				time.Sleep(10 * time.Second)
			} else {
				break
			}
		}
		if err != nil {
			return err
		}

		if len(curwebServer.Status.Hosts) == 0 {
			t.Logf("WebServer.Status.Hosts is empty\n")
			return errors.New("Route is empty!")
		}
		t.Logf("Route:  (%s)\n", curwebServer.Status.Hosts)
		URL = "http://" + curwebServer.Status.Hosts[0] + URI
	}

	// Wait a little to avoid 503 codes.
	time.Sleep(10 * time.Second)

	req, err := http.NewRequest("GET", URL, nil)
	if err != nil {
		t.Logf("GET: (%s) FAILED\n", URL)
		return err
	}
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		// Probably the  dns information needs more time.
		t.Logf("GET: (%s) FAILED\n", URL)
		for i := 1; i < 20; i++ {
			time.Sleep(60 * time.Second)
			res, err = client.Do(req)
			if err == nil {
				break
			}
		}
		if err != nil {
			t.Logf("GET: (%s) FAILED 10 times\n", URL)
			return err
		}
	}
	body, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		t.Logf("GET: (%s) FAILED no Body\n", URL)
		return err
	}
	if res.StatusCode != 200 {
		t.Logf("FAIL status: %d body: %s\n", res.StatusCode, body)
		return errors.New(URL + " returns: " + strconv.Itoa(res.StatusCode))
	}
	t.Logf("GET: (%s) Done\n", URL)
	t.Logf("GET: body (%s) Done\n", body)
	if strings.Contains(string(body), content) {
		t.Logf("GET: body (%s) Done\n", strconv.FormatBool(strings.Contains(string(body), content)))
		return nil
	} else {
		t.Logf("GET: body (%s) wrong content\n", strconv.FormatBool(strings.Contains(string(body), content)))
		// we retry until the webserver gets updated
		for i := 1; i < 20; i++ {
			time.Sleep(60 * time.Second)
			res, err = client.Do(req)
			if err != nil {
				t.Logf("GET: (%s) FAILED: %s try: %d\n", URL, err, i)
				continue
				// return errors.New(URL + " does not contain" + content)
			}
			body, err := ioutil.ReadAll(res.Body)
			res.Body.Close()
			if err != nil {
				t.Logf("GET: (%s) FAILED no Body\n", URL)
				return errors.New(URL + " does not contain" + content)
			}
			if res.StatusCode != 200 {
				t.Logf("FAIL status: %d body: %s\n", res.StatusCode, body)
				return errors.New(URL + " does not contain" + content)
			}
			if strings.Contains(string(body), content) {
				t.Logf("GET: body (%s) Done\n", strconv.FormatBool(strings.Contains(string(body), content)))
				return nil
			}
			t.Logf("GET: body (%s:%s) wrong content try: %d\n", strconv.FormatBool(strings.Contains(string(body), content)), body, i)
		}
		t.Logf("GET: (%s) FAILED 10 times\n", URL)
		return errors.New(URL + " does not contain" + content)
	}
}
