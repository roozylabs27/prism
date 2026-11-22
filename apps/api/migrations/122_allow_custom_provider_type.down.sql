DELETE FROM providers WHERE type = 'custom';
ALTER TABLE providers DROP CONSTRAINT IF EXISTS providers_type_check;
ALTER TABLE providers ADD CONSTRAINT providers_type_check
    CHECK (type IN ('openai', 'anthropic', 'google', 'openrouter', 'opencode', 'groq', 'deepseek'));
