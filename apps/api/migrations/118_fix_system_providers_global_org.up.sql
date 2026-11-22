-- Migration 118: Ensure system providers are globally accessible across all organizations
-- Resets org_id and workspace_id to NULL for global system providers so multi-tenant isolation
-- does not hide system providers (OpenAI, Anthropic, Google, OpenCode) from non-default organizations.

UPDATE providers
SET org_id = NULL, workspace_id = NULL
WHERE slug IN ('openai', 'anthropic', 'google', 'opencode-zen', 'opencode-go', 'groq', 'deepseek');
