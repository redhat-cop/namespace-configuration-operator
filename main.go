/*
Copyright 2020 Red Hat Community of Practice.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	userv1 "github.com/openshift/api/user/v1"
	"go.uber.org/zap/zapcore"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	redhatcopv1alpha1 "github.com/redhat-cop/namespace-configuration-operator/api/v1alpha1"
	"github.com/redhat-cop/namespace-configuration-operator/controllers"
	"github.com/redhat-cop/namespace-configuration-operator/internal/version"
	"github.com/redhat-cop/operator-utils/pkg/util/discoveryclient"
	"github.com/redhat-cop/operator-utils/pkg/util/lockedresourcecontroller"
	// +kubebuilder:scaffold:imports
)

const (
	AllowSystemNamespacesEnvVarKey = "ALLOW_SYSTEM_NAMESPACES"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(redhatcopv1alpha1.AddToScheme(scheme))
	utilruntime.Must(userv1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var showVersion bool
	flag.BoolVar(&showVersion, "version", false, "Print the version banner and exit.")
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	// Configure zap logger options
	// See: https://sdk.operatorframework.io/docs/building-operators/golang/references/logging/
	opts := zap.Options{
		Development: true,
	}

	// Support environment variables for containerized deployments
	// These can be set in Kubernetes Deployment env section or ConfigMap
	// Note: Official SDK recommendation is to use --zap-* flags in container args,
	// but environment variables provide more flexibility for ConfigMap-based configuration
	// Priority: Command line flags > Environment variables > Defaults
	if zapLogLevel := os.Getenv("ZAP_LOG_LEVEL"); zapLogLevel != "" {
		// Parse log level from environment variable
		// Valid values: "error", "info", "debug", or integer "0"-"10"
		// See: https://sdk.operatorframework.io/docs/building-operators/golang/references/logging/
		var level zapcore.Level
		if err := level.UnmarshalText([]byte(zapLogLevel)); err == nil {
			// Successfully parsed as string ("error", "info", "debug")
			opts.Level = level
		} else {
			// Try parsing as integer for custom debug levels
			// Integer values > 0 correspond to custom debug levels of increasing verbosity
			if intLevel, err := strconv.Atoi(zapLogLevel); err == nil && intLevel >= 0 {
				// For custom debug levels, use negative values (zap convention)
				// Note: zap.Options.Level uses zapcore.Level which can be negative for debug
				opts.Level = zapcore.Level(-intLevel)
			}
		}
	}

	// Check for ZAP_DEVEL environment variable (true/false)
	// Development mode: console encoder, debug level, stacktraces on warnings
	// Production mode: JSON encoder, info level, stacktraces on errors
	if zapDevel := os.Getenv("ZAP_DEVEL"); zapDevel != "" {
		if zapDevel == "false" || zapDevel == "0" {
			opts.Development = false
		} else if zapDevel == "true" || zapDevel == "1" {
			opts.Development = true
		}
	}

	// Bind zap flags to command line (--zap-log-level, --zap-devel, etc.)
	// Flags take precedence over environment variables
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	// Log level can be controlled via (in order of precedence):
	// 1. Command line flags: --zap-log-level=info --zap-devel=false (highest priority)
	//    Recommended for cluster deployments: use args in Deployment spec
	// 2. Environment variables: ZAP_LOG_LEVEL and ZAP_DEVEL (for ConfigMap-based config)
	// 3. Defaults: Development=true, Level=Debug

	// Print startup banner with version and commit info. With --version that is all the process does;
	// it lets a pulled image be asked what it is without a kubeconfig (hack/ci-image.sh pull).
	version.PrintStartupBanner()
	if showVersion {
		os.Exit(0)
	}

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// Objects this operator created before the enforcer applied server-side carry one managedFields
	// entry, manager "manager" (this binary's name, taken from the user agent), operation Update; the
	// library folds that entry into its apply manager on first contact so fields a template no longer
	// renders are removed. Named explicitly: a build under another name (`make run` records "main", a
	// renamed image its own name) must still recognise the objects a production binary wrote; objects
	// written by such a build are not folded, and keep their stale fields (review of PR #40).
	lockedresourcecontroller.LegacyFieldManagers = []string{"manager"}

	syncPeriod, err := parseSyncPeriod(os.Getenv("SYNC_PERIOD_SECONDS"))
	if err != nil {
		setupLog.Error(err, "invalid SYNC_PERIOD_SECONDS")
		os.Exit(1)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "b0b2f089.redhat.io",
		SyncPeriod:             &syncPeriod,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&controllers.NamespaceConfigReconciler{
		EnforcingReconciler:   lockedresourcecontroller.NewEnforcingReconciler(mgr.GetClient(), mgr.GetScheme(), mgr.GetConfig(), mgr.GetAPIReader(), mgr.GetEventRecorderFor("NamespaceConfig_controller"), true, true),
		Log:                   ctrl.Log.WithName("controllers").WithName("NamespaceConfig"),
		AllowSystemNamespaces: checkNamespaceScope(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "NamespaceConfig")
		os.Exit(1)
	}
	ctx := context.WithValue(context.TODO(), "restConfig", mgr.GetConfig())

	userConfigController := &controllers.UserConfigReconciler{
		EnforcingReconciler: lockedresourcecontroller.NewEnforcingReconciler(mgr.GetClient(), mgr.GetScheme(), mgr.GetConfig(), mgr.GetAPIReader(), mgr.GetEventRecorderFor("UserConfig_controller"), true, true),
		Log:                 ctrl.Log.WithName("controllers").WithName("UserConfig"),
	}

	if ok, err := discoveryclient.IsGVKDefined(ctx, schema.GroupVersionKind{
		Group:   "user.openshift.io",
		Version: "v1",
		Kind:    "User",
	}); !ok || err != nil {
		if err != nil {
			setupLog.Error(err, "unable to check whether resource User.user.openshift.io exists")
			os.Exit(1)
		}
		setupLog.Info("User.user.openshift.io is not served by this cluster; the UserConfig controller is not started")
	} else {
		if err = (userConfigController).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "UserConfig")
			os.Exit(1)
		}
	}

	groupConfigController := &controllers.GroupConfigReconciler{
		EnforcingReconciler: lockedresourcecontroller.NewEnforcingReconciler(mgr.GetClient(), mgr.GetScheme(), mgr.GetConfig(), mgr.GetAPIReader(), mgr.GetEventRecorderFor("GroupConfig_controller"), true, true),
		Log:                 ctrl.Log.WithName("controllers").WithName("GroupConfig"),
	}

	if ok, err := discoveryclient.IsGVKDefined(ctx, schema.GroupVersionKind{
		Group:   "user.openshift.io",
		Version: "v1",
		Kind:    "Group",
	}); !ok || err != nil {
		if err != nil {
			setupLog.Error(err, "unable to check whether resource Group.user.openshift.io exists")
			os.Exit(1)
		}
		setupLog.Info("Group.user.openshift.io is not served by this cluster; the GroupConfig controller is not started")
	} else {
		if err = (groupConfigController).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "GroupConfig")
			os.Exit(1)
		}
	}
	// +kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("health", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("check", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func checkNamespaceScope() bool {
	allow, err := parseAllowSystemNamespaces(os.Getenv(AllowSystemNamespacesEnvVarKey))
	if err != nil {
		// Not fatal: the safe default (system namespaces excluded) applies, but say so, because a
		// typo here used to be indistinguishable from "false".
		setupLog.Error(err, "ignoring invalid "+AllowSystemNamespacesEnvVarKey+"; system namespaces stay excluded")
	}
	return allow
}

// parseSyncPeriod reads SYNC_PERIOD_SECONDS: empty means the default (every 10 hours); otherwise a
// positive integer number of seconds.
func parseSyncPeriod(value string) (time.Duration, error) {
	const defaultSyncPeriod = 36000 * time.Second
	if value == "" {
		return defaultSyncPeriod, nil
	}
	seconds, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("SYNC_PERIOD_SECONDS must be an integer number of seconds, got %q: %w", value, err)
	}
	if seconds <= 0 {
		return 0, fmt.Errorf("SYNC_PERIOD_SECONDS must be positive, got %d", seconds)
	}
	return time.Duration(seconds) * time.Second, nil
}

// parseAllowSystemNamespaces reads ALLOW_SYSTEM_NAMESPACES: empty means false; anything else must
// parse as a bool. A parse error also yields false, with the error returned so it can be logged.
func parseAllowSystemNamespaces(value string) (bool, error) {
	if value == "" {
		return false, nil
	}
	allow, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%s must be a boolean, got %q: %w", AllowSystemNamespacesEnvVarKey, value, err)
	}
	return allow, nil
}
