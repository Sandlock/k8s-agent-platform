package api

import (
	"context"
	"log"
	"time"

	extensionsv1alpha1 "sigs.k8s.io/agent-sandbox/extensions/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// RunReconciler periodically checks that every non-gone sandbox in the DB
// still has a live SandboxClaim. If the claim is gone, the sandbox is marked
// gone in the DB. Runs until ctx is cancelled.
func (s *Server) RunReconciler(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.reconcileOnce(ctx)
		}
	}
}

func (s *Server) reconcileOnce(ctx context.Context) {
	if s.db == nil {
		return
	}

	// Fetch all running sandboxes from DB.
	rows, err := s.db.Query(ctx,
		`SELECT id, provider_ref FROM sandboxes WHERE status = 'running'`)
	if err != nil {
		log.Printf("reconciler: query: %v", err)
		return
	}
	defer rows.Close()

	type entry struct{ id, ref string }
	var entries []entry
	for rows.Next() {
		var e entry
		if err := rows.Scan(&e.id, &e.ref); err == nil {
			entries = append(entries, e)
		}
	}
	rows.Close()

	if len(entries) == 0 {
		return
	}

	// Build a set of existing SandboxClaim names.
	var claimList extensionsv1alpha1.SandboxClaimList
	if err := s.k8s.List(ctx, &claimList, &client.ListOptions{}); err != nil {
		log.Printf("reconciler: list claims: %v", err)
		return
	}
	existing := make(map[string]struct{}, len(claimList.Items))
	for _, c := range claimList.Items {
		existing[c.Namespace+"/"+c.Name] = struct{}{}
	}

	// Mark gone any sandbox whose claim no longer exists.
	for _, e := range entries {
		_, ns, name, ok := parseClaimRef(e.ref)
		if !ok {
			continue
		}
		if _, found := existing[ns+"/"+name]; !found {
			_, err := s.db.Exec(ctx,
				`UPDATE sandboxes SET status='gone' WHERE id=$1`, e.id)
			if err != nil {
				log.Printf("reconciler: mark gone %s: %v", e.id, err)
			} else {
				log.Printf("reconciler: marked %s gone (claim %s/%s not found)", e.id, ns, name)
			}
		}
	}
}
