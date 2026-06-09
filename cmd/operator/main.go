/*
Copyright 2026 Sandlock Authors.

Use of this software is governed by the Business Source License 1.1 included
in the LICENSE file.
*/

package main

import (
	"flag"
	"os"

	sandlockv1alpha1 "github.com/sandlock/k8s-agent-platform/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(sandlockv1alpha1.AddToScheme(scheme))
	_ = corev1.AddToScheme(scheme)
}

func main() {
	var metricsAddr string
	var probeAddr string
	var supervisorImage string
	var runtimeClass string

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8383", "Address for the metrics endpoint.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8384", "Address for health probes.")
	flag.StringVar(&supervisorImage, "supervisor-image", "sandlock/supervisor:latest", "Container image for the agent supervisor.")
	flag.StringVar(&runtimeClass, "runtime-class", "", "RuntimeClass for sandbox pods (e.g. gvisor). Empty = default.")
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&zap.Options{})))
	log := ctrl.Log.WithName("operator")

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsserver.Options{BindAddress: metricsAddr},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         false,
	})
	if err != nil {
		log.Error(err, "unable to create manager")
		os.Exit(1)
	}

	if err := (&SandboxReconciler{
		Client:          mgr.GetClient(),
		Scheme:          mgr.GetScheme(),
		SupervisorImage: supervisorImage,
		RuntimeClass:    runtimeClass,
	}).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to set up Sandbox controller")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		log.Error(err, "unable to add healthz check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		log.Error(err, "unable to add readyz check")
		os.Exit(1)
	}

	log.Info("starting operator")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		log.Error(err, "manager exited")
		os.Exit(1)
	}
}
