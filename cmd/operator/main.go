package main

import (
	"flag"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	adminclient "helix/internal/admin/client"
	"helix/internal/operator/controller"
	helixv1 "helix/internal/operator/types"
)

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(helixv1.AddToScheme(scheme))
}

func main() {
	var (
		metricsAddr string
		probeAddr   string
		leaderElect bool
		adminURL    string
		adminUser   string
		adminPass   string
	)

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "metrics endpoint address")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "health probe address")
	flag.BoolVar(&leaderElect, "leader-elect", true, "enable leader election")
	flag.StringVar(&adminURL, "admin-url", envOrDefault("HELIX_ADMIN_URL", "http://helix-gateway:9090"), "helix admin API URL")
	flag.StringVar(&adminUser, "admin-user", envOrDefault("HELIX_ADMIN_USER", "admin"), "admin API user")
	flag.StringVar(&adminPass, "admin-password", os.Getenv("HELIX_ADMIN_PASSWORD"), "admin API password")
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(false)))
	logger := ctrl.Log.WithName("helix-operator")

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: metricsAddr,
		},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         leaderElect,
		LeaderElectionID:       "helix.io",
	})
	if err != nil {
		logger.Error(err, "unable to start manager")
		os.Exit(1)
	}

	admin := adminclient.New(adminURL, adminUser, adminPass)

	if err := (&controller.GatewayUpstreamReconciler{
		Client: mgr.GetClient(),
		Admin:  admin,
	}).SetupWithManager(mgr); err != nil {
		logger.Error(err, "unable to create GatewayUpstream controller")
		os.Exit(1)
	}

	if err := (&controller.GatewayRouteReconciler{
		Client: mgr.GetClient(),
		Admin:  admin,
	}).SetupWithManager(mgr); err != nil {
		logger.Error(err, "unable to create GatewayRoute controller")
		os.Exit(1)
	}

	if err := (&controller.HelixIPPolicyReconciler{
		Client: mgr.GetClient(),
		Admin:  admin,
	}).SetupWithManager(mgr); err != nil {
		logger.Error(err, "unable to create HelixIPPolicy controller")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		logger.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		logger.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	logger.Info("starting helix operator", "admin_url", adminURL)
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		logger.Error(err, "manager exited with error")
		os.Exit(1)
	}
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
