package utils

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	v2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	// "github.com/operator-framework/operator-sdk/pkg/test"
	routev1 "github.com/openshift/api/route/v1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	webserversv1alpha1 "github.com/web-servers/jws-operator/api/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	// metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubectl/pkg/util/podutils"

	// podv1 "k8s.io/kubernetes/pkg/api/v1/pod"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

/*
Result for the demo webapp

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

func PrometheusTest(clt client.Client, ctx context.Context, t *testing.T, namespace string, webServer *webserversv1alpha1.WebServer, testURI string, domain string) (err error) {
	// create a http client
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	schemeBuilder := runtime.NewSchemeBuilder(
		monitoringv1.AddToScheme,
	)

	err = schemeBuilder.AddToScheme(scheme.Scheme)
	if err != nil {
		t.Logf("schemeBuilder.AddToScheme failed: %s\n", err)
		return err
	}

	// get the token
	s := ""
	url := "http://localhost:9090"
	token, err := exec.Command("oc", "whoami", "-t").Output()
	if err != nil {
		t.Logf("oc whoami -t failed Error: %s", err)
		token, err = exec.Command("ibmcloud", "iam", "oauth-tokens").Output()
		if err != nil {
			t.Errorf("ibmcloud iam oauth-tokens Failed Error: %s", err)
			return err
		}
		s = string(token)
		t.Logf("token: %s\n", s)
		stoken := strings.Split(s, " ")
		s = stoken[4]
		s = strings.Replace(s, "\n", "", -1)
		t.Logf("token: *%s*\n", s)
	} else {
		s = string(token)
		s = strings.Replace(s, "\n", "", -1)
		url = "https://thanos-querier-openshift-monitoring." + domain
	}

	unixTime := time.Now().Unix()
	var unixTimeStart int64 = unixTime
	var unixTimeEnd int64 = unixTime + 3600

	cookie, err := WebServerRouteTest(clt, ctx, t, webServer, testURI, false, nil, false)
	if err != nil {
		t.Logf("PrometheusTest: WebServerRouteTest failed")
		return err
	}
	_ = cookie
	time.Sleep(time.Second * 120) //waiting for some queries from healh check...

	// create a http request to Prometheus server
	req, err := http.NewRequest("GET", url+"/api/v1/query_range?query=tomcat_bytesreceived_total&start="+strconv.FormatInt(unixTimeStart, 10)+"&end="+strconv.FormatInt(unixTimeEnd, 10)+"&step=14", nil)
	if err != nil {
		//		t.Logf("Failed using: " + url + "/api/v1/query_range?query=tomcat_bytesreceived_total&start=" + strconv.FormatInt(unixTimeStart, 10) + "&end=" + strconv.FormatInt(unixTimeEnd, 10) + "&step=14")
		t.Fatal(err)
	}
	if strings.HasPrefix(url, "https://") {
		req.Header.Set("Authorization", "Bearer "+s)
		req.Header.Set("Accept", "application/json")
	} else {
		podname := GetThanos(clt, ctx, t)
		//		t.Logf("using pod: " + podname)
		cmd := exec.Command("oc", "port-forward", "-n", "openshift-monitoring", "pod/"+podname, "9090")
		stdout, err := cmd.StdoutPipe()
		cmd.Stderr = cmd.Stdout
		err = cmd.Start()
		if err != nil {
			t.Errorf("oc port-forward -n openshift-monitoring pod/%s 9090 failed Error: %s", podname, err)
		}
		go func() {
			err = cmd.Wait()
			t.Errorf("oc port-forward -n openshift-monitoring pod/%s 9090 failed Error: %s", podname, err)
		}()
		tmp := make([]byte, 1024)
		_, err = stdout.Read(tmp)
		//		t.Logf(string(tmp))
	}

	// curl -k \
	//  -H "Authorization: Bearer $TOKEN" \
	// -H 'Accept: application/json' \
	// "https://thanos-querier-openshift-monitoring.apps.jws-qe-afll.dynamic.xpaas/api/v1/query?query=tomcat_errorcount_total"

	// send the request
	t.Logf("GET: host *%s* URI *%s*\n", req.Host, req.URL.RequestURI())
	res, err := client.Do(req)
	for i := 0; i < 60; i++ {
		if err == nil {
			break
		}
		t.Errorf("request to %s failed Error: %s", url, err)
		time.Sleep(1000 * time.Millisecond)
	}
	if err != nil {
		t.Errorf("request to %s failed Error: %s", url, err)
		t.Fatal(err)
	}

	// check the response status code
	if res.StatusCode != http.StatusOK {
		t.Errorf("unexpected status code: %d", res.StatusCode)
		t.Errorf("unexpected from: %s", url)
		t.Errorf("unexpected token: %s", s)
	}

	// read the response body
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}

	// print the response body
	t.Logf("Response body: %s", string(body))

	if strings.Contains(string(body), webServer.Name) && strings.Contains(string(body), "tomcat_bytesreceived_total") {
		t.Logf("Response body contains the expected message")
		return nil
	} else {
		//		t.Logf("Failed using: " + url + "/api/v1/query_range?query=tomcat_bytesreceived_total&start=" + strconv.FormatInt(unixTimeStart, 10) + "&end=" + strconv.FormatInt(unixTimeEnd, 10) + "&step=14")
		t.Fatal("Response body does not contain expected message")
	}

	return errors.New("Response body does not contain expected message")
}

func GetThanos(clt client.Client, ctx context.Context, t *testing.T) (thanos string) {
	podList := &corev1.PodList{}
	labels := map[string]string{
		"app.kubernetes.io/name": "thanos-query",
	}

	listOpts := []client.ListOption{
		client.InNamespace("openshift-monitoring"),
		client.MatchingLabels(labels),
	}
	err := clt.List(ctx, podList, listOpts...)
	if err != nil {
		t.Logf("List pods failed: %s", err)
		return ""
	}
	return podList.Items[0].ObjectMeta.Name
}

func AutoScalingTest(clt client.Client, ctx context.Context, t *testing.T, webServer *webserversv1alpha1.WebServer, testURI string, hpa *v2.HorizontalPodAutoscaler) (err error) {

	curwebServer := &webserversv1alpha1.WebServer{}
	err = clt.Get(ctx, types.NamespacedName{Name: webServer.ObjectMeta.Name, Namespace: webServer.ObjectMeta.Namespace}, curwebServer)
	if err != nil {
		return errors.New("can't read webserver")
	}
	URL := ""
	if os.Getenv("NODENAME") != "" {
		// here we need to use nodePort
		balancer := &corev1.Service{}
		err = clt.Get(ctx, types.NamespacedName{Name: webServer.Spec.ApplicationName + "-lb", Namespace: webServer.ObjectMeta.Namespace}, balancer)
		if err != nil {
			t.Logf("WebServer.Status.Hosts error!!!")
			return errors.New("can't read balancer")
		}
		port := balancer.Spec.Ports[0].NodePort

		URL = "http://" + os.Getenv("NODENAME") + ":" + strconv.Itoa(int(port)) + testURI

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
			return err
		}

		if len(curwebServer.Status.Hosts) == 0 {
			t.Logf("WebServer.Status.Hosts is empty\n")
			return errors.New("route is empty")
		}
		t.Logf("Route:  (%s)\n", curwebServer.Status.Hosts)

		URL = "http://" + curwebServer.Status.Hosts[0] + testURI

	}

	// Wait a little to let the hpa scale down the pod
	Eventually(func() bool {
		err = clt.Get(ctx, types.NamespacedName{Name: webServer.ObjectMeta.Name, Namespace: webServer.ObjectMeta.Namespace}, curwebServer)
		if err != nil {
			t.Fatalf("can't read webserver")
			return false
		}

		if int32(curwebServer.Status.Replicas) == int32(4) {
			t.Logf("Replicas:  (%d:4)\n", int32(curwebServer.Status.Replicas))
			return false
		} else {
			return true
		}

	}, time.Second*420, time.Second*30).Should(BeTrue())

	err = clt.Get(ctx, types.NamespacedName{Name: webServer.ObjectMeta.Name, Namespace: webServer.ObjectMeta.Namespace}, curwebServer)
	if err != nil {
		return errors.New("can't read webserver")
	}

	if int32(curwebServer.Status.Replicas) == int32(4) {
		return errors.New("didn't scaled down")
	}

	Eventually(func() bool {

		for i := 0; i < 100; i++ {
			go getRequest(URL)
		}

		if err != nil {
			return false
		}

		err = clt.Get(ctx, types.NamespacedName{Name: webServer.ObjectMeta.Name, Namespace: webServer.ObjectMeta.Namespace}, curwebServer)
		if err != nil {
			t.Fatalf("can't read webserver")
			return false
		}

		if int32(curwebServer.Status.Replicas) > int32(1) {
			return true
		}

		t.Logf("Replicas:  (%d>1)\n", int32(curwebServer.Status.Replicas))
		return false

	}, time.Second*250, time.Millisecond*10).Should(BeTrue())

	err = clt.Get(ctx, types.NamespacedName{Name: webServer.ObjectMeta.Name, Namespace: webServer.ObjectMeta.Namespace}, curwebServer)
	if err != nil {
		return errors.New("can't read webserver")
	}

	if int32(curwebServer.Status.Replicas) < int32(2) {
		t.Logf("Replicas:  (%d<2)\n", int32(curwebServer.Status.Replicas))
		return errors.New("didn't scaled up")
	} else {
		return nil
	}

}

func getRequest(URL string) (interface{}, error) {
	cmd := exec.Command("curl", URL)
	stdout, err := cmd.Output()

	return stdout, err
}

// WebServerScale changes the replica number of the WebServer resource
func WebServerScale(clt client.Client, ctx context.Context, t *testing.T, webServer *webserversv1alpha1.WebServer, testURI string, newReplicasValue int32) {

	err := clt.Get(ctx, types.NamespacedName{Name: webServer.ObjectMeta.Name, Namespace: webServer.ObjectMeta.Namespace}, webServer)
	if err != nil {
		t.Fatal(err)
	}

	webServer.Spec.Replicas = newReplicasValue

	updateWebServer(clt, ctx, t, webServer, name, namespace)

	t.Logf("Updated application %s number of replicas to %d\n", name, webServer.Spec.Replicas)

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

// WebServerRouteTest tests the Route created for the operator pods
func WebServerRouteTest(clt client.Client, ctx context.Context, t *testing.T, webServer *webserversv1alpha1.WebServer, URI string, sticky bool, oldCookie *http.Cookie, isSecure bool) (sessionCookie *http.Cookie, err error) {

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
		if isSecure {
			URL = "https://" + os.Getenv("NODENAME") + ":" + strconv.Itoa(int(port)) + URI
		} else {
			URL = "http://" + os.Getenv("NODENAME") + ":" + strconv.Itoa(int(port)) + URI
		}
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
		t.Logf("Route:  (%s)\n", webServer.Status.Hosts)
		t.Logf("RouteHostName:  (%s)\n", webServer.Spec.TLSConfig.RouteHostname)
		t.Logf("TLSSecret:  (%s)\n", webServer.Spec.TLSConfig.TLSSecret)

		t.Logf("Route:  (%s)\n", curwebServer.Status.Hosts)
		t.Logf("RouteHostName:  (%s)\n", curwebServer.Spec.TLSConfig.RouteHostname)
		t.Logf("TLSSecret:  (%s)\n", curwebServer.Spec.TLSConfig.TLSSecret)
		if isSecure {
			if len(curwebServer.Spec.TLSConfig.RouteHostname) <= 4 {
				URL = "https://" + curwebServer.Status.Hosts[0] + URI
			} else {
				// We have something like tls:hostname
				URL = "https://" + curwebServer.Spec.TLSConfig.RouteHostname[4:] + URI
			}
		} else {
			URL = "http://" + curwebServer.Status.Hosts[0] + URI
		}
	}

	// Wait a little to avoid 503 codes.
	time.Sleep(60 * time.Second)

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
	var client = &http.Client{}
	if isSecure { //disable security check for the client to overcome issues with the certificate
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		client = &http.Client{Transport: tr}
	}
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

// generateLabelsForWebServer return a map of labels that are used for identification
//
//	of objects belonging to the particular WebServer instance
func generateLabelsForWebServer(webServer *webserversv1alpha1.WebServer) map[string]string {
	labels := map[string]string{
		"deployment": webServer.Spec.ApplicationName,
		"WebServer":  webServer.Name,
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

func GetHost(route *routev1.Route) string {
	if len(route.Status.Ingress) > 0 {
		host := route.Status.Ingress[0].Host
		return host
	}
	return ""
}

// WebServerTestFor tests the pod for a content in the URI
func WebServerTestFor(clt client.Client, ctx context.Context, t *testing.T, webServer *webserversv1alpha1.WebServer, URI string, content string) (err error) {

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
				if res.StatusCode == 503 {
					continue
				}
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
