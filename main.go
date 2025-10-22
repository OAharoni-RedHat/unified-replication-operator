/*
Copyright 2024.

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
	"flag"
	"os"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	replicationv1alpha1 "github.com/unified-replication/operator/api/v1alpha1"
	"github.com/unified-replication/operator/controllers"
	"github.com/unified-replication/operator/pkg"
	"github.com/unified-replication/operator/pkg/adapters"
	"github.com/unified-replication/operator/pkg/discovery"
	"github.com/unified-replication/operator/pkg/translation"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(apiextensionsv1.AddToScheme(scheme))

	utilruntime.Must(replicationv1alpha1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		// Leader election disabled - single replica deployment only
		LeaderElection: false,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// Initialize components
	translationEngine := translation.NewEngine()
	discoveryEngine := discovery.NewEngine(mgr.GetClient(), discovery.DefaultDiscoveryConfig())

	// Initialize adapter registry
	adapterRegistry := adapters.NewRegistry()
	adapterRegistry.RegisterFactory(adapters.NewCephAdapterFactory())
	adapterRegistry.RegisterFactory(adapters.NewTridentAdapterFactory())
	adapterRegistry.RegisterFactory(adapters.NewPowerStoreAdapterFactory())

	// Initialize controller engine
	controllerEngine := pkg.NewControllerEngine(mgr.GetClient(), discoveryEngine, translationEngine, adapterRegistry, pkg.DefaultControllerEngineConfig())

	// Initialize advanced features
	stateMachine := controllers.NewStateMachine()
	retryManager := controllers.NewRetryManager(&controllers.RetryStrategy{
		MaxAttempts:  5,
		InitialDelay: 1 * time.Second,
		MaxDelay:     5 * time.Minute,
		Multiplier:   2.0,
	})
	circuitBreaker := controllers.NewCircuitBreaker(5, 2, 60*time.Second)

	// Setup the UnifiedVolumeReplication controller
	if err = (&controllers.UnifiedVolumeReplicationReconciler{
		Client:                  mgr.GetClient(),
		Log:                     ctrl.Log.WithName("controllers").WithName("UnifiedVolumeReplication"),
		Scheme:                  mgr.GetScheme(),
		Recorder:                mgr.GetEventRecorderFor("unified-replication-operator"),
		AdapterRegistry:         adapterRegistry,
		DiscoveryEngine:         discoveryEngine,
		TranslationEngine:       translationEngine,
		ControllerEngine:        controllerEngine,
		StateMachine:            stateMachine,
		RetryManager:            retryManager,
		CircuitBreaker:          circuitBreaker,
		MaxConcurrentReconciles: 3,
		ReconcileTimeout:        5 * time.Minute,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "UnifiedVolumeReplication")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
