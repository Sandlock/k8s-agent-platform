/*
Copyright 2026 Sandlock Authors.

Use of this software is governed by the Business Source License 1.1 included
in the LICENSE file.
*/

package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/sandlock/k8s-agent-platform/internal/api"
	"github.com/sandlock/k8s-agent-platform/internal/db"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	agentv1alpha1 "sigs.k8s.io/agent-sandbox/api/v1alpha1"
	extensionsv1alpha1 "sigs.k8s.io/agent-sandbox/extensions/api/v1alpha1"
	ctrlconfig "sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(agentv1alpha1.AddToScheme(scheme))
	utilruntime.Must(extensionsv1alpha1.AddToScheme(scheme))
	_ = corev1.AddToScheme(scheme)
}

func main() {
	addr := getenv("LISTEN_ADDR", ":8090")
	dbURL := os.Getenv("DATABASE_URL")
	sandboxNS := getenv("SANDBOX_NAMESPACE", "sandboxes")

	cfg, err := ctrlconfig.GetConfig()
	if err != nil {
		log.Fatalf("kubeconfig: %v", err)
	}
	k8sClient, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		log.Fatalf("k8s client: %v", err)
	}

	var srv *api.Server
	if dbURL != "" {
		dbPool, err := db.Open(context.Background(), dbURL)
		if err != nil {
			log.Fatalf("db: %v", err)
		}
		srv = api.NewServer(k8sClient, sandboxNS, dbPool)
	} else {
		log.Println("DATABASE_URL not set — running without DB (no auth, in-memory sandbox store)")
		srv = api.NewServer(k8sClient, sandboxNS, nil)
	}

	log.Printf("control plane listening on %s", addr)
	if err := http.ListenAndServe(addr, srv.Handler()); err != nil {
		log.Fatalf("server: %v", err)
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
