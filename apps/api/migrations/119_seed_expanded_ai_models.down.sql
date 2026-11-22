-- Rollback Migration 119
DELETE FROM models WHERE slug IN (
    'gemini-2.0-flash-lite',
    'gemini-2.0-flash-thinking-exp',
    'gemini-1.5-flash-8b',
    'gemini-2.0-pro-exp-02-05',
    'llama-3.1-8b-instant',
    'deepseek-r1-distill-llama-70b',
    'mixtral-8x7b-32768',
    'gemma2-9b-it',
    'chatgpt-4o-latest',
    'claude-3-opus'
);
