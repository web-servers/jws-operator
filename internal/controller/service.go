package controller

import (
	"context"

	webserversv1alpha1 "github.com/web-servers/jws-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// GetOrCreateNewPrometheusService either returns the headless service or create
func (r *WebServerReconciler) GetOrCreateNewPrometheusService(w *webserversv1alpha1.WebServer, ctx context.Context, labels map[string]string) (*corev1.Service, error) {
	service := &corev1.Service{}
	if err := r.Get(ctx, client.ObjectKey{
		Namespace: w.Namespace,
		Name:      PrometeusServiceName(w),
	}, service); err != nil {
		if errors.IsNotFound(err) {
			if err := r.Create(ctx, r.generatePrometeusService(w, labels)); err != nil {
				if errors.IsAlreadyExists(err) {
					return nil, nil
				}
				return nil, err
			}
			return nil, nil
		}
	}
	return service, nil
}

// generatePrometeusService returns a service exposing the prometheus port of WebServer
// Like the newAdminService of wildfly operator
func (r *WebServerReconciler) generatePrometeusService(w *webserversv1alpha1.WebServer, labels map[string]string) *corev1.Service {
	headlessService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      PrometeusServiceName(w),
			Namespace: w.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Type:      corev1.ServiceTypeClusterIP,
			Selector:  labels,
			ClusterIP: corev1.ClusterIPNone,
			Ports: []corev1.ServicePort{
				{
					Name: "admin",
					Port: 9404,
				},
			},
		},
	}

	err := controllerutil.SetControllerReference(w, headlessService, r.Scheme)
	if err != nil {
		log.Error(err, "SetControllerReference was not successful")
	}

	return headlessService
}

// PrometeusServiceName returns the name of prometeus admin service
func PrometeusServiceName(w *webserversv1alpha1.WebServer) string {
	return w.Name + "-admin"
}
