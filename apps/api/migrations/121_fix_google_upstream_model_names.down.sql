-- Migration 121 Down: Revert model names
UPDATE models SET name = 'gemini-3.6-flash' WHERE provider_id = 'd0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11' AND slug IN ('gemini-2.0-flash', 'gemini-1.5-flash', 'gemini-1.5-pro', 'gemini-flash', 'gemini-pro', 'gemini-2.0-pro');
UPDATE models SET name = slug WHERE provider_id = 'd0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11' AND slug IN ('orchestrator-neon', 'orchestrator-argon', 'gemini-4-neon', 'gemini-4-argon', 'gemini-4-hydrogen', 'gemini-4-carbon', 'gemini-3.8-flash', 'gemini-3.7-flash', 'gemini-3.6-flash');
