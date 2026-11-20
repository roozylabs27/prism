-- Rollback Migration 110: Remove agent_id binding from gateway_api_keys
DROP INDEX IF EXISTS idx_gateway_api_keys_agent_id;
ALTER TABLE gateway_api_keys DROP COLUMN IF EXISTS agent_id;
