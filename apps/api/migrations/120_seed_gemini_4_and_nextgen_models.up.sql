-- Migration 120: Seed Gemini 4 Series, Orchestrators, and Next-Gen Models
-- Matches the active Google Gemini model lineup from the user environment:
-- Gemini 4 Neon, Gemini 4 Argon, Gemini 4 Hydrogen, Orchestrator (Neon), Orchestrator (Argon), Gemini 3.8 Flash, Gemini 4 Carbon
-- Also seeds Claude Opus 5.5, Claude Sonnet 5.5, and GPT-OSS 120B

INSERT INTO models (
    provider_id, name, slug, display_name, enabled, context_window,
    supports_tools, supports_vision, quality_score, speed_score, reasoning_score, coding_score,
    created_at, updated_at
)
VALUES
    -- Google Gemini Next-Gen Series (provider_id: 'd0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11')
    ('d0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'gemini-4-neon', 'gemini-4-neon', 'Gemini 4 Neon', true, 1048576, true, true, 0.96, 0.92, 0.95, 0.96, NOW(), NOW()),
    ('d0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'gemini-4-argon', 'gemini-4-argon', 'Gemini 4 Argon', true, 1048576, true, true, 0.94, 0.94, 0.93, 0.94, NOW(), NOW()),
    ('d0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'gemini-4-hydrogen', 'gemini-4-hydrogen', 'Gemini 4 Hydrogen', true, 1048576, true, true, 0.92, 0.96, 0.90, 0.92, NOW(), NOW()),
    ('d0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'orchestrator-neon', 'orchestrator-neon', 'Orchestrator (Neon)', true, 1048576, true, true, 0.97, 0.90, 0.97, 0.97, NOW(), NOW()),
    ('d0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'orchestrator-argon', 'orchestrator-argon', 'Orchestrator (Argon)', true, 1048576, true, true, 0.95, 0.92, 0.95, 0.95, NOW(), NOW()),
    ('d0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'gemini-3.8-flash', 'gemini-3.8-flash', 'Gemini 3.8 Flash', true, 1048576, true, true, 0.90, 0.97, 0.89, 0.90, NOW(), NOW()),
    ('d0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'gemini-4-carbon', 'gemini-4-carbon', 'Gemini 4 Carbon', false, 1048576, false, false, 0.98, 0.70, 0.99, 0.98, NOW(), NOW()),

    -- Next-Gen Anthropic & Open Source models from the same suite
    ('c0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'claude-opus-5.5', 'claude-opus-5.5', 'Claude Opus 5.5', true, 200000, true, true, 0.98, 0.75, 0.98, 0.97, NOW(), NOW()),
    ('c0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'claude-sonnet-5.5', 'claude-sonnet-5.5', 'Claude Sonnet 5.5', true, 200000, true, true, 0.96, 0.88, 0.96, 0.97, NOW(), NOW()),
    ('b0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'gpt-oss-120b', 'gpt-oss-120b', 'GPT-OSS 120B (Medium)', true, 128000, true, false, 0.91, 0.90, 0.90, 0.91, NOW(), NOW())
ON CONFLICT (provider_id, slug) DO UPDATE SET
    name = EXCLUDED.name,
    display_name = EXCLUDED.display_name,
    enabled = EXCLUDED.enabled,
    context_window = EXCLUDED.context_window,
    supports_tools = EXCLUDED.supports_tools,
    supports_vision = EXCLUDED.supports_vision,
    quality_score = EXCLUDED.quality_score,
    speed_score = EXCLUDED.speed_score,
    reasoning_score = EXCLUDED.reasoning_score,
    coding_score = EXCLUDED.coding_score,
    updated_at = NOW();
