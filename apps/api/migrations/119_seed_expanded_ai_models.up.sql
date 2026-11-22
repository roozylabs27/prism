-- Migration 119: Seed Expanded AI Models for Google Gemini, Groq, OpenAI, Anthropic, and DeepSeek
-- Adds standard current production models and updates capabilities (tools & vision)

INSERT INTO models (provider_id, name, slug, display_name, enabled, context_window, supports_tools, supports_vision, quality_score, speed_score, reasoning_score, coding_score, created_at, updated_at)
VALUES
    -- =========================================================================
    -- Google Gemini (provider_id: 'd0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11')
    -- =========================================================================
    ('d0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'gemini-2.0-flash', 'gemini-2.0-flash', 'Gemini 2.0 Flash', true, 1048576, true, true, 0.88, 0.95, 0.87, 0.88, NOW(), NOW()),
    ('d0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'gemini-2.0-flash-lite', 'gemini-2.0-flash-lite', 'Gemini 2.0 Flash-Lite', true, 1048576, true, true, 0.82, 0.98, 0.80, 0.82, NOW(), NOW()),
    ('d0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'gemini-2.0-flash-thinking-exp', 'gemini-2.0-flash-thinking-exp', 'Gemini 2.0 Flash Thinking Exp', true, 1048576, true, true, 0.92, 0.85, 0.94, 0.91, NOW(), NOW()),
    ('d0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'gemini-1.5-flash', 'gemini-1.5-flash', 'Gemini 1.5 Flash', true, 1048576, true, true, 0.84, 0.92, 0.82, 0.83, NOW(), NOW()),
    ('d0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'gemini-1.5-flash-8b', 'gemini-1.5-flash-8b', 'Gemini 1.5 Flash 8B', true, 1048576, true, true, 0.78, 0.97, 0.75, 0.76, NOW(), NOW()),
    ('d0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'gemini-1.5-pro', 'gemini-1.5-pro', 'Gemini 1.5 Pro', true, 2097152, true, true, 0.92, 0.80, 0.91, 0.90, NOW(), NOW()),
    ('d0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'gemini-2.0-pro-exp-02-05', 'gemini-2.0-pro-exp-02-05', 'Gemini 2.0 Pro Exp', true, 2097152, true, true, 0.95, 0.78, 0.96, 0.95, NOW(), NOW()),

    -- =========================================================================
    -- Groq Fast Inference (provider_id: 'a2eebc99-9c0b-4ef8-bb6d-6bb9bd380a11')
    -- =========================================================================
    ('a2eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'llama-3.3-70b-versatile', 'llama-3.3-70b-versatile', 'Llama 3.3 70B (Groq)', true, 128000, true, false, 0.89, 0.96, 0.88, 0.87, NOW(), NOW()),
    ('a2eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'llama-3.1-8b-instant', 'llama-3.1-8b-instant', 'Llama 3.1 8B Instant (Groq)', true, 128000, true, false, 0.79, 0.99, 0.76, 0.78, NOW(), NOW()),
    ('a2eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'deepseek-r1-distill-llama-70b', 'deepseek-r1-distill-llama-70b', 'DeepSeek R1 Distill 70B (Groq)', true, 128000, false, false, 0.93, 0.92, 0.95, 0.92, NOW(), NOW()),
    ('a2eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'mixtral-8x7b-32768', 'mixtral-8x7b-32768', 'Mixtral 8x7B (Groq)', true, 32768, true, false, 0.81, 0.95, 0.79, 0.80, NOW(), NOW()),
    ('a2eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'gemma2-9b-it', 'gemma2-9b-it', 'Gemma 2 9B (Groq)', true, 8192, true, false, 0.77, 0.97, 0.74, 0.75, NOW(), NOW()),

    -- =========================================================================
    -- OpenAI (provider_id: 'b0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11')
    -- =========================================================================
    ('b0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'gpt-4o', 'gpt-4o', 'GPT-4o', true, 128000, true, true, 0.94, 0.88, 0.92, 0.93, NOW(), NOW()),
    ('b0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'gpt-4o-mini', 'gpt-4o-mini', 'GPT-4o Mini', true, 128000, true, true, 0.86, 0.94, 0.83, 0.85, NOW(), NOW()),
    ('b0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'o3-mini', 'o3-mini', 'o3-mini Reasoning', true, 200000, true, false, 0.93, 0.86, 0.95, 0.94, NOW(), NOW()),
    ('b0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'o1', 'o1', 'OpenAI o1', true, 200000, true, true, 0.96, 0.70, 0.97, 0.96, NOW(), NOW()),
    ('b0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'chatgpt-4o-latest', 'chatgpt-4o-latest', 'ChatGPT-4o Latest', true, 128000, true, true, 0.95, 0.87, 0.93, 0.94, NOW(), NOW()),

    -- =========================================================================
    -- Anthropic (provider_id: 'c0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11')
    -- =========================================================================
    ('c0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'claude-3-7-sonnet-20250219', 'claude-3-7-sonnet', 'Claude 3.7 Sonnet', true, 200000, true, true, 0.97, 0.84, 0.97, 0.98, NOW(), NOW()),
    ('c0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'claude-3-5-sonnet-20241022', 'claude-3-5-sonnet', 'Claude 3.5 Sonnet', true, 200000, true, true, 0.95, 0.86, 0.94, 0.96, NOW(), NOW()),
    ('c0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'claude-3-5-haiku-20241022', 'claude-3-5-haiku', 'Claude 3.5 Haiku', true, 200000, true, true, 0.87, 0.94, 0.84, 0.86, NOW(), NOW()),
    ('c0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'claude-3-opus-20240229', 'claude-3-opus', 'Claude 3 Opus', true, 200000, true, true, 0.93, 0.72, 0.92, 0.91, NOW(), NOW()),

    -- =========================================================================
    -- DeepSeek AI (provider_id: 'a1eebc99-9c0b-4ef8-bb6d-6bb9bd380a11')
    -- =========================================================================
    ('a1eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'deepseek-chat', 'deepseek-chat', 'DeepSeek V3 (Chat)', true, 64000, true, false, 0.91, 0.90, 0.89, 0.91, NOW(), NOW()),
    ('a1eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'deepseek-reasoner', 'deepseek-reasoner', 'DeepSeek R1 (Reasoner)', true, 64000, false, false, 0.95, 0.76, 0.97, 0.94, NOW(), NOW())
ON CONFLICT (provider_id, slug) DO UPDATE SET
    display_name = EXCLUDED.display_name,
    enabled = true,
    supports_tools = EXCLUDED.supports_tools,
    supports_vision = EXCLUDED.supports_vision,
    context_window = CASE WHEN EXCLUDED.context_window > 0 THEN EXCLUDED.context_window ELSE models.context_window END,
    quality_score = EXCLUDED.quality_score,
    speed_score = EXCLUDED.speed_score,
    reasoning_score = EXCLUDED.reasoning_score,
    coding_score = EXCLUDED.coding_score,
    updated_at = NOW();
