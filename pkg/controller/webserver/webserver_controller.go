package webserver

import (
	"context"
	"reflect"
	"sort"
	"strconv"
	"time"

	webserversv1alpha1 "github.com/web-servers/jws-operator/pkg/apis/webservers/v1alpha1"

	appsv1 "github.com/openshift/api/apps/v1"
	buildv1 "github.com/openshift/api/build/v1"
	routev1 "github.com/openshift/api/route/v1"
	kbappsv1 "k8s.io/api/apps/v1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var (
	log = logf.Log.WithName("webserver_controller")
)

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new WebServer Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileWebServer{client: mgr.GetClient(), scheme: mgr.GetScheme(), isOpenShift: isOpenShift(mgr.GetConfig()), useKUBEPing: true}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("webserver-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource WebServer
	err = c.Watch(&source.Kind{Type: &webserversv1alpha1.WebServer{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource Pods and requeue the owner WebServer
	enqueueRequestForOwner := handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &webserversv1alpha1.WebServer{},
	}
	for _, obj := range []runtime.Object{&kbappsv1.Deployment{}, &corev1.Service{}} {
		if err = c.Watch(&source.Kind{Type: obj}, &enqueueRequestForOwner); err != nil {
			return err
		}
	}
	if isOpenShift(mgr.GetConfig()) {
		for _, obj := range []runtime.Object{&appsv1.DeploymentConfig{}, &routev1.Route{}} {
			if err = c.Watch(&source.Kind{Type: obj}, &enqueueRequestForOwner); err != nil {
				return err
			}
		}
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileWebServer{}

// ReconcileWebServer reconciles a WebServer object
type ReconcileWebServer struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client      client.Client
	scheme      *runtime.Scheme
	isOpenShift bool
	useKUBEPing bool
}

// Reconcile reads that state of the cluster for a WebServer object and makes changes based on the state read
// and what is in the WebServer.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileWebServer) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	//Log an empty line to separate reconciliation logs
	log.Info("")
	log = logf.Log.WithName("webserver_controller").WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	log.Info("Reconciling WebServer")
	updateStatus := false
	requeue := false
	updateDeployment := false
	isKubernetes := !r.isOpenShift
	result := reconcile.Result{}
	var err error = nil

	// Fetch the WebServer
	webServer, err := r.getWebServer(request)
	if err != nil {
		return reconcile.Result{}, err
	}

	webServer = r.setDefaultValues(webServer)

	if webServer.Spec.WebImageStream != nil && webServer.Spec.WebImage != nil {
		log.Error(err, "Both the WebImageStream and WebImage fields are being used. Only one can be used.")
		return reconcile.Result{}, err
	} else if webServer.Spec.WebImageStream == nil && webServer.Spec.WebImage == nil {
		log.Error(err, "WebImageStream or WebImage required")
		return reconcile.Result{}, err
	} else if webServer.Spec.WebImageStream != nil && isKubernetes {
		log.Error(err, "Image Streams can only be used in an Openshift cluster")
		return reconcile.Result{}, nil
	}

	// Check if a Service for routing already exists, and if not create a new one
	routingService := r.generateRoutingService(webServer)
	result, err = r.createResource(webServer, routingService, routingService.Kind, routingService.Name, routingService.Namespace)
	if err != nil || result != (reconcile.Result{}) {
		return result, err
	}

	if webServer.Spec.UseSessionClustering {

		if r.useKUBEPing {

			// Check if a RoleBinding for the KUBEPing exists, and if not create one.
			rolebinding := r.generateRoleBinding(webServer)
			result, err = r.createResource(webServer, rolebinding, rolebinding.Kind, rolebinding.Name, rolebinding.Namespace)
			if err != nil || result != (reconcile.Result{}) {
				return result, err
			}

		} else {

			// Check if a Service for DNSPing already exists, and if not create a new one
			dnsService := r.generateServiceForDNS(webServer)
			result, err = r.createResource(webServer, dnsService, dnsService.Kind, dnsService.Name, dnsService.Namespace)
			if err != nil || result != (reconcile.Result{}) {
				return result, err
			}

		}

		// Check if a ConfigMap for the KUBEPing exists, and if not create one.
		configMap := r.generateConfigMapForDNS(webServer)
		result, err = r.createResource(webServer, configMap, configMap.Kind, configMap.Name, configMap.Namespace)
		if err != nil || result != (reconcile.Result{}) {
			return result, err
		}

	}

	var foundReplicas int32
	if webServer.Spec.WebImage != nil {

		// Check if a webapp needs to be built
		if webServer.Spec.WebImage.WebApp != nil && webServer.Spec.WebImage.WebApp.SourceRepositoryURL != "" && webServer.Spec.WebImage.WebApp.Builder != nil && webServer.Spec.WebImage.WebApp.Builder.Image != "" {

			// Check if a Persistent Volume Claim already exists, and if not create a new one
			pvc := r.generatePersistentVolumeClaim(webServer)
			result, err = r.createResource(webServer, pvc, pvc.Kind, pvc.Name, pvc.Namespace)
			if err != nil || result != (reconcile.Result{}) {
				return result, err
			}

			// Check if a build Pod for the webapp already exists, and if not create a new one
			buildPod := r.generateBuildPod(webServer)
			result, err = r.createResource(webServer, buildPod, buildPod.Kind, buildPod.Name, buildPod.Namespace)
			if err != nil || result != (reconcile.Result{}) {
				return result, err
			}

			result, err = r.checkBuildPodPhase(buildPod)
			if err != nil || result != (reconcile.Result{}) {
				return result, err
			}

		}

		// Check if a Deployment already exists, and if not create a new one
		deployment := r.generateDeployment(webServer)
		result, err = r.createResource(webServer, deployment, deployment.Kind, deployment.Name, deployment.Namespace)
		if err != nil || result != (reconcile.Result{}) {
			return result, err
		}

		foundImage := deployment.Spec.Template.Spec.Containers[0].Image
		if foundImage != webServer.Spec.WebImage.ApplicationImage {
			log.Info("WebServer application image change detected. Deployment update scheduled")
			deployment.Spec.Template.Spec.Containers[0].Image = webServer.Spec.WebImage.ApplicationImage
			updateDeployment = true
		}

		// Handle Scaling
		foundReplicas = *deployment.Spec.Replicas
		replicas := webServer.Spec.Replicas
		if foundReplicas != replicas {
			log.Info("Deployment replicas number does not match the WebServer specification. Deployment update scheduled")
			deployment.Spec.Replicas = &replicas
			updateDeployment = true
		}

		if updateDeployment {
			err = r.client.Update(context.TODO(), deployment)
			if err != nil {
				log.Error(err, "Failed to update Deployment.", "Deployment.Namespace", deployment.Namespace, "Deployment.Name", deployment.Name)
				return reconcile.Result{}, err
			}
			// Spec updated - return and requeue
			return reconcile.Result{Requeue: true}, nil
		}

	} else if webServer.Spec.WebImageStream != nil {

		imageStreamName := webServer.Spec.WebImageStream.ImageStreamName
		imageStreamNamespace := webServer.Spec.WebImageStream.ImageStreamNamespace

		// Check if we need to build the webapp from sources
		if webServer.Spec.WebImageStream.WebSources != nil {

			// Check if an Image Stream already exists, and if not create a new one
			imageStream := r.generateImageStream(webServer)
			result, err = r.createResource(webServer, imageStream, imageStream.Kind, imageStream.Name, imageStream.Namespace)
			if err != nil || result != (reconcile.Result{}) {
				return result, err
			}

			// Change the Image Stream that the deployment config will use later to deploy the webserver
			imageStreamName = imageStream.Name
			imageStreamNamespace = imageStream.Namespace

			// Check if a BuildConfig already exists, and if not create a new one
			buildConfig := r.generateBuildConfig(webServer)
			result, err = r.createResource(webServer, buildConfig, buildConfig.Kind, buildConfig.Name, buildConfig.Namespace)
			if err != nil || result != (reconcile.Result{}) {
				return result, err
			}

			// Check if a Build has been created by the BuildConfig
			build := &buildv1.Build{}
			err = r.client.Get(context.TODO(), types.NamespacedName{Name: webServer.Spec.ApplicationName + "-" + strconv.FormatInt(buildConfig.Status.LastVersion, 10), Namespace: webServer.Namespace}, build)
			if err != nil && !errors.IsNotFound(err) {
				log.Info("Failed to get the Build")
				return reconcile.Result{}, err
			}

			// If the Build was unsuccessful, stop the operator
			switch build.Status.Phase {

			case buildv1.BuildPhaseFailed:
				log.Info("Application build failed: " + build.Status.Message)
				return reconcile.Result{}, nil
			case buildv1.BuildPhaseError:
				log.Info("Application build failed: " + build.Status.Message)
				return reconcile.Result{}, nil
			case buildv1.BuildPhaseCancelled:
				log.Info("Application build canceled")
				return reconcile.Result{}, nil

			}
		}

		// Check if a DeploymentConfig already exists and if not, create a new one
		deploymentConfig := r.generateDeploymentConfig(webServer, imageStreamName, imageStreamNamespace)
		result, err = r.createResource(webServer, deploymentConfig, deploymentConfig.Kind, deploymentConfig.Name, deploymentConfig.Namespace)
		if err != nil || result != (reconcile.Result{}) {
			return result, err
		}

		if int(deploymentConfig.Status.LatestVersion) == 0 {
			log.Info("The DeploymentConfig has not finished deploying the pods yet")
			return reconcile.Result{RequeueAfter: (500 * time.Millisecond)}, nil
		}

		// Handle Scaling
		foundReplicas = deploymentConfig.Spec.Replicas
		replicas := webServer.Spec.Replicas
		if foundReplicas != replicas {
			log.Info("DeploymentConfig replicas number does not match the WebServer specification. DeploymentConfig update scheduled")
			deploymentConfig.Spec.Replicas = replicas
			updateDeployment = true
		}

		if updateDeployment {
			err = r.client.Update(context.TODO(), deploymentConfig)
			if err != nil {
				log.Info("Failed to update DeploymentConfig.", "DeploymentConfig.Namespace", deploymentConfig.Namespace, "DeploymentConfig.Name", deploymentConfig.Name)
				return reconcile.Result{}, err
			}
			// Spec updated - return and requeue
			return reconcile.Result{Requeue: true}, nil
		}
	}

	if r.isOpenShift {

		// Check if a Route already exists, and if not create a new one
		route := r.generateRoute(webServer)
		result, err = r.createResource(webServer, route, route.Kind, route.Name, route.Namespace)
		if err != nil || result != (reconcile.Result{}) {
			return result, err
		}

		hosts := make([]string, len(route.Status.Ingress))
		for i, ingress := range route.Status.Ingress {
			hosts[i] = ingress.Host
		}

		sort.Strings(hosts)
		if !reflect.DeepEqual(hosts, webServer.Status.Hosts) {
			updateStatus = true
			webServer.Status.Hosts = hosts
			log.Info("Status.Hosts update scheduled")
		}
	}

	// List of pods which belongs under this webServer instance
	podList, err := r.getPodList(webServer)
	if err != nil {
		log.Error(err, "Failed to get pod list.", "WebServer.Namespace", webServer.Namespace, "WebServer.Name", webServer.Name)
		return reconcile.Result{}, err
	}

	// Make sure the number of active pods is the desired replica size.
	numberOfDeployedPods := int32(len(podList.Items))
	if numberOfDeployedPods != webServer.Spec.Replicas {
		log.Info("The number of deployed pods does not match the WebServer specification, reconciliation requeue scheduled")
		requeue = true
	}

	// Get the status of the active pods
	podsStatus, requeue := r.getPodStatus(podList.Items)
	if !reflect.DeepEqual(podsStatus, webServer.Status.Pods) {
		log.Info("Status.Pods update scheduled")
		webServer.Status.Pods = podsStatus
		updateStatus = true
	}

	// Update the replicas
	if webServer.Status.Replicas != foundReplicas {
		log.Info("Status.Replicas update scheduled")
		webServer.Status.Replicas = foundReplicas
		updateStatus = true
	}

	// Update the scaledown
	numberOfPodsToScaleDown := foundReplicas - webServer.Spec.Replicas
	if webServer.Status.ScalingdownPods != numberOfPodsToScaleDown {
		log.Info("Status.ScalingdownPods update scheduled")
		webServer.Status.ScalingdownPods = numberOfPodsToScaleDown
		updateStatus = true
	}

	if updateStatus {
		err := r.updateWebServerStatus(webServer, r.client)
		if err != nil {
			return reconcile.Result{}, err
		}
	}

	if requeue {
		log.Info("Requeuing reconciliation")
		return reconcile.Result{RequeueAfter: (500 * time.Millisecond)}, nil
	}

	log.Info("Reconciliation complete")
	return reconcile.Result{}, nil
}
