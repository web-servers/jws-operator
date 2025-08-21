package controller

import (
	"context"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	webserversv1alpha1 "github.com/web-servers/jws-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// GetOrCreateNewServiceMonitor either returns the headless service or create it
func (r *WebServerReconciler) GetOrCreateNewServiceMonitor(w *webserversv1alpha1.WebServer, ctx context.Context, labels map[string]string) (*monitoringv1.ServiceMonitor, error) {
	serviceMonitor := &monitoringv1.ServiceMonitor{}
	if err := r.Client.Get(ctx, client.ObjectKey{
		Namespace: w.Namespace,
		Name:      w.Name,
	}, serviceMonitor); err != nil {
		if errors.IsNotFound(err) {
			if err := r.Client.Create(ctx, r.generateServiceMonitor(w, labels)); err != nil {
				if errors.IsAlreadyExists(err) {
					return nil, nil
				}
				return nil, err
			}
			return nil, nil
		}
	}
	return serviceMonitor, nil
}

func (r *WebServerReconciler) generateServiceMonitor(w *webserversv1alpha1.WebServer, labels map[string]string) *monitoringv1.ServiceMonitor {
	service := &monitoringv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      w.Name,
			Namespace: w.Namespace,
			Labels:    labels,
		},
		Spec: monitoringv1.ServiceMonitorSpec{
			Endpoints: []monitoringv1.Endpoint{{
				Port: "admin",
			}},
			Selector: metav1.LabelSelector{
				MatchLabels: labels,
			},
		},
	}
	controllerutil.SetControllerReference(w, service, r.Scheme)
	return service
}

// hasServiceMonitor checks if ServiceMonitor kind is registered in the cluster.
func hasServiceMonitor(c *rest.Config) bool {
	return CustomResourceDefinitionExists(schema.GroupVersionKind{
		Group:   "monitoring.coreos.com",
		Version: monitoringv1.Version,
		Kind:    monitoringv1.ServiceMonitorsKind,
	}, c)
}
