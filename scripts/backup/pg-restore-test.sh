#!/usr/bin/env bash
# Restores a dump into a scratch database next to the live one and checks it
# holds the schema, then drops it. Run weekly; a backup that has never been
# restored is not known to work.
#
#   scripts/backup/pg-restore-test.sh [dump-file]   default: the newest dump
#
# Weekly from cron:
#   30 3 * * 0 cd /opt/prism && scripts/backup/pg-restore-test.sh >> /var/log/prism-backup.log 2>&1
set -euo pipefail
cd "$(dirname "$0")"
# shellcheck source=scripts/backup/lib.sh source-path=SCRIPTDIR
. ./lib.sh
on_failure "backup restore test"

dump="${1:-$(find "$backup_dir" -name 'prism_*.dump' -print | sort | tail -n 1)}"
if [[ -z "$dump" || ! -f "$dump" ]]; then
  alert "Prism backup restore test on $(hostname) found no dump in $backup_dir"
  exit 1
fi

scratch="prism_restore_check_$(date -u +%Y%m%d%H%M%S)"
cleanup() { pg psql -q -d "$pg_db" -c "DROP DATABASE IF EXISTS $scratch WITH (FORCE)" > /dev/null || true; }
trap cleanup EXIT

pg psql -q -d "$pg_db" -c "CREATE DATABASE $scratch" > /dev/null
pg pg_restore --no-owner --no-privileges --exit-on-error -d "$scratch" < "$dump"

read -r version dirty tables <<< "$(pg psql -At -F ' ' -d "$scratch" -c \
  "SELECT version, dirty, (SELECT count(*) FROM information_schema.tables WHERE table_schema = 'public') FROM schema_migrations")"
if [[ "$dirty" != f || "$tables" -lt 10 ]]; then
  alert "Prism backup restore test on $(hostname): $dump restored to schema version $version (dirty=$dirty) with $tables tables"
  exit 1
fi

# Verify Row-Level Security flags survived dump/restore across core tenant tables.
rls_missing="$(pg psql -At -d "$scratch" -c "
  SELECT count(*) FROM (
    VALUES ('providers'), ('credentials'), ('gateway_api_keys'), ('agents'),
           ('budgets'), ('request_logs'), ('ai_audit_trails'), ('approval_requests')
  ) AS req(tbl)
  LEFT JOIN pg_class c ON c.relname = req.tbl AND c.relkind = 'r'
  LEFT JOIN pg_namespace n ON n.oid = c.relnamespace AND n.nspname = 'public'
  WHERE c.oid IS NULL OR c.relrowsecurity IS DISTINCT FROM true
")"
if [[ "${rls_missing:-1}" -ne 0 ]]; then
  alert "Prism backup restore test on $(hostname): $rls_missing expected RLS tables missing or unprotected in $scratch"
  exit 1
fi

# Verify checksum/row-count parity against live database for core seeded/control tables.
for check_tbl in schema_migrations providers models permissions roles; do
  src_cnt="$(pg psql -At -d "$pg_db" -c "SELECT count(*) FROM $check_tbl")"
  dst_cnt="$(pg psql -At -d "$scratch" -c "SELECT count(*) FROM $check_tbl")"
  if [[ "$src_cnt" != "$dst_cnt" ]]; then
    alert "Prism backup restore test on $(hostname): row-count mismatch on $check_tbl (live=$src_cnt, restored=$dst_cnt)"
    exit 1
  fi
done

# Non-superuser DB role verification: confirm NOSUPERUSER NOBYPASSRLS role enforces RLS isolation in restored scratch DB.
leaked_rows="$(pg psql -At -d "$scratch" -v ON_ERROR_STOP=1 -c "
  DO \$\$
  BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'prism_app_verify') THEN
      CREATE ROLE prism_app_verify NOLOGIN NOSUPERUSER NOBYPASSRLS;
    END IF;
  END \$\$;
  GRANT USAGE ON SCHEMA public TO prism_app_verify;
  GRANT SELECT ON ALL TABLES IN SCHEMA public TO prism_app_verify;
  INSERT INTO users (id, email, name, password_hash)
    VALUES ('rls_verify_u1', 'rls-verify-u1@example.invalid', 'RLS Verify U1', 'x')
    ON CONFLICT DO NOTHING;
  INSERT INTO organizations (id, name, slug, owner_id)
    VALUES ('rls_verify_org_a', 'RLS Verify Org A', 'rls-verify-org-a', 'rls_verify_u1'),
           ('rls_verify_org_b', 'RLS Verify Org B', 'rls-verify-org-b', 'rls_verify_u1')
    ON CONFLICT DO NOTHING;
  INSERT INTO agents (id, user_id, org_id, name, display_name)
    VALUES ('rls_verify_agent_a', 'rls_verify_u1', 'rls_verify_org_a', 'verify_a', 'Verify A'),
           ('rls_verify_agent_b', 'rls_verify_u1', 'rls_verify_org_b', 'verify_b', 'Verify B')
    ON CONFLICT DO NOTHING;
  BEGIN;
  SET LOCAL ROLE prism_app_verify;
  SELECT set_config('app.current_user_id', 'rls_verify_u1', true);
  SELECT set_config('app.current_org_id', 'rls_verify_org_a', true);
  CREATE TEMP TABLE _rls_leak_check AS
    SELECT id FROM agents WHERE id IN ('rls_verify_agent_a', 'rls_verify_agent_b');
  RESET ROLE;
  SELECT count(*) FROM _rls_leak_check WHERE id = 'rls_verify_agent_b';
  COMMIT;
" | tail -n 1)"
if [[ "${leaked_rows:-1}" != 0 ]]; then
  alert "Prism backup restore test on $(hostname): non-superuser RLS isolation check failed in $scratch (leaked=$leaked_rows)"
  exit 1
fi

echo "restored $dump: schema version $version, $tables tables, RLS & non-superuser role verified"
