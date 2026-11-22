-- Rollback Migration 120
DELETE FROM models WHERE slug IN (
    'gemini-4-neon',
    'gemini-4-argon',
    'gemini-4-hydrogen',
    'orchestrator-neon',
    'orchestrator-argon',
    'gemini-3.8-flash',
    'gemini-4-carbon',
    'claude-opus-5.5',
    'claude-sonnet-5.5',
    'gpt-oss-120b'
);
