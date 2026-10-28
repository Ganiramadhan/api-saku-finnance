package database

import (
	"fmt"

	"gorm.io/gorm"
)

var auditedTables = []string{
	"wallets",          // account identity + balance_cached
	"wallet_transfers", // money movement between two wallets (hard-deleted, no soft delete)
	"transactions",     // core ledger: amount, type, category, wallet
	"budgets",          // spending limits per wallet/category
}

func MigrateAudit(db *gorm.DB) error {
	if err := execAll(db, auditBaseStatements); err != nil {
		return fmt.Errorf("audit: base objects: %w", err)
	}

	for _, table := range auditedTables {
		if err := execAll(db, auditTriggerStatements(table)); err != nil {
			return fmt.Errorf("audit: attach trigger to %s: %w", table, err)
		}
	}

	if err := lockDownAuditLogs(db); err != nil {
		return fmt.Errorf("audit: lock down audit_logs: %w", err)
	}

	return nil
}

var auditBaseStatements = []string{
	createExtension,
	createAuditLogsTable,
	auditLogsComment,
	idxAuditLogsTableRecord,
	idxAuditLogsActor,
	idxAuditLogsRequestID,
	idxAuditLogsDirectChange,
	idxAuditLogsCreatedAtBRIN,
	createRedactFn,
	createAuditTriggerFn,
}

const createExtension = `CREATE EXTENSION IF NOT EXISTS pgcrypto`

const createAuditLogsTable = `
CREATE TABLE IF NOT EXISTS audit_logs (
	id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),

	-- Application actor, resolved server-side from a verified JWT
	-- (middleware.AuthRequired -> platform/audit). NULL means the write
	-- carried no application context: either an unauthenticated code path,
	-- or a statement issued directly against PostgreSQL outside the API.
	actor_id             UUID NULL,

	-- The PostgreSQL role that executed the statement. Populated
	-- unconditionally by the trigger (current_user), independent of the
	-- application context above — this is what lets you tell "the API's
	-- database role changed this row" apart from "someone connected with
	-- psql as a different role changed this row".
	db_user              TEXT NOT NULL,

	action               TEXT NOT NULL CHECK (action IN ('INSERT', 'UPDATE', 'DELETE')),
	table_name           TEXT NOT NULL,
	record_id            TEXT NOT NULL,

	old_data             JSONB NULL,
	new_data             JSONB NULL,
	-- UPDATE only: {"column": {"old": ..., "new": ...}} for columns that
	-- actually changed value. NULL for INSERT/DELETE (old_data/new_data
	-- already carry the full row there).
	changed_data         JSONB NULL,

	-- TEXT rather than INET: values come straight from c.IP() (fiber), which
	-- can legitimately be "" behind some proxy configs, and TEXT keeps the
	-- Go read side (database/sql scanning into *string) simple.
	ip_address           TEXT NULL,
	user_agent           TEXT NULL,
	request_id           TEXT NULL,

	-- true when the write reached PostgreSQL without app.* session context,
	-- i.e. it did not go through the SAKU API's transaction wrapper. This is
	-- the signal used to flag direct SQL manipulation (see scenario D in
	-- docs/audit/README.md).
	is_direct_db_change  BOOLEAN NOT NULL DEFAULT true,

	transaction_id       BIGINT NOT NULL,
	statement_ts         TIMESTAMPTZ NOT NULL,
	created_at           TIMESTAMPTZ NOT NULL DEFAULT now()
)`

const auditLogsComment = `COMMENT ON TABLE audit_logs IS
	'Append-only audit trail written exclusively by audit_trigger_fn(). Do not INSERT/UPDATE/DELETE directly.'`

const idxAuditLogsTableRecord = `
	CREATE INDEX IF NOT EXISTS idx_audit_logs_table_record
		ON audit_logs (table_name, record_id, created_at DESC)`

const idxAuditLogsActor = `
	CREATE INDEX IF NOT EXISTS idx_audit_logs_actor
		ON audit_logs (actor_id, created_at DESC) WHERE actor_id IS NOT NULL`

const idxAuditLogsRequestID = `
	CREATE INDEX IF NOT EXISTS idx_audit_logs_request_id
		ON audit_logs (request_id) WHERE request_id IS NOT NULL`

const idxAuditLogsDirectChange = `
	CREATE INDEX IF NOT EXISTS idx_audit_logs_direct_change
		ON audit_logs (created_at DESC) WHERE is_direct_db_change = true`

const idxAuditLogsCreatedAtBRIN = `
	CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at_brin
		ON audit_logs USING BRIN (created_at)`

const createRedactFn = `
CREATE OR REPLACE FUNCTION audit_redact(p JSONB) RETURNS JSONB
LANGUAGE sql
IMMUTABLE
AS $$
	SELECT CASE
		WHEN p IS NULL THEN NULL
		ELSE (
			SELECT jsonb_object_agg(
				kv.key,
				CASE
					WHEN kv.key ~* '(password|passwd|secret|token|api[_-]?key|access[_-]?key)'
						THEN '"[REDACTED]"'::jsonb
					ELSE kv.value
				END
			)
			FROM jsonb_each(p) AS kv(key, value)
		)
	END;
$$;
`

const createAuditTriggerFn = `
CREATE OR REPLACE FUNCTION audit_trigger_fn() RETURNS TRIGGER
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, public
AS $$
DECLARE
	v_actor_id      UUID;
	v_ip            TEXT;
	v_user_agent    TEXT;
	v_request_id    TEXT;
	v_old_data      JSONB;
	v_new_data      JSONB;
	v_changed_data  JSONB;
	v_record_id     TEXT;
	v_is_direct     BOOLEAN;
BEGIN
	-- app.* is set for the duration of the transaction by
	-- platform/audit.ApplySessionContext (SELECT set_config(..., true)),
	-- executed as the first statement of every write to an audited table.
	-- current_setting(name, true) returns NULL instead of raising when the
	-- variable was never set for this transaction — which is exactly the
	-- "no application context" / direct SQL case.
	BEGIN
		v_actor_id := NULLIF(current_setting('app.current_user_id', true), '')::uuid;
	EXCEPTION WHEN OTHERS THEN
		v_actor_id := NULL;
	END;
	v_ip := NULLIF(current_setting('app.client_ip', true), '');
	v_user_agent := NULLIF(current_setting('app.user_agent', true), '');
	v_request_id := NULLIF(current_setting('app.request_id', true), '');

	-- A statement is treated as "direct" (bypassed the API) when it carries
	-- no request id at all. A logged-in user's write always has one; a
	-- background job could call ApplySessionContext with an empty Actor and
	-- would also be flagged here, which is the conservative/correct choice.
	v_is_direct := (v_request_id IS NULL);

	IF TG_OP = 'INSERT' THEN
		v_new_data  := audit_redact(to_jsonb(NEW));
		v_record_id := to_jsonb(NEW) ->> 'id';
	ELSIF TG_OP = 'UPDATE' THEN
		v_old_data  := audit_redact(to_jsonb(OLD));
		v_new_data  := audit_redact(to_jsonb(NEW));
		v_record_id := to_jsonb(NEW) ->> 'id';
		SELECT jsonb_object_agg(o.key, jsonb_build_object('old', o.value, 'new', n.value))
		INTO v_changed_data
		FROM jsonb_each(v_old_data) AS o(key, value)
		JOIN jsonb_each(v_new_data) AS n(key, value) USING (key)
		WHERE o.value IS DISTINCT FROM n.value;
	ELSIF TG_OP = 'DELETE' THEN
		v_old_data  := audit_redact(to_jsonb(OLD));
		v_record_id := to_jsonb(OLD) ->> 'id';
	END IF;

	INSERT INTO audit_logs (
		actor_id, db_user, action, table_name, record_id,
		old_data, new_data, changed_data,
		ip_address, user_agent, request_id, is_direct_db_change,
		transaction_id, statement_ts
	) VALUES (
		v_actor_id, current_user, TG_OP, TG_TABLE_NAME, COALESCE(v_record_id, ''),
		v_old_data, v_new_data, v_changed_data,
		v_ip, v_user_agent, v_request_id, v_is_direct,
		txid_current(), statement_timestamp()
	);

	IF TG_OP = 'DELETE' THEN
		RETURN OLD;
	END IF;
	RETURN NEW;
END;
$$;
`

func auditTriggerStatements(table string) []string {
	return []string{
		fmt.Sprintf(`DROP TRIGGER IF EXISTS trg_audit_%[1]s ON %[1]s`, table),
		fmt.Sprintf(`
			CREATE TRIGGER trg_audit_%[1]s
				AFTER INSERT OR UPDATE OR DELETE ON %[1]s
				FOR EACH ROW EXECUTE FUNCTION audit_trigger_fn()`, table),
	}
}

func lockDownAuditLogs(db *gorm.DB) error {
	return db.Exec(`
DO $$
DECLARE
	app_role TEXT := current_user;
BEGIN
	EXECUTE format('REVOKE UPDATE, DELETE, TRUNCATE ON audit_logs FROM %I', app_role);
	EXECUTE format('REVOKE UPDATE, DELETE, TRUNCATE ON audit_logs FROM PUBLIC');
EXCEPTION WHEN insufficient_privilege THEN
	-- best effort only; do not fail the whole migration over this
	NULL;
END $$;
`).Error
}
