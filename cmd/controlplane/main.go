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
	"time"

	"github.com/sandlock/k8s-agent-platform/internal/api"
	"github.com/sandlock/k8s-agent-platform/internal/auth"
	"github.com/sandlock/k8s-agent-platform/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	agentv1alpha1 "sigs.k8s.io/agent-sandbox/api/v1alpha1"
	extensionsv1alpha1 "sigs.k8s.io/agent-sandbox/extensions/api/v1alpha1"
	extensionsv1beta1 "sigs.k8s.io/agent-sandbox/extensions/api/v1beta1"
	ctrlconfig "sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(agentv1alpha1.AddToScheme(scheme))
	utilruntime.Must(extensionsv1alpha1.AddToScheme(scheme))
	utilruntime.Must(extensionsv1beta1.AddToScheme(scheme))
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

	selfURL := os.Getenv("SELF_URL")

	var srv *api.Server
	if dbURL != "" {
		dbPool, err := db.Open(context.Background(), dbURL)
		if err != nil {
			log.Fatalf("db: %v", err)
		}
		seedAdmin(context.Background(), dbPool)
		srv = api.NewServer(k8sClient, sandboxNS, dbPool, selfURL)
		go srv.RunReconciler(context.Background(), 30*time.Second)
	} else {
		log.Println("DATABASE_URL not set — running without DB (no auth, in-memory sandbox store)")
		srv = api.NewServer(k8sClient, sandboxNS, nil, selfURL)
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

// seedAdmin creates the admin user on first boot if no users exist.
func seedAdmin(ctx context.Context, pool *pgxpool.Pool) {
	var count int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&count); err != nil || count > 0 {
		return
	}
	password := os.Getenv("ADMIN_PASSWORD")
	if password == "" {
		log.Println("ADMIN_PASSWORD not set — skipping admin seed")
		return
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		log.Printf("seed admin: hash password: %v", err)
		return
	}
	_, err = pool.Exec(ctx,
		`INSERT INTO users(username, password_hash, is_admin, must_change_password)
		 VALUES('admin', $1, true, true)`,
		hash,
	)
	if err != nil {
		log.Printf("seed admin: insert: %v", err)
		return
	}
	log.Println("admin user created — password is in the sandlock-admin Secret")
}
