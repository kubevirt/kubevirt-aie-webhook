package main

import (
	"context"
	"os"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	kubevirtv1 "kubevirt.io/api/core/v1"

	"kubevirt.io/kubevirt-aie-webhook/pkg/config"
	webhookpkg "kubevirt.io/kubevirt-aie-webhook/pkg/webhook"
)

const (
	configMapName = "kubevirt-aie-launcher-config"
	configDataKey = "config.yaml"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(kubevirtv1.AddToScheme(scheme))
}

// configMapReconciler watches the launcher config ConfigMap and updates the store.
type configMapReconciler struct {
	client client.Client
	store  *config.ConfigStore
	ns     string
}

func (r *configMapReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	if req.Name != configMapName || req.Namespace != r.ns {
		return reconcile.Result{}, nil
	}

	var cm corev1.ConfigMap
	if err := r.client.Get(ctx, req.NamespacedName, &cm); err != nil {
		log.Error(err, "unable to fetch ConfigMap")
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	data, ok := cm.Data[configDataKey]
	if !ok {
		log.Info("ConfigMap missing data key", "key", configDataKey)
		return reconcile.Result{}, nil
	}

	if err := r.store.Update([]byte(data)); err != nil {
		log.Error(err, "failed to parse launcher config")
		return reconcile.Result{}, err
	}

	log.Info("launcher config updated")
	return reconcile.Result{}, nil
}

func main() {
	ctrl.SetLogger(zap.New(zap.UseDevMode(false)))

	namespace := os.Getenv("NAMESPACE")
	if namespace == "" {
		namespace = "kubevirt"
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: ":8080",
		},
		HealthProbeBindAddress: ":8081",
		WebhookServer: webhook.NewServer(webhook.Options{
			Port: 9443,
		}),
	})
	if err != nil {
		setupLog.Error(err, "unable to create manager")
		os.Exit(1)
	}

	store := config.NewConfigStore()

	// Set up ConfigMap watcher
	cmReconciler := &configMapReconciler{
		client: mgr.GetClient(),
		store:  store,
		ns:     namespace,
	}
	if err := ctrl.NewControllerManagedBy(mgr).
		Named("configmap-watcher").
		Watches(&corev1.ConfigMap{}, handler.EnqueueRequestsFromMapFunc(
			func(ctx context.Context, obj client.Object) []reconcile.Request {
				if obj.GetName() != configMapName || obj.GetNamespace() != namespace {
					return nil
				}
				return []reconcile.Request{{
					NamespacedName: types.NamespacedName{
						Name:      obj.GetName(),
						Namespace: obj.GetNamespace(),
					},
				}}
			},
		)).
		Complete(cmReconciler); err != nil {
		setupLog.Error(err, "unable to create ConfigMap controller")
		os.Exit(1)
	}

	// Register mutating webhook handler
	mgr.GetWebhookServer().Register("/mutate-pods", &webhook.Admission{
		Handler: &webhookpkg.VirtLauncherMutator{
			Client:  mgr.GetClient(),
			Store:   store,
			Decoder: admission.NewDecoder(scheme),
		},
	})

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
