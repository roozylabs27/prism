-- Wave 1 (REM-03 / AUD-P0-03): Enforce unique account links per (user_id, provider_id)
-- after deduplicating any historical duplicate OAuth account rows.

DELETE FROM account a
USING account b
WHERE a.ctid < b.ctid
  AND a.user_id = b.user_id
  AND a.provider_id = b.provider_id;

CREATE UNIQUE INDEX IF NOT EXISTS uq_account_user_provider ON account (user_id, provider_id);
