# Database Migrations Runbook — RoozyLabs Prism

**Document Version:** v2.7.0  
**Verification Status:** `VERIFIED`  

---

## 1. Migration Execution Mechanism

Database migrations are managed sequentially using `golang-migrate` (`github.com/golang-migrate/migrate/v4`) located in `apps/api/migrations/` (migrations `001` through `114`, every migration strictly paired `.up.sql` / `.down.sql`).

In production Docker Compose deployments (`docker-compose.yml`), `api` and `api-green` run with `RUN_MIGRATIONS=false` so replicas never contend on DDL locks; schema migrations run exclusively inside the one-shot `migrator` service (`./server --migrate-only` with `RUN_MIGRATIONS=true`) prior to green/blue cutover. When running the binary standalone (`RUN_MIGRATIONS=true`), `database.RunMigrations` executes pending `.up.sql` scripts automatically:

```go
if err := database.RunMigrations(cfg.DatabaseURL, "./migrations"); err != nil {
    log.Fatal("Failed to run migrations:", err)
}
```

---

## 2. Migration Execution Guidelines

### Schema Migration Rules
1. **Backward Compatibility**: Column additions must be nullable (`NULL`) or have default values (`DEFAULT`).
2. **Idempotency**: DDL statements MUST use `IF NOT EXISTS` or `DROP TABLE IF EXISTS` (e.g. `ALTER TABLE ai_audit_trails ADD COLUMN IF NOT EXISTS org_id VARCHAR(64);`).
3. **Transaction Safety**: Wrap multi-statement SQL migrations in transactional blocks where supported.

---

## 3. Manual Migration Commands & Reversibility Limitations

> [!CAUTION]
> **Reversibility Limitations**: Migrations containing data transformations or index drops are **NOT automatically reversible** without potential data loss. Always perform a `pg_dump` backup BEFORE executing manual schema migrations.

### Execute Migrations Manually
```bash
# Verify current schema version
docker compose exec postgres psql -U postgres -d prism -c "SELECT * FROM schema_migrations;"

# Execute migration up via migrate CLI
migrate -path apps/api/migrations -database "postgres://postgres:postgres@localhost:5433/prism?sslmode=disable" up
```

### Rollback Failed Migration
```bash
# Rollback single migration step down
migrate -path apps/api/migrations -database "postgres://postgres:postgres@localhost:5433/prism?sslmode=disable" down 1

# Force schema version if migration dirty
migrate -path apps/api/migrations -database "postgres://postgres:postgres@localhost:5433/prism?sslmode=disable" force <VERSION>
```

---

## 4. Forward-Fix Expand/Contract Migration Policy

Because Prism uses zero-downtime blue/green application rollouts (`scripts/deploy/blue-green.sh`), the outgoing N-1 API container briefly coexists with and may receive rollback traffic (`--rollback`) against the migrated N schema:

1. **Expand First:** Add new tables, nullable columns, `DEFAULT` constraints, RLS policies, and indexes in an additive `.up.sql` migration so the N-1 API binary continues reading and writing without error.
2. **Never Drop or Break N-1 In-Flight:** Do not rename, drop, or narrow columns/constraints still read or written by the N-1 binary until at least one stable release cycle after N-1 is retired.
3. **Contract In a Separate Release:** Column drops or constraint tightenings ship in a subsequent migration only after N-1 has been decommissioned.
4. **Fix Forward, Not Down:** Do NOT execute automatic `down` migrations during automated rollback. Fix schema issues via a corrective forward migration (`N+1`) or, for corrupted data/state, restore from a verified backup (`scripts/backup/pg-restore-test.sh` & [backup-recovery.md](./backup-recovery.md)).
5. **Dual-File Pairing & Non-Superuser RLS Safety:** Every migration MUST ship paired `.up.sql` and `.down.sql` files, and any new tenant-scoped table MUST enable `ROW LEVEL SECURITY` with explicit `app.current_org_id` / `app.current_user_id` policies tested under `NOSUPERUSER NOBYPASSRLS`.
