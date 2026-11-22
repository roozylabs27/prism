-- Migration 121: Align Google models with valid Google AI Studio upstream model endpoints
-- Provider: Google ('d0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11')

-- 1. Direct valid production Google Gemini models
UPDATE models SET name = 'gemini-3.5-flash' WHERE provider_id = 'd0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11' AND slug IN ('gemini-3.5-flash', 'gemini-2.0-flash', 'gemini-1.5-flash', 'gemini-flash', 'gemini-1.5-pro', 'gemini-pro', 'gemini-2.0-pro');
UPDATE models SET name = 'gemini-3.5-flash-lite' WHERE provider_id = 'd0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11' AND slug IN ('gemini-3.5-flash-lite', 'gemini-2.0-flash-lite', 'gemini-1.5-flash-8b');
UPDATE models SET name = 'gemini-3.6-flash' WHERE provider_id = 'd0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11' AND slug = 'gemini-3.6-flash';
UPDATE models SET name = 'gemini-3.7-flash' WHERE provider_id = 'd0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11' AND slug = 'gemini-3.7-flash';
UPDATE models SET name = 'gemini-3.8-flash' WHERE provider_id = 'd0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11' AND slug = 'gemini-3.8-flash';

-- 2. Virtual and Orchestrator aliases: route upstream to gemini-3.5-flash
UPDATE models SET name = 'gemini-3.5-flash' WHERE provider_id = 'd0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11' AND slug IN (
    'orchestrator-neon',
    'orchestrator-argon',
    'gemini-4-neon',
    'gemini-4-argon',
    'gemini-4-hydrogen',
    'gemini-4-carbon'
);
