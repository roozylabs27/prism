-- Rollback Migration 118: Revert system providers to default organization
UPDATE providers
SET org_id = 'org_default', workspace_id = 'ws_default'
WHERE slug IN ('openai', 'anthropic', 'google', 'opencode-zen', 'opencode-go');
