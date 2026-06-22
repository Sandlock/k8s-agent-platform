ALTER TABLE agent_snapshots DROP CONSTRAINT IF EXISTS agent_snapshots_user_repo_branch_key;
ALTER TABLE agent_snapshots ADD CONSTRAINT agent_snapshots_user_id_repo_url_key UNIQUE (user_id, repo_url);
ALTER TABLE agent_snapshots DROP COLUMN IF EXISTS branch;
ALTER TABLE sandboxes DROP COLUMN IF EXISTS branch;
