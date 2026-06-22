-- Add branch column to sandboxes so snapshot lookups can key on it.
ALTER TABLE sandboxes ADD COLUMN IF NOT EXISTS branch TEXT NOT NULL DEFAULT '';

-- Re-key agent_snapshots unique constraint to include branch so each
-- (repo, branch) pair gets its own independent session snapshot.
ALTER TABLE agent_snapshots ADD COLUMN IF NOT EXISTS branch TEXT NOT NULL DEFAULT '';
ALTER TABLE agent_snapshots DROP CONSTRAINT IF EXISTS agent_snapshots_user_id_repo_url_key;
ALTER TABLE agent_snapshots ADD CONSTRAINT agent_snapshots_user_repo_branch_key UNIQUE (user_id, repo_url, branch);
