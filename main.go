package main

import (
	"context"
	"crypto/tls"
	"flag"
	"os"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
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

//nolint:funlen
func main() {
	var (
		metricsAddr                                        string
		metricsCertPath, metricsCertName, metricsCertKey   string
		webhookCertPath, webhookCertName, webhookCertKey   string
		probeAddr                                          string
		secureMetrics                                      bool
		enableHTTP2                                        bool
		tlsOpts                                            []func(*tls.Config)
	)

	flag.StringVar(&metricsAddr, "metrics-bind-address", "0", "The address the metrics endpoint binds to. "+
		"Use :8443 for HTTPS or :8080 for HTTP, or leave as 0 to disable the metrics service.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&secureMetrics, "metrics-secure", true,
		"If set, the metrics endpoint is served securely via HTTPS. Use --metrics-secure=false to use HTTP instead.")
	flag.StringVar(&webhookCertPath, "webhook-cert-path", "", "The directory that contains the webhook certificate.")
	flag.StringVar(&webhookCertName, "webhook-cert-name", "tls.crt", "The name of the webhook certificate file.")
	flag.StringVar(&webhookCertKey, "webhook-cert-key", "tls.key", "The name of the webhook key file.")
	flag.StringVar(&metricsCertPath, "metrics-cert-path", "",
		"The directory that contains the metrics server certificate.")
	flag.StringVar(&metricsCertName, "metrics-cert-name", "tls.crt", "The name of the metrics server certificate file.")
	flag.StringVar(&metricsCertKey, "metrics-cert-key", "tls.key", "The name of the metrics server key file.")
	flag.BoolVar(&enableHTTP2, "enable-http2", false,
		"If set, HTTP/2 will be enabled for the metrics and webhook servers")

	opts := zap.Options{}
	opts.BindFlags(flag.CommandLine)

	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	namespace := os.Getenv("NAMESPACE")
	if namespace == "" {
		namespace = "kubevirt"
	}

	// if the enable-http2 flag is false (the default), http/2 should be disabled
	// due to its vulnerabilities. More specifically, disabling http/2 will
	// prevent from being vulnerable to the HTTP/2 Stream Cancellation and
	// Rapid Reset CVEs. For more information see:
	// - https://github.com/advisories/GHSA-qppj-fm5r-hxr3
	// - https://github.com/advisories/GHSA-4374-p667-p6c8
	disableHTTP2 := func(c *tls.Config) {
		setupLog.Info("disabling http/2")
		c.NextProtos = []string{"http/1.1"}
	}

	if !enableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	webhookTLSOpts := tlsOpts
	webhookServerOptions := webhook.Options{
		TLSOpts: webhookTLSOpts,
	}

	if webhookCertPath != "" {
		setupLog.Info("Initializing webhook certificate watcher using provided certificates",
			"webhook-cert-path", webhookCertPath, "webhook-cert-name", webhookCertName, "webhook-cert-key", webhookCertKey)

		webhookServerOptions.CertDir = webhookCertPath
		webhookServerOptions.CertName = webhookCertName
		webhookServerOptions.KeyName = webhookCertKey
	}

	webhookServer := webhook.NewServer(webhookServerOptions)

	metricsServerOptions := metricsserver.Options{
		BindAddress:   metricsAddr,
		SecureServing: secureMetrics,
		TLSOpts:       tlsOpts,
	}

	if secureMetrics {
		metricsServerOptions.FilterProvider = filters.WithAuthenticationAndAuthorization
	}

	if metricsCertPath != "" {
		setupLog.Info("Initializing metrics certificate watcher using provided certificates",
			"metrics-cert-path", metricsCertPath, "metrics-cert-name", metricsCertName, "metrics-cert-key", metricsCertKey)

		metricsServerOptions.CertDir = metricsCertPath
		metricsServerOptions.CertName = metricsCertName
		metricsServerOptions.KeyName = metricsCertKey
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Cache: cache.Options{
			ReaderFailOnMissingInformer: true,
			ByObject: map[client.Object]cache.ByObject{
				&corev1.ConfigMap{}: {
					Field: fields.SelectorFromSet(fields.Set{
						"metadata.name": configMapName,
					}),
					Namespaces: map[string]cache.Config{
						namespace: {},
					},
				},
			},
		},
		Metrics:                metricsServerOptions,
		HealthProbeBindAddress: probeAddr,
		WebhookServer:          webhookServer,
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

	// Register mutating webhook handler using the manager's API reader for
	// VMI lookups. The manager's default client reads through the cache, which
	// only has an informer for ConfigMaps. With ReaderFailOnMissingInformer
	// enabled, any Get/List for a type without a running informer (such as
	// VirtualMachineInstance) will fail rather than lazily starting one.
	mgr.GetWebhookServer().Register("/mutate-pods", &webhook.Admission{
		Handler: &webhookpkg.VirtLauncherMutator{
			Client:  mgr.GetAPIReader(),
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
