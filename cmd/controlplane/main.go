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

	sandlockv1alpha1 "github.com/sandlock/k8s-agent-platform/api/v1alpha1"
	"github.com/sandlock/k8s-agent-platform/internal/api"
	"github.com/sandlock/k8s-agent-platform/internal/db"
	"github.com/sandlock/k8s-agent-platform/internal/pool"
	"github.com/sandlock/k8s-agent-platform/internal/provider"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrlconfig "sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(sandlockv1alpha1.AddToScheme(scheme))
	_ = corev1.AddToScheme(scheme)
}

func main() {
	addr := getenv("LISTEN_ADDR", ":8090")
	dbURL := os.Getenv("DATABASE_URL")

	cfg, err := ctrlconfig.GetConfig()
	if err != nil {
		log.Fatalf("kubeconfig: %v", err)
	}
	k8sClient, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		log.Fatalf("k8s client: %v", err)
	}

	prov := provider.NewKubernetesProvider(k8sClient)

	poolMgr := pool.New(k8sClient, prov)
	go poolMgr.Run(context.Background())

	var srv *api.Server
	if dbURL != "" {
		dbPool, err := db.Open(context.Background(), dbURL)
		if err != nil {
			log.Fatalf("db: %v", err)
		}
		srv = api.NewServer(k8sClient, prov, poolMgr, dbPool)
	} else {
		log.Println("DATABASE_URL not set — running without DB (no auth, in-memory sandbox store)")
		srv = api.NewServer(k8sClient, prov, poolMgr, nil)
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
