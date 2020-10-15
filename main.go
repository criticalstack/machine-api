/*


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

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	machinev1alpha1 "github.com/criticalstack/machine-api/api/v1alpha1"
	configcontroller "github.com/criticalstack/machine-api/controllers/config"
	csrapprovercontroller "github.com/criticalstack/machine-api/controllers/csrapprover"
	infraprovidercontroller "github.com/criticalstack/machine-api/controllers/infraprovider"
	machinecontroller "github.com/criticalstack/machine-api/controllers/machine"
	nodecontroller "github.com/criticalstack/machine-api/controllers/node"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)

	_ = machinev1alpha1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var configConcurrency int
	var machineConcurrency int
	var nodeConcurrency int
	var csrApproverConcurreny int
	var infraProviderConcurrency int
	var externalReadyWait time.Duration
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.IntVar(&configConcurrency, "config-concurrency", 10,
		"Number of configs to process simultaneously")
	flag.IntVar(&machineConcurrency, "machine-concurrency", 10,
		"Number of machines to process simultaneously")
	flag.IntVar(&nodeConcurrency, "node-concurrency", 10,
		"Number of nodes to process simultaneously")
	flag.IntVar(&csrApproverConcurreny, "csrapprover-concurrency", 10,
		"Number of csrs to process simultaneously")
	flag.IntVar(&infraProviderConcurrency, "infraprovider-concurrency", 10,
		"Number of infraproviders to process simultaneously")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.DurationVar(&externalReadyWait, "external-ready-wait", 30*time.Second,
		"Amount of time to wait between polls for external resources to be ready")
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: metricsAddr,
		Port:               9443,
		LeaderElection:     enableLeaderElection,
		LeaderElectionID:   "78c2e11e.crit.sh",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&configcontroller.ConfigReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("Config"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr, controller.Options{MaxConcurrentReconciles: configConcurrency}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Config")
		os.Exit(1)
	}
	if err = (&machinecontroller.MachineReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("Machine"),
	}).SetupWithManager(mgr, controller.Options{MaxConcurrentReconciles: machineConcurrency}, externalReadyWait); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Machine")
		os.Exit(1)
	}
	if err = (&nodecontroller.NodeReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("Node"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr, controller.Options{MaxConcurrentReconciles: nodeConcurrency}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Node")
		os.Exit(1)
	}
	if err = (&csrapprovercontroller.CSRApproverReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("CSRApprover"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr, controller.Options{MaxConcurrentReconciles: nodeConcurrency}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "CSRApprover")
		os.Exit(1)
	}
	if err = (&infraprovidercontroller.InfrastructureProviderReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("InfrastructureProvider"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr, controller.Options{MaxConcurrentReconciles: infraProviderConcurrency}, externalReadyWait); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "InfrastructureProvider")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
