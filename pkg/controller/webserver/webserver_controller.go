package webserver

import (
	"context"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/util/intstr"

	webserversv1alpha1 "github.com/web-servers/jws-operator/pkg/apis/webservers/v1alpha1"

	appsv1 "github.com/openshift/api/apps/v1"
	buildv1 "github.com/openshift/api/build/v1"
	imagev1 "github.com/openshift/api/image/v1"
	routev1 "github.com/openshift/api/route/v1"
	kbappsv1 "k8s.io/api/apps/v1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"

	// rbac "rbac.authorization.k8s.io/v1"
	rbac "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var (
	log = logf.Log.WithName("controller_webserver")
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
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling WebServer")
	updateStatus := false
	requeue := false
	updateDeployment := false

	// Fetch the WebServer tomcat
	webServer := &webserversv1alpha1.WebServer{}
	err := r.client.Get(context.TODO(), request.NamespacedName, webServer)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			reqLogger.Info("WebServer resource not found. Ignoring since object must have been deleted")
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		reqLogger.Error(err, "Failed to get WebServer resource")
		return reconcile.Result{}, err
	}

	ser := r.serviceForWebServer(webServer)
	// Check if the Service for the Route exists
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: ser.Name, Namespace: ser.Namespace}, &corev1.Service{})
	if err != nil && errors.IsNotFound(err) {
		// Define a new Service
		reqLogger.Info("Creating a new Service for the Route.", "Service.Namespace", ser.Namespace, "Service.Name", ser.Name)
		err = r.client.Create(context.TODO(), ser)
		if err != nil && !errors.IsAlreadyExists(err) {
			reqLogger.Error(err, "Failed to create a new Service.", "Service.Namespace", ser.Namespace, "Service.Name", ser.Name)
			return reconcile.Result{}, err
		}
		// Service created successfully - return and requeue
		return reconcile.Result{Requeue: true}, nil
	} else if err != nil {
		reqLogger.Error(err, "Failed to get Service.")
		return reconcile.Result{}, err
	}

	if webServer.Spec.UseSessionClustering {
		// Create a RoleBinding for the KUBEPing
		if r.useKUBEPing {
			rolebinding := r.roleBindingForWebServer(webServer)
			err = r.client.Get(context.TODO(), types.NamespacedName{Name: rolebinding.Name, Namespace: rolebinding.Namespace}, &rbac.RoleBinding{})
			if err != nil && errors.IsNotFound(err) {
				// Define a new RoleBinding
				reqLogger.Info("Creating a new RoleBinding.", "RoleBinding.Namespace", rolebinding.Namespace, "RoleBinding.Name", rolebinding.Name)
				err = r.client.Create(context.TODO(), rolebinding)
				if err != nil && !errors.IsAlreadyExists(err) {
					reqLogger.Error(err, "Failed to create a new RoleBinding.", "RoleBinding.Namespace", rolebinding.Namespace, "RoleBinding.Name", rolebinding.Name)
					// We ignore the error.
					return reconcile.Result{Requeue: true}, nil
				}
				// Service created successfully - return and requeue
				return reconcile.Result{Requeue: true}, nil
			} else if err != nil {
				reqLogger.Error(err, "Failed to get RoleBinding.")
				// We ignore the error.
				r.useKUBEPing = false
				return reconcile.Result{Requeue: true}, nil
			}
		}

		if !r.useKUBEPing {
			ser1 := r.serviceForWebServerDNS(webServer)
			// Check if the Service for DNSPing exists
			err = r.client.Get(context.TODO(), types.NamespacedName{Name: ser1.Name, Namespace: ser1.Namespace}, &corev1.Service{})
			if err != nil && errors.IsNotFound(err) {
				// Define a new Service
				reqLogger.Info("Creating a new Service for DNSPing.", "Service.Namespace", ser1.Namespace, "Service.Name", ser1.Name)
				err = r.client.Create(context.TODO(), ser1)
				if err != nil && !errors.IsAlreadyExists(err) {
					reqLogger.Error(err, "Failed to create a new Service.", "Service.Namespace", ser1.Namespace, "Service.Name", ser1.Name)
					return reconcile.Result{}, err
				}
				// Service created successfully - return and requeue
				return reconcile.Result{Requeue: true}, nil
			} else if err != nil {
				reqLogger.Error(err, "Failed to get Service.")
				return reconcile.Result{}, err
			}
		}
		cmap := r.cmapForWebServerDNS(webServer, r.useKUBEPing)
		// Check if the ConfigMap for DNSPing exists
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: cmap.Name, Namespace: cmap.Namespace}, &corev1.ConfigMap{})
		if err != nil && errors.IsNotFound(err) {
			// Define a new ConfigMap
			reqLogger.Info("Creating a new ConfigMap.", "ConfigMap.Namespace", cmap.Namespace, "ConfigMap.Name", cmap.Name)
			err = r.client.Create(context.TODO(), cmap)
			if err != nil && !errors.IsAlreadyExists(err) {
				reqLogger.Error(err, "Failed to create a new ConfigMap.", "ConfigMap.Namespace", cmap.Namespace, "ConfigMap.Name", cmap.Name)
				return reconcile.Result{}, err
			}
			// Service created successfully - return and requeue
			return reconcile.Result{Requeue: true}, nil
		} else if err != nil {
			reqLogger.Error(err, "Failed to get ConfigMap.")
			return reconcile.Result{}, err
		}

	}

	// Check if the Route already exists, if not create a new one
	if r.isOpenShift {
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: webServer.Spec.ApplicationName, Namespace: webServer.Namespace}, &routev1.Route{})
		if err != nil && errors.IsNotFound(err) {
			// Define a new Route
			rou := r.routeForWebServer(webServer)
			reqLogger.Info("Creating a new Route.", "Route.Namespace", rou.Namespace, "Route.Name", rou.Name)
			err = r.client.Create(context.TODO(), rou)
			if err != nil && !errors.IsAlreadyExists(err) {
				reqLogger.Error(err, "Failed to create a new Route.", "Route.Namespace", rou.Namespace, "Route.Name", rou.Name)
				return reconcile.Result{}, err
			}
			// Route created successfully - return and requeue
			return reconcile.Result{Requeue: true}, nil
		} else if err != nil {
			reqLogger.Error(err, "Failed to get Route.")
			return reconcile.Result{}, err
		}
	}

	foundReplicas := int32(-1) // we need the foundDeployment.Spec.Replicas which is &appsv1.DeploymentConfig{} or &kbappsv1.Deployment{}
	webImage := webServer.Spec.WebImage
	applicationImage := ""
	if webImage != nil {
		applicationImage = webImage.ApplicationImage
	}
	if applicationImage == "" {
		// Are we building from sources and/or use the ImageStream

		if webServer.Spec.WebImageStream == nil {
			reqLogger.Info("WebImageStream or WebImage required")
			return reconcile.Result{}, nil
		}

		myImageName := webServer.Spec.WebImageStream.ImageStreamName
		myImageNameSpace := webServer.Spec.WebImageStream.ImageStreamNamespace

		if webServer.Spec.WebImageStream.WebSources != nil {
			// Check if the ImageStream already exists, if not create a new one
			img := &imagev1.ImageStream{}
			err = r.client.Get(context.TODO(), types.NamespacedName{Name: webServer.Spec.ApplicationName, Namespace: webServer.Namespace}, img)
			if err != nil && errors.IsNotFound(err) {
				// Define a new ImageStream
				img = r.imageStreamForWebServer(webServer)
				reqLogger.Info("Creating a new ImageStream.", "ImageStream.Namespace", img.Namespace, "ImageStream.Name", img.Name)
				err = r.client.Create(context.TODO(), img)
				if err != nil && !errors.IsAlreadyExists(err) {
					reqLogger.Error(err, "Failed to create a new ImageStream.", "ImageStream.Namespace", img.Namespace, "ImageStream.Name", img.Name)
					return reconcile.Result{}, err
				}
				// ImageStream created successfully - return and requeue
				return reconcile.Result{Requeue: true}, nil
			} else if err != nil {
				reqLogger.Error(err, "Failed to get ImageStream.")
				return reconcile.Result{}, err
			}
			myImageName = img.Name
			myImageNameSpace = img.Namespace

			buildConfig := &buildv1.BuildConfig{}
			// Check if the BuildConfig already exists, if not create a new one
			err = r.client.Get(context.TODO(), types.NamespacedName{Name: webServer.Spec.ApplicationName, Namespace: webServer.Namespace}, buildConfig)
			if err != nil && errors.IsNotFound(err) {
				// Define a new BuildConfig
				buildConfig = r.buildConfigForWebServer(webServer)
				reqLogger.Info("Creating a new BuildConfig.", "BuildConfig.Namespace", buildConfig.Namespace, "BuildConfig.Name", buildConfig.Name)
				err = r.client.Create(context.TODO(), buildConfig)
				if err != nil && !errors.IsAlreadyExists(err) {
					reqLogger.Error(err, "Failed to create a new BuildConfig.", "BuildConfig.Namespace", buildConfig.Namespace, "BuildConfig.Name", buildConfig.Name)
					return reconcile.Result{}, err
				}
				// BuildConfig created successfully - return and requeue
				return reconcile.Result{Requeue: true}, nil
			} else if err != nil {
				reqLogger.Error(err, "Failed to get BuildConfig.")
				return reconcile.Result{}, err
			}

			build := &buildv1.Build{}
			err = r.client.Get(context.TODO(), types.NamespacedName{Name: webServer.Spec.ApplicationName + "-" + strconv.FormatInt(buildConfig.Status.LastVersion, 10), Namespace: webServer.Namespace}, build)
			if err != nil && !errors.IsNotFound(err) {
				reqLogger.Error(err, "Failed to get Build")
			}

			switch build.Status.Phase {
			case buildv1.BuildPhaseFailed:
				reqLogger.Info("Application build failed: " + build.Status.Message)
				return reconcile.Result{}, nil
			case buildv1.BuildPhaseError:
				reqLogger.Info("Application build failed: " + build.Status.Message)
				return reconcile.Result{}, nil
			case buildv1.BuildPhaseCancelled:
				reqLogger.Info("Application build canceled")
				return reconcile.Result{}, nil
			}
		}

		// Check if the DeploymentConfig already exists, if not create a new one
		foundDeployment := &appsv1.DeploymentConfig{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: webServer.Spec.ApplicationName, Namespace: webServer.Namespace}, foundDeployment)
		if err != nil && errors.IsNotFound(err) {
			// Define a new DeploymentConfig
			dep := r.deploymentConfigForWebServer(webServer, myImageName, myImageNameSpace, r.useKUBEPing)
			reqLogger.Info("Creating a new DeploymentConfig.", "DeploymentConfig.Namespace", dep.Namespace, "DeploymentConfig.Name", dep.Name)
			err = r.client.Create(context.TODO(), dep)
			if err != nil && !errors.IsAlreadyExists(err) {
				reqLogger.Error(err, "Failed to create a new DeploymentConfig.", "DeploymentConfig.Namespace", dep.Namespace, "DeploymentConfig.Name", dep.Name)
				return reconcile.Result{}, err
			}
			// DeploymentConfig created successfully - return and requeue
			return reconcile.Result{Requeue: true}, nil
		} else if err != nil {
			reqLogger.Error(err, "Failed to get DeploymentConfig.")
			return reconcile.Result{}, err
		}

		if int(foundDeployment.Status.LatestVersion) == 0 {
			reqLogger.Info("The DeploymentConfig has not finished deploying the pods yet")
		}

		// Handle Scaling
		foundReplicas = foundDeployment.Spec.Replicas
		replicas := webServer.Spec.Replicas
		if foundReplicas != replicas {
			reqLogger.Info("DeploymentConfig replicas number does not match the WebServer specification")
			foundDeployment.Spec.Replicas = replicas
			err = r.client.Update(context.TODO(), foundDeployment)
			if err != nil {
				reqLogger.Error(err, "Failed to update DeploymentConfig.", "DeploymentConfig.Namespace", foundDeployment.Namespace, "DeploymentConfig.Name", foundDeployment.Name)
				return reconcile.Result{}, err
			}
			// Spec updated - return and requeue
			return reconcile.Result{Requeue: true}, nil
		}
	} else {
		// Check if the Deployment already exists, if not create a new one
		foundDeployment := &kbappsv1.Deployment{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: webServer.Spec.ApplicationName, Namespace: webServer.Namespace}, foundDeployment)
		if err != nil && errors.IsNotFound(err) {
			// Define a new Deployment
			dep := r.deploymentForWebServer(webServer, r.useKUBEPing)
			reqLogger.Info("Creating a new Deployment.", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
			err = r.client.Create(context.TODO(), dep)
			if err != nil && !errors.IsAlreadyExists(err) {
				reqLogger.Error(err, "Failed to create a new Deployment.", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
				return reconcile.Result{}, err
			}
			// Deployment created successfully - return and requeue
			return reconcile.Result{Requeue: true}, nil
		} else if err != nil {
			reqLogger.Error(err, "Failed to get Deployment.")
			return reconcile.Result{}, err
		}

		foundImage := foundDeployment.Spec.Template.Spec.Containers[0].Image
		if foundImage != applicationImage {
			reqLogger.Info("WebServer application image change detected. Deployment update scheduled")
			foundDeployment.Spec.Template.Spec.Containers[0].Image = applicationImage
			updateDeployment = true
		}

		// Handle Scaling
		foundReplicas = *foundDeployment.Spec.Replicas
		replicas := webServer.Spec.Replicas
		if foundReplicas != replicas {
			reqLogger.Info("Deployment replicas number does not match the WebServer specification")
			foundDeployment.Spec.Replicas = &replicas
			updateDeployment = true
		}

		if updateDeployment {
			err = r.client.Update(context.TODO(), foundDeployment)
			if err != nil {
				reqLogger.Error(err, "Failed to update Deployment.", "Deployment.Namespace", foundDeployment.Namespace, "Deployment.Name", foundDeployment.Name)
				return reconcile.Result{}, err
			}
			// Spec updated - return and requeue
			return reconcile.Result{Requeue: true}, nil
		}
	}

	// List of pods which belongs under this webServer instance
	podList, err := GetPodsForWebServer(r, webServer)
	if err != nil {
		reqLogger.Error(err, "Failed to get pod list.", "WebServer.Namespace", webServer.Namespace, "WebServer.Name", webServer.Name)
		return reconcile.Result{}, err
	}

	// Update the pod status...
	podsMissingIP, podsStatus := getPodStatus(podList.Items)
	if podsMissingIP {
		reqLogger.Info("Some pods don't have an IP address yet, reconciliation requeue scheduled")
		requeue = true
	}
	if !reflect.DeepEqual(podsStatus, webServer.Status.Pods) {
		// reqLogger.Info("Will update the WebServer pod status", "New pod status list", podsStatus)
		// reqLogger.Info("Will update the WebServer pod status", "Existing pod status list", webServer.Status.Pods)
		reqLogger.Info("Status.Pods update scheduled")
		webServer.Status.Pods = podsStatus
		updateStatus = true
	}

	if r.isOpenShift {
		route := &routev1.Route{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: webServer.Spec.ApplicationName, Namespace: webServer.Namespace}, route)
		if err != nil {
			reqLogger.Error(err, "Failed to get Route.", "Route.Namespace", route.Namespace, "Route.Name", route.Name)
			return reconcile.Result{}, err
		}

		hosts := make([]string, len(route.Status.Ingress))
		for i, ingress := range route.Status.Ingress {
			hosts[i] = ingress.Host
		}
		sort.Strings(hosts)
		if !reflect.DeepEqual(hosts, webServer.Status.Hosts) {
			updateStatus = true
			webServer.Status.Hosts = hosts
			reqLogger.Info("Status.Hosts update scheduled")
		}
	}

	// Make sure the number of active pods is the desired replica size.
	numberOfDeployedPods := int32(len(podList.Items))
	if numberOfDeployedPods != webServer.Spec.Replicas {
		reqLogger.Info("The number of deployed pods does not match the WebServer specification, reconciliation requeue scheduled")
		requeue = true
	}

	// Update the replicas
	if webServer.Status.Replicas != foundReplicas {
		reqLogger.Info("Status.Replicas update scheduled")
		webServer.Status.Replicas = foundReplicas
		updateStatus = true
	}
	// Update the scaledown
	numberOfPodsToScaleDown := foundReplicas - webServer.Spec.Replicas
	if webServer.Status.ScalingdownPods != numberOfPodsToScaleDown {
		reqLogger.Info("Status.ScalingdownPods update scheduled")
		webServer.Status.ScalingdownPods = numberOfPodsToScaleDown
		updateStatus = true
	}
	// Update if needed.
	if updateStatus {
		err := UpdateWebServerStatus(webServer, r.client)
		if err != nil {
			return reconcile.Result{}, err
		}
	}
	if requeue {
		reqLogger.Info("Requeuing reconciliation")
		return reconcile.Result{RequeueAfter: (500 * time.Millisecond)}, nil
	}
	reqLogger.Info("Reconciliation complete")
	return reconcile.Result{}, nil
}

func (r *ReconcileWebServer) serviceForWebServer(t *webserversv1alpha1.WebServer) *corev1.Service {

	service := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Service",
		},
		ObjectMeta: objectMetaForWebServer(t, t.Spec.ApplicationName),
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{{
				Name:       "ui",
				Port:       8080,
				TargetPort: intstr.FromInt(8080),
			}},
			Selector: map[string]string{
				"deploymentConfig": t.Spec.ApplicationName,
				"WebServer":        t.Name,
			},
		},
	}

	controllerutil.SetControllerReference(t, service, r.scheme)
	return service
}

func (r *ReconcileWebServer) serviceForWebServerDNS(t *webserversv1alpha1.WebServer) *corev1.Service {

	service := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Service",
		},
		ObjectMeta: objectMetaForWebServer(t, "webserver-"+t.Name),
		Spec: corev1.ServiceSpec{
			ClusterIP: "None",
			Ports: []corev1.ServicePort{{
				Name:       "http",
				Port:       8080,
				TargetPort: intstr.FromInt(8080),
			}},
			Selector: map[string]string{
				"application": t.Spec.ApplicationName,
			},
		},
	}

	controllerutil.SetControllerReference(t, service, r.scheme)
	return service
}

func (r *ReconcileWebServer) roleBindingForWebServer(t *webserversv1alpha1.WebServer) *rbac.RoleBinding {
	rolebinding := &rbac.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1beta",
			Kind:       "RoleBinding",
		},
		ObjectMeta: objectMetaForWebServer(t, "webserver-"+t.Name),
		RoleRef: rbac.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "view",
		},
		Subjects: []rbac.Subject{{
			Kind: "ServiceAccount",
			Name: "default",
		}},
	}
	controllerutil.SetControllerReference(t, rolebinding, r.scheme)
	return rolebinding
}

func (r *ReconcileWebServer) cmapForWebServerDNS(t *webserversv1alpha1.WebServer, useKUBEPing bool) *corev1.ConfigMap {

	cmap := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: objectMetaForWebServer(t, "webserver-"+t.Name),
		Data:       commandForServerXml(useKUBEPing),
	}

	controllerutil.SetControllerReference(t, cmap, r.scheme)
	return cmap
}

func (r *ReconcileWebServer) deploymentConfigForWebServer(t *webserversv1alpha1.WebServer, image string, namespace string, useKUBEPing bool) *appsv1.DeploymentConfig {

	replicas := int32(1)
	podTemplateSpec := podTemplateSpecForWebServer(t, t.Spec.ApplicationName, useKUBEPing)
	deploymentConfig := &appsv1.DeploymentConfig{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps.openshift.io/v1",
			Kind:       "DeploymentConfig",
		},
		ObjectMeta: objectMetaForWebServer(t, t.Spec.ApplicationName),
		Spec: appsv1.DeploymentConfigSpec{
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.DeploymentStrategyTypeRecreate,
			},
			Triggers: []appsv1.DeploymentTriggerPolicy{{
				Type: appsv1.DeploymentTriggerOnImageChange,
				ImageChangeParams: &appsv1.DeploymentTriggerImageChangeParams{
					Automatic:      true,
					ContainerNames: []string{t.Spec.ApplicationName},
					From: corev1.ObjectReference{
						Kind:      "ImageStreamTag",
						Name:      image + ":latest",
						Namespace: namespace,
					},
				},
			},
				{
					Type: appsv1.DeploymentTriggerOnConfigChange,
				}},
			Replicas: replicas,
			Selector: map[string]string{
				"deploymentConfig": t.Spec.ApplicationName,
				"WebServer":        t.Name,
			},
			Template: &podTemplateSpec,
		},
	}

	controllerutil.SetControllerReference(t, deploymentConfig, r.scheme)
	return deploymentConfig
}

func (r *ReconcileWebServer) deploymentForWebServer(t *webserversv1alpha1.WebServer, useKUBEPing bool) *kbappsv1.Deployment {

	replicas := int32(1)
	podTemplateSpec := podTemplateSpecForWebServer(t, t.Spec.WebImage.ApplicationImage, useKUBEPing)
	deployment := &kbappsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "k8s.io/api/apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: objectMetaForWebServer(t, t.Spec.ApplicationName),
		Spec: kbappsv1.DeploymentSpec{
			Strategy: kbappsv1.DeploymentStrategy{
				Type: kbappsv1.RecreateDeploymentStrategyType,
			},
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"deploymentConfig": t.Spec.ApplicationName,
					"WebServer":        t.Name,
				},
			},
			Template: podTemplateSpec,
		},
	}

	controllerutil.SetControllerReference(t, deployment, r.scheme)
	return deployment
}

func objectMetaForWebServer(t *webserversv1alpha1.WebServer, name string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      name,
		Namespace: t.Namespace,
		Labels: map[string]string{
			"application": t.Spec.ApplicationName,
		},
	}
}

func podTemplateSpecForWebServer(t *webserversv1alpha1.WebServer, image string, useKUBEPing bool) corev1.PodTemplateSpec {
	objectMeta := objectMetaForWebServer(t, t.Spec.ApplicationName)
	objectMeta.Labels["deploymentConfig"] = t.Spec.ApplicationName
	objectMeta.Labels["WebServer"] = t.Name
	var health *webserversv1alpha1.WebServerHealthCheckSpec = &webserversv1alpha1.WebServerHealthCheckSpec{}
	if t.Spec.WebImage != nil {
		health = t.Spec.WebImage.WebServerHealthCheck
	} else {
		health = t.Spec.WebImageStream.WebServerHealthCheck
	}
	terminationGracePeriodSeconds := int64(60)
	return corev1.PodTemplateSpec{
		ObjectMeta: objectMeta,
		Spec: corev1.PodSpec{
			TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
			Containers: []corev1.Container{{
				Name:            t.Spec.ApplicationName,
				Image:           image,
				ImagePullPolicy: "Always",
				ReadinessProbe:  createReadinessProbe(t, health),
				LivenessProbe:   createLivenessProbe(t, health),
				Ports: []corev1.ContainerPort{{
					Name:          "jolokia",
					ContainerPort: 8778,
					Protocol:      corev1.ProtocolTCP,
				}, {
					Name:          "http",
					ContainerPort: 8080,
					Protocol:      corev1.ProtocolTCP,
				}},
				Env:          createEnvVars(t, useKUBEPing),
				VolumeMounts: createVolumeMounts(t),
			}},
			Volumes: createVolumes(t),
		},
	}
}

func (r *ReconcileWebServer) routeForWebServer(t *webserversv1alpha1.WebServer) *routev1.Route {
	objectMeta := objectMetaForWebServer(t, t.Spec.ApplicationName)
	objectMeta.Annotations = map[string]string{
		"description": "Route for application's http service.",
	}
	route := &routev1.Route{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "route.openshift.io/v1",
			Kind:       "Route",
		},
		ObjectMeta: objectMeta,
		Spec: routev1.RouteSpec{
			To: routev1.RouteTargetReference{
				Name: t.Spec.ApplicationName,
			},
		},
	}

	controllerutil.SetControllerReference(t, route, r.scheme)
	return route
}

func (r *ReconcileWebServer) imageStreamForWebServer(t *webserversv1alpha1.WebServer) *imagev1.ImageStream {

	imageStream := &imagev1.ImageStream{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "image.openshift.io/v1",
			Kind:       "ImageStream",
		},
		ObjectMeta: objectMetaForWebServer(t, t.Spec.ApplicationName),
	}

	controllerutil.SetControllerReference(t, imageStream, r.scheme)
	return imageStream
}

func (r *ReconcileWebServer) buildConfigForWebServer(t *webserversv1alpha1.WebServer) *buildv1.BuildConfig {

	buildConfig := &buildv1.BuildConfig{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "build.openshift.io/v1",
			Kind:       "BuildConfig",
		},
		ObjectMeta: objectMetaForWebServer(t, t.Spec.ApplicationName),
		Spec: buildv1.BuildConfigSpec{
			CommonSpec: buildv1.CommonSpec{
				Source: buildv1.BuildSource{
					Type: "Git",
					Git: &buildv1.GitBuildSource{
						URI: t.Spec.WebImageStream.WebSources.SourceRepositoryURL,
						Ref: t.Spec.WebImageStream.WebSources.SourceRepositoryRef,
					},
					ContextDir: t.Spec.WebImageStream.WebSources.ContextDir,
				},
				Strategy: buildv1.BuildStrategy{
					Type: "Source",
					SourceStrategy: &buildv1.SourceBuildStrategy{
						Env:       createEnvBuild(t),
						ForcePull: true,
						From: corev1.ObjectReference{
							Kind:      "ImageStreamTag",
							Namespace: t.Spec.WebImageStream.ImageStreamNamespace,
							Name:      t.Spec.WebImageStream.ImageStreamName + ":latest",
						},
					},
				},
				Output: buildv1.BuildOutput{
					To: &corev1.ObjectReference{
						Kind: "ImageStreamTag",
						Name: t.Spec.ApplicationName + ":latest",
					},
				},
			},
			Triggers: createBuildTriggerPolicy(t),
		},
	}

	controllerutil.SetControllerReference(t, buildConfig, r.scheme)
	return buildConfig
}

// create the shell script to modify server.xml
//
func commandForServerXml(useKUBEPing bool) map[string]string {
	cmd := make(map[string]string)
	if useKUBEPing {
		cmd["test.sh"] = "FILE=`find /opt -name server.xml`\n" +
			"grep -q MembershipProvider ${FILE}\n" +
			"if [ $? -ne 0 ]; then\n" +
			"  sed -i '/cluster.html/a        <Cluster className=\"org.apache.catalina.ha.tcp.SimpleTcpCluster\" channelSendOptions=\"6\">\\n <Channel className=\"org.apache.catalina.tribes.group.GroupChannel\">\\n <Membership className=\"org.apache.catalina.tribes.membership.cloud.CloudMembershipService\" membershipProviderClassName=\"org.apache.catalina.tribes.membership.cloud.KubernetesMembershipProvider\"/>\\n </Channel>\\n </Cluster>\\n' ${FILE}\n" +
			"fi\n"
	} else {
		cmd["test.sh"] = "FILE=`find /opt -name server.xml`\n" +
			"grep -q MembershipProvider ${FILE}\n" +
			"if [ $? -ne 0 ]; then\n" +
			"  sed -i '/cluster.html/a        <Cluster className=\"org.apache.catalina.ha.tcp.SimpleTcpCluster\" channelSendOptions=\"6\">\\n <Channel className=\"org.apache.catalina.tribes.group.GroupChannel\">\\n <Membership className=\"org.apache.catalina.tribes.membership.cloud.CloudMembershipService\" membershipProviderClassName=\"org.apache.catalina.tribes.membership.cloud.DNSMembershipProvider\"/>\\n </Channel>\\n </Cluster>\\n' ${FILE}\n" +
			"fi\n"
	}
	return cmd
}

// createLivenessProbe returns a custom probe if the serverLivenessScript string is defined and not empty in the Custom Resource.
// Otherwise, it uses the default /health Valve via curl.
//
// If defined, serverLivenessScript must be a shell script that
// complies to the Kubernetes probes requirements and use the following format
// shell -c "command"
func createLivenessProbe(t *webserversv1alpha1.WebServer, health *webserversv1alpha1.WebServerHealthCheckSpec) *corev1.Probe {
	livenessProbeScript := ""
	if health != nil {
		livenessProbeScript = health.ServerLivenessScript
	}
	if livenessProbeScript != "" {
		return createCustomProbe(t, livenessProbeScript)
	} else {
		/* Use the default one */
		return &corev1.Probe{
			Handler: corev1.Handler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/health",
					Port: intstr.FromInt(8080),
				},
			},
		}
	}
}

// createReadinessProbe returns a custom probe if the serverReadinessScript string is defined and not empty in the Custom Resource.
// Otherwise, it uses the default /health Valve via curl.
//
// If defined, serverReadinessScript must be a shell script that
// complies to the Kubernetes probes requirements and use the following format
// shell -c "command"
func createReadinessProbe(t *webserversv1alpha1.WebServer, health *webserversv1alpha1.WebServerHealthCheckSpec) *corev1.Probe {
	readinessProbeScript := ""
	if health != nil {
		readinessProbeScript = health.ServerReadinessScript
	}
	if readinessProbeScript != "" {
		return createCustomProbe(t, readinessProbeScript)
	} else {
		/* Use the default one */
		return &corev1.Probe{
			Handler: corev1.Handler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/health",
					Port: intstr.FromInt(8080),
				},
			},
		}
	}
}

func createCustomProbe(t *webserversv1alpha1.WebServer, probeScript string) *corev1.Probe {
	// If the script has the following format: shell -c "command"
	// we create the slice ["shell", "-c", "command"]
	probeScriptSlice := make([]string, 0)
	pos := strings.Index(probeScript, "\"")
	if pos != -1 {
		probeScriptSlice = append(strings.Split(probeScript[0:pos], " "), probeScript[pos:])
	} else {
		probeScriptSlice = strings.Split(probeScript, " ")
	}
	return &corev1.Probe{
		Handler: corev1.Handler{
			Exec: &corev1.ExecAction{
				Command: probeScriptSlice,
			},
		},
	}
}

func isOpenShift(c *rest.Config) bool {
	var err error
	var dcclient *discovery.DiscoveryClient
	dcclient, err = discovery.NewDiscoveryClientForConfig(c)
	if err != nil {
		log.Info("isOpenShift discovery.NewDiscoveryClientForConfig has encountered a problem")
		return false
	}
	apiList, err := dcclient.ServerGroups()
	if err != nil {
		log.Info("isOpenShift client.ServerGroups has encountered a problem")
		return false
	}
	for _, v := range apiList.Groups {
		log.Info(v.Name)
		if v.Name == "route.openshift.io" {

			log.Info("route.openshift.io was found in apis, platform is OpenShift")
			return true
		}
	}
	return false
}

// GetPodsForWebServer lists pods which belongs to the Web server
// the pods are differentiated based on the selectors
func GetPodsForWebServer(r *ReconcileWebServer, j *webserversv1alpha1.WebServer) (*corev1.PodList, error) {
	podList := &corev1.PodList{}

	listOpts := []client.ListOption{
		client.InNamespace(j.Namespace),
		client.MatchingLabels(LabelsForWeb(j)),
	}
	err := r.client.List(context.TODO(), podList, listOpts...)

	if err == nil {
		// sorting pods by number in the name
		SortPodListByName(podList)
	}
	return podList, err
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
			log.Info("labels: ", labelKey, " : ", labelValue)
			labels[labelKey] = labelValue
		}
	}
	return labels
}

// SortPodListByName sorts the pod list by number in the name
//  expecting the format which the StatefulSet works with which is `<podname>-<number>`
func SortPodListByName(podList *corev1.PodList) *corev1.PodList {
	sort.SliceStable(podList.Items, func(i, j int) bool {
		return podList.Items[i].ObjectMeta.Name < podList.Items[j].ObjectMeta.Name
	})
	return podList
}

// UpdateWebServerStatus updates status of the WebServer resource.
func UpdateWebServerStatus(j *webserversv1alpha1.WebServer, client client.Client) error {
	logger := log.WithValues("WebServer.Namespace", j.Namespace, "WebServer.Name", j.Name)
	logger.Info("Updating the status of WebServer")

	if err := client.Status().Update(context.Background(), j); err != nil {
		logger.Error(err, "Failed to update the status of WebServer")
		return err
	}

	logger.Info("The status of WebServer was updated successfully")
	return nil
}

func UpdateStatus(j *webserversv1alpha1.WebServer, client client.Client, objectDefinition runtime.Object) error {
	logger := log.WithValues("WebServer.Namespace", j.Namespace, "WebServer.Name", j.Name)
	logger.Info("Updating the status of the resource")

	if err := client.Update(context.Background(), objectDefinition); err != nil {
		logger.Error(err, "Failed to update the status of the resource")
		return err
	}

	logger.Info("The status of the resource was updated successfully")
	return nil
}

// getPodStatus returns the pod names of the array of pods passed in
func getPodStatus(pods []corev1.Pod) (bool, []webserversv1alpha1.PodStatus) {
	var requeue = false
	var podStatuses []webserversv1alpha1.PodStatus
	for _, pod := range pods {
		podState := webserversv1alpha1.PodStateFailed

		switch pod.Status.Phase {
		case corev1.PodPending:
			podState = webserversv1alpha1.PodStatePending
		case corev1.PodRunning:
			podState = webserversv1alpha1.PodStateActive
		}

		podStatuses = append(podStatuses, webserversv1alpha1.PodStatus{
			Name:  pod.Name,
			PodIP: pod.Status.PodIP,
			State: podState,
		})
		if pod.Status.PodIP == "" {
			requeue = true
		}
	}
	return requeue, podStatuses
}

// Create the env for the pods we are starting.
func createEnvVars(t *webserversv1alpha1.WebServer, useKUBEPing bool) []corev1.EnvVar {
	value := "webserver-" + t.Name
	if useKUBEPing && t.Spec.UseSessionClustering {
		value = t.Namespace
	}
	env := []corev1.EnvVar{
		{
			Name:  "KUBERNETES_NAMESPACE",
			Value: value,
		},
	}
	if t.Spec.UseSessionClustering {
		// Add parameter USE_SESSION_CLUSTERING
		env = append(env, corev1.EnvVar{
			Name:  "ENV_FILES",
			Value: "/test/my-files/test.sh",
		})
	}
	return env
}

// Create the env for the maven build
func createEnvBuild(t *webserversv1alpha1.WebServer) []corev1.EnvVar {
	var env []corev1.EnvVar
	sources := t.Spec.WebImageStream.WebSources
	if sources != nil {
		params := sources.WebSourcesParams
		if params != nil {
			if params.MavenMirrorURL != "" {
				env = append(env, corev1.EnvVar{
					Name:  "MAVEN_MIRROR_URL",
					Value: params.MavenMirrorURL,
				})
			}
			if params.ArtifactDir != "" {
				env = append(env, corev1.EnvVar{
					Name:  "ARTIFACT_DIR",
					Value: params.ArtifactDir,
				})
			}
		}
	}
	return env
}

// Create the BuildTriggerPolicy
func createBuildTriggerPolicy(t *webserversv1alpha1.WebServer) []buildv1.BuildTriggerPolicy {
	env := []buildv1.BuildTriggerPolicy{
		{
			Type:        "ImageChange",
			ImageChange: &buildv1.ImageChangeTrigger{},
		},
		{
			Type: "ConfigChange",
		},
	}
	sources := t.Spec.WebImageStream.WebSources
	if sources != nil {
		params := sources.WebSourcesParams
		if params != nil {
			if params.GithubWebhookSecret != "" {
				env = append(env, buildv1.BuildTriggerPolicy{
					Type: "GitHub",
					GitHubWebHook: &buildv1.WebHookTrigger{
						Secret: params.GithubWebhookSecret,
					},
				})
			}
			if params.GenericWebhookSecret != "" {
				env = append(env, buildv1.BuildTriggerPolicy{
					Type: "Generic",
					GenericWebHook: &buildv1.WebHookTrigger{
						Secret: params.GenericWebhookSecret,
					},
				})
			}
		}
	}
	return env
}

// Create the VolumeMounts
func createVolumeMounts(t *webserversv1alpha1.WebServer) []corev1.VolumeMount {
	if t.Spec.UseSessionClustering {
		volm := []corev1.VolumeMount{{
			Name:      "webserver-" + t.Name,
			MountPath: "/test/my-files",
		}}
		return volm
	}
	return nil
}

// Create the Volumes
func createVolumes(t *webserversv1alpha1.WebServer) []corev1.Volume {
	if t.Spec.UseSessionClustering {
		vol := []corev1.Volume{{
			Name: "webserver-" + t.Name,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "webserver-" + t.Name,
					},
				},
			},
		}}
		return vol
	}
	return nil
}
