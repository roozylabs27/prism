-- Migration 110: Bind optional agent_id on gateway_api_keys
ALTER TABLE gateway_api_keys
    ADD COLUMN IF NOT EXISTS agent_id UUID REFERENCES agents(id) ON DELETE RESTRICT;

CREATE INDEX IF NOT EXISTS idx_gateway_api_keys_agent_id
    ON gateway_api_keys (agent_id)
    WHERE agent_id IS NOT NULL;
