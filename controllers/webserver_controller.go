package controllers

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	webserversv1alpha1 "github.com/web-servers/jws-operator/api/v1alpha1"

	appsv1 "github.com/openshift/api/apps/v1"
	buildv1 "github.com/openshift/api/build/v1"

	imagestreamv1 "github.com/openshift/api/image/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
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
// func Add(mgr manager.Manager) error {
// 	return add(mgr, newReconciler(mgr))
// }

// newReconciler returns a new reconcile.Reconciler
// func newReconciler(mgr manager.Manager) reconcile.Reconciler {
// 	return &WebServerReconciler{client: mgr.GetClient(), scheme: mgr.GetScheme(), isOpenShift: isOpenShift(mgr.GetConfig()), useKUBEPing: true}
// }

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func (r *WebServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.isOpenShift = isOpenShift(mgr.GetConfig())
	r.hasServiceMonitor = hasServiceMonitor(mgr.GetConfig())
	if r.isOpenShift {
		return ctrl.NewControllerManagedBy(mgr).
			For(&webserversv1alpha1.WebServer{}).
			Owns(&appsv1.DeploymentConfig{}).
			Complete(r)
	} else {
		return ctrl.NewControllerManagedBy(mgr).
			For(&webserversv1alpha1.WebServer{}).
			Complete(r)
	}

}

// var _ reconcile.Reconciler = &WebServerReconciler{}

// WebServerReconciler reconciles a WebServer object
type WebServerReconciler struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	// client      client.Client
	// scheme      *runtime.Scheme
	client.Client
	*runtime.Scheme
	isOpenShift       bool
	hasServiceMonitor bool
}

// It seems we shouldn't mess up directly in role.yaml...
// and it is probably needing a _very_ carefull check here too !!
// +kubebuilder:rbac:groups="core",resources=configmaps,verbs=create;get;list;delete;watch
// +kubebuilder:rbac:groups="core",resources=pods,verbs=create;get;list;delete;watch
// +kubebuilder:rbac:groups="core",resources=services,verbs=create;get;list;delete;watch
// +kubebuilder:rbac:groups="core",resources=persistentvolumeclaims,verbs=create;get;list;delete;watch
// +kubebuilder:rbac:groups="core",resources=services/finalizers,verbs=update
// +kubebuilder:rbac:groups="core",resources=namespaces,verbs=get

// +kubebuilder:rbac:groups="apps",resources=jws-operator,verbs=update
// +kubebuilder:rbac:groups="apps",resources=deployments,verbs=create;get;list;delete;watch;update;patch
// +kubebuilder:rbac:groups="apps",resources=deployments/finalizers,verbs=update

// +kubebuilder:rbac:groups="apps.openshift.io",resources=deploymentconfigs,verbs=create;get;list;delete;update;watch

// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=servicemonitors,verbs=create;get;

// +kubebuilder:rbac:groups=image.openshift.io,resources=imagestreams,verbs=create;get;list;delete;watch

// +kubebuilder:rbac:groups=build.openshift.io,resources=buildconfigs,verbs=create;get;list;delete;watch
// +kubebuilder:rbac:groups=build.openshift.io,resources=builds,verbs=create;get;list;delete;watch

// +kubebuilder:rbac:groups=apps.openshift.io,resources=deploymentconfigs,verbs=create;get;list;delete

// +kubebuilder:rbac:groups=route.openshift.io,resources=routes,verbs=create;get;list;delete;watch

// +kubebuilder:rbac:groups=web.servers.org,resources=webservers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=web.servers.org,resources=webservers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=web.servers.org,resources=webservers/finalizers,verbs=update

// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=rolebindings,verbs=create;grant;get;list;watch

// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=servicemonitors,verbs=create;delete;get;list;watch

// Reconcile reads that state of the cluster for a WebServer object and makes changes based on the state read
// and what is in the WebServer.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
// func (r *WebServerReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
func (r *WebServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	//Log an empty line to separate reconciliation logs
	log.Info("")
	log = logf.Log.WithName("webserver_controller").WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)
	log.Info("Reconciling WebServer")
	updateStatus := false
	requeue := false
	updateDeployment := false
	isKubernetes := !r.isOpenShift
	result := ctrl.Result{}
	var err error = nil

	// Fetch the WebServer
	webServer, err := r.getWebServer(ctx, req)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	webServer = r.setDefaultValues(webServer)

	// Set the selector label, this should be done for the pods as well to allow targeting the CR with HPA
	webServer.Status.Selector = fmt.Sprintf("app.kubernetes.io/name=%s", webServer.Name)

	// create a Prometheus ServiceMonitor (if the resource exists on the cluster)
	if r.hasServiceMonitor {
		if serviceMonitor, err := r.GetOrCreateNewServiceMonitor(webServer, ctx, r.generateLabelsForWeb(webServer)); err != nil {
			return reconcile.Result{}, err
		} else if serviceMonitor == nil {
			log.Info("Webserver: Create Prometheus ServiceMonitor and requeue reconciliation")
			return reconcile.Result{Requeue: true}, nil
		}
		if servicePrometeus, err := r.GetOrCreateNewPrometheusService(webServer, ctx, r.generateLabelsForWeb(webServer)); err != nil {
			return reconcile.Result{}, err
		} else if servicePrometeus == nil {
			log.Info("Webserver: Create Prometheus Service and requeue reconciliation")
			return reconcile.Result{Requeue: true}, nil
		}
	}

	if webServer.Spec.WebImageStream != nil && webServer.Spec.WebImage != nil {
		log.Error(err, "Both the WebImageStream and WebImage fields are being used. Only one can be used.")
		return ctrl.Result{}, err
	} else if webServer.Spec.WebImageStream == nil && webServer.Spec.WebImage == nil {
		log.Error(err, "WebImageStream or WebImage required")
		return ctrl.Result{}, err
	} else if webServer.Spec.WebImageStream != nil && isKubernetes {
		log.Error(err, "Image Streams can only be used in an Openshift cluster")
		return ctrl.Result{}, nil
	}

	// Check if a Service for routing already exists, and if not create a new one
	routingService := &corev1.Service{}
	if strings.HasPrefix(webServer.Spec.RouteHostname, "TLS") || strings.HasPrefix(webServer.Spec.RouteHostname, "tls") {
		log.Info("generating routing service with port 8443 " + "cause webServer.Spec.RouteHostname= " + webServer.Spec.RouteHostname)
		routingService = r.generateRoutingService(webServer, 8443)
	} else {
		log.Info("generating routing service with port 8080 " + "cause webServer.Spec.RouteHostname= " + webServer.Spec.RouteHostname)
		routingService = r.generateRoutingService(webServer, 8080)
	}
	result, err = r.createService(ctx, webServer, routingService, routingService.Name, routingService.Namespace)
	if err != nil || result != (ctrl.Result{}) {
		return result, err
	}

	if webServer.Spec.UseSessionClustering {

		if r.needgetUseKUBEPing(webServer) {

			// Check if a RoleBinding for the KUBEPing exists, and if not create one.
			rolename := "view-kubeping-" + webServer.Name
			rolebinding := r.generateRoleBinding(webServer, rolename)
			//
			// The example in docs seems to use view (name: view and roleRef ClusterRole/view for our ServiceAccount)
			// like:
			// oc policy add-role-to-user view system:serviceaccount:tomcat-in-the-cloud:default -n tomcat-in-the-cloud
			useKUBEPing, update, err := r.createRoleBinding(ctx, webServer, rolebinding, rolename, rolebinding.Namespace)
			if !useKUBEPing {
				// Update the webServer annotation to prevent retrying
				log.Info("Won't use KUBEPing missing view permissions")
			} else {
				log.Info("Will use KUBEPing")
				if update {
					// We have created the Role Binding
					log.Info("We have created the Role Binding")
					return ctrl.Result{Requeue: update}, nil
				}
			}
			update, err = r.setUseKUBEPing(ctx, webServer, useKUBEPing)
			if err != nil {
				log.Error(err, "Failed to add a new Annotations")
				return reconcile.Result{}, err
			} else {
				if update {
					log.Info("Add a new Annotations")
					return ctrl.Result{Requeue: update}, nil
				}
			}
		}
		if !r.getUseKUBEPing(webServer) {

			// Check if a Service for DNSPing already exists, and if not create a new one
			dnsService := r.generateServiceForDNS(webServer)
			result, err = r.createService(ctx, webServer, dnsService, dnsService.Name, dnsService.Namespace)
			if err != nil || result != (ctrl.Result{}) {
				return result, err
			}

		}

		// Check if exists a ConfigMap for the server.xml <Cluster/> definition otherwise create it.
		configMap := r.generateConfigMapForDNS(webServer)
		result, err = r.createConfigMap(ctx, webServer, configMap, configMap.Name, configMap.Namespace)
		if err != nil || result != (ctrl.Result{}) {
			return result, err
		}

	}

	var foundReplicas int32
	if webServer.Spec.WebImage != nil {

		// Check if a webapp needs to be built
		if webServer.Spec.WebImage.WebApp != nil && webServer.Spec.WebImage.WebApp.SourceRepositoryURL != "" && webServer.Spec.WebImage.WebApp.Builder != nil && webServer.Spec.WebImage.WebApp.Builder.Image != "" {

			// Create a ConfigMap for custom build script
			if webServer.Spec.WebImage.WebApp.Builder.ApplicationBuildScript != "" {
				configMap := r.generateConfigMapForCustomBuildScript(webServer)
				result, err = r.createConfigMap(ctx, webServer, configMap, configMap.Name, configMap.Namespace)
				if err != nil || result != (ctrl.Result{}) {
					return result, err
				}
				// Check the script has changed, if yes delete it and requeue
				if configMap.Data["build.sh"] != webServer.Spec.WebImage.WebApp.Builder.ApplicationBuildScript {
					// Just Delete and requeue
					err = r.Client.Delete(ctx, configMap)
					if err != nil && errors.IsNotFound(err) {
						return ctrl.Result{}, nil
					}
					log.Info("Webserver hash changed: Delete Builder ConfigMap and requeue reconciliation")
					return ctrl.Result{RequeueAfter: (500 * time.Millisecond)}, nil
				}
			}

			// Check if a build Pod for the webapp already exists, and if not create a new one
			buildPod := r.generateBuildPod(webServer)
			log.Info("WebServe createBuildPod: " + buildPod.Name + " in " + buildPod.Namespace + " using: " + buildPod.Spec.Volumes[0].VolumeSource.Secret.SecretName + " and: " + buildPod.Spec.Containers[0].Image)
			result, err = r.createBuildPod(ctx, webServer, buildPod, buildPod.Name, buildPod.Namespace)
			if err != nil || result != (ctrl.Result{}) {
				return result, err
			}

			result, err = r.checkBuildPodPhase(buildPod)
			if err != nil || result != (ctrl.Result{}) {
				return result, err
			}

			// Check if we need to delete it and recreate it.
			currentHash := r.getWebServerHash(webServer)
			if buildPod.Labels["webserver-hash"] != currentHash {
				// Just Delete and requeue
				err = r.Client.Delete(ctx, buildPod)
				if err != nil && errors.IsNotFound(err) {
					return ctrl.Result{}, nil
				}
				log.Info("Webserver hash changed: Delete BuildPod and requeue reconciliation")
				return ctrl.Result{RequeueAfter: (500 * time.Millisecond)}, nil
			}

		}

		// Check if a Deployment already exists, and if not create a new one
		deployment := r.generateDeployment(webServer)
		log.Info("WebServe createDeployment: " + deployment.Name + " in " + deployment.Namespace + " using: " + deployment.Spec.Template.Spec.Containers[0].Image)
		result, err = r.createDeployment(ctx, webServer, deployment, deployment.Name, deployment.Namespace)
		if err != nil || result != (ctrl.Result{}) {
			log.Info("WebServer can't create deployment")
			return result, err
		}

		// Check if we need to update it.
		currentHash := r.getWebServerHash(webServer)
		if deployment.ObjectMeta.Labels["webserver-hash"] == "" {
			deployment.ObjectMeta.Labels["webserver-hash"] = currentHash
			updateDeployment = true
		} else {
			if deployment.ObjectMeta.Labels["webserver-hash"] != currentHash {
				// Just Update and requeue
				r.generateUpdatedDeployment(webServer, deployment)
				deployment.ObjectMeta.Labels["webserver-hash"] = currentHash
				err = r.Client.Update(ctx, deployment)
				if err != nil {
					log.Error(err, "Failed to update Deployment.", "Deployment.Namespace", deployment.Namespace, "Deployment.Name", deployment.Name)
					if errors.IsConflict(err) {
						log.V(1).Info(err.Error())
					} else {
						return ctrl.Result{}, nil
					}
				}
				log.Info("Webserver hash changed: Update Deployment and requeue reconciliation")
				return ctrl.Result{RequeueAfter: (500 * time.Millisecond)}, nil
			}
		}

		foundImage := deployment.Spec.Template.Spec.Containers[0].Image
		if foundImage != webServer.Spec.WebImage.ApplicationImage {
			// if we are using a builder that it normal otherwise we need to redeploy.
			if webServer.Spec.WebImage.WebApp == nil {
				log.Info("WebServer application image change detected. Deployment update scheduled")
				deployment.Spec.Template.Spec.Containers[0].Image = webServer.Spec.WebImage.ApplicationImage
				updateDeployment = true
			}
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
			err = r.Update(ctx, deployment)
			if err != nil {
				log.Error(err, "Failed to update Deployment.", "Deployment.Namespace", deployment.Namespace, "Deployment.Name", deployment.Name)
				if errors.IsConflict(err) {
					log.V(1).Info(err.Error())
				} else {
					return ctrl.Result{}, err
				}
			}
			// Spec updated - return and requeue
			return ctrl.Result{Requeue: true}, nil
		}

	} else if webServer.Spec.WebImageStream != nil {

		imageStreamName := webServer.Spec.WebImageStream.ImageStreamName
		imageStreamNamespace := webServer.Spec.WebImageStream.ImageStreamNamespace

		// Check if we need to build the webapp from sources
		if webServer.Spec.WebImageStream.WebSources != nil {

			// Check if an Image Stream already exists, and if not create a new one
			imageStream := r.generateImageStream(webServer)
			result, err = r.createImageStream(ctx, webServer, imageStream, imageStream.Name, imageStream.Namespace)
			if err != nil || result != (ctrl.Result{}) {
				return result, err
			}

			// Change the Image Stream that the deployment config will use later to deploy the webserver
			imageStreamName = imageStream.Name
			imageStreamNamespace = imageStream.Namespace

			is := &imagestreamv1.ImageStream{}

			err = r.Get(ctx, client.ObjectKey{
				Namespace: imageStreamNamespace,
				Name:      imageStreamName,
			}, is)

			if errors.IsNotFound(err) {
				log.Error(err, "Namespace/ImageStream doesn't exist.")
				return ctrl.Result{}, nil
			}

			// Check if a BuildConfig already exists, and if not create a new one
			buildConfig := r.generateBuildConfig(webServer)
			result, err = r.createBuildConfig(ctx, webServer, buildConfig, buildConfig.Name, buildConfig.Namespace)
			if err != nil || result != (ctrl.Result{}) {
				return result, err
			}

			// Check if a Build has been created by the BuildConfig
			build := &buildv1.Build{}
			err = r.Get(ctx, types.NamespacedName{Name: webServer.Spec.ApplicationName + "-" + strconv.FormatInt(buildConfig.Status.LastVersion, 10), Namespace: webServer.Namespace}, build)
			if err != nil && !errors.IsNotFound(err) {
				log.Info("Failed to get the Build")
				return ctrl.Result{}, err
			}

			// If the Build was unsuccessful, stop the operator
			switch build.Status.Phase {

			case buildv1.BuildPhaseFailed:
				log.Info("Application build failed: " + build.Status.Message)
				return ctrl.Result{}, nil
			case buildv1.BuildPhaseError:
				log.Info("Application build failed: " + build.Status.Message)
				return ctrl.Result{}, nil
			case buildv1.BuildPhaseCancelled:
				log.Info("Application build canceled")
				return ctrl.Result{}, nil

			}
		}

		is := &imagestreamv1.ImageStream{}

		err = r.Get(ctx, client.ObjectKey{
			Namespace: imageStreamNamespace,
			Name:      imageStreamName,
		}, is)

		if errors.IsNotFound(err) {
			log.Error(err, "Namespace/ImageStream doesn't exist.")
			return ctrl.Result{}, nil
		}

		// Check if a DeploymentConfig already exists and if not, create a new one
		deploymentConfig := r.generateDeploymentConfig(webServer, imageStreamName, imageStreamNamespace)
		result, err = r.createDeploymentConfig(ctx, webServer, deploymentConfig, deploymentConfig.Name, deploymentConfig.Namespace)
		if err != nil || result != (ctrl.Result{}) {
			return result, err
		}

		// Check if we need to delete it and recreate it.
		currentHash := r.getWebServerHash(webServer)
		if deploymentConfig.Labels["webserver-hash"] == "" {
			deploymentConfig.ObjectMeta.Labels["webserver-hash"] = currentHash
			updateDeployment = true
		} else {
			if deploymentConfig.Labels["webserver-hash"] != currentHash {
				// Just Update and requeue
				r.generateUpdatedDeploymentConfig(webServer, imageStreamName, imageStreamNamespace, deploymentConfig)
				deploymentConfig.ObjectMeta.Labels["webserver-hash"] = currentHash
				err = r.Client.Update(ctx, deploymentConfig)
				if err != nil {
					log.Error(err, "Failed to update DeploymentConfig.", "Deployment.Namespace", deploymentConfig.Namespace, "Deployment.Name", deploymentConfig.Name)
					if errors.IsConflict(err) {
						log.V(1).Info(err.Error())
					} else {
						return ctrl.Result{}, nil
					}
				}
				log.Info("Webserver hash changed: Update DeploymentConfig and requeue reconciliation")
				return ctrl.Result{RequeueAfter: (500 * time.Millisecond)}, nil
			}
		}

		if int(deploymentConfig.Status.LatestVersion) == 0 {
			log.Info("The DeploymentConfig has not finished deploying the pods yet")
			return ctrl.Result{RequeueAfter: (500 * time.Millisecond)}, nil
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
			err = r.Update(ctx, deploymentConfig)
			if err != nil {
				log.Info("Failed to update DeploymentConfig." + "DeploymentConfig.Namespace" + deploymentConfig.Namespace + "DeploymentConfig.Name" + deploymentConfig.Name)
				if errors.IsConflict(err) {
					log.V(1).Info(err.Error())
				} else {
					return ctrl.Result{}, err
				}

			}
			// Spec updated - return and requeue
			return ctrl.Result{Requeue: true}, nil
		}
	}

	if r.isOpenShift {

		if webServer.Spec.RouteHostname != "NONE" && (!strings.HasPrefix(webServer.Spec.RouteHostname, "TLS") && !strings.HasPrefix(webServer.Spec.RouteHostname, "tls")) {

			// Check if a Route already exists, and if not create a new one
			route := r.generateRoute(webServer)
			result, err = r.createRoute(ctx, webServer, route, route.Name, route.Namespace)
			if err != nil || result != (ctrl.Result{}) {
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
		} else if strings.HasPrefix(webServer.Spec.RouteHostname, "TLS") || strings.HasPrefix(webServer.Spec.RouteHostname, "tls") {
			// Check if a Route already exists, and if not create a new one
			route := r.generateSecureRoute(webServer)
			result, err = r.createRoute(ctx, webServer, route, route.Name, route.Namespace)
			if err != nil || result != (ctrl.Result{}) {
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
	} else {
		// on kuberntes we use a loadbalancer service
		loadbalancer := r.generateLoadBalancer(webServer)
		result, err = r.createService(ctx, webServer, loadbalancer, loadbalancer.Name, loadbalancer.Namespace)
		if err != nil || result != (ctrl.Result{}) {
			return result, err
		}
		if len(loadbalancer.Status.LoadBalancer.Ingress) == 0 {
			return ctrl.Result{Requeue: true}, nil
		}

		hosts := make([]string, len(loadbalancer.Status.LoadBalancer.Ingress))
		for i, ingress := range loadbalancer.Status.LoadBalancer.Ingress {
			hosts[i] = ingress.Hostname
			log.Info("Status.Hosts have: " + hosts[i])
		}
		log.Info("Status.Hosts number of Ingress: " + strconv.Itoa(len(loadbalancer.Status.LoadBalancer.Ingress)))

		sort.Strings(hosts)
		if !reflect.DeepEqual(hosts, webServer.Status.Hosts) {
			updateStatus = true
			webServer.Status.Hosts = hosts
			log.Info("Status.Hosts update scheduled")
		}
	}

	// List of pods which belongs under this webServer instance
	podList, err := r.getPodList(ctx, webServer)
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
	podsStatus, newrequeue := r.getPodStatus(podList.Items)
	if newrequeue {
		requeue = true
	}
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

		if err := r.Status().Update(ctx, webServer); err != nil {
			log.Error(err, "Failed to update the status of WebServer")
			if errors.IsConflict(err) {
				log.V(1).Info(err.Error())
				return ctrl.Result{Requeue: true}, nil
			} else {
				return ctrl.Result{}, err
			}
		}

	}

	if requeue {
		log.Info("Requeuing reconciliation")
		return ctrl.Result{RequeueAfter: (500 * time.Millisecond)}, nil
	}

	log.Info("Reconciliation complete")
	return ctrl.Result{}, nil
}
