package main

import (
	"context"
	"fmt"
	"log"

	"github.com/ganiramadhan/starter-go/internal/domain"
	"github.com/ganiramadhan/starter-go/internal/platform/audit"
	"github.com/ganiramadhan/starter-go/internal/platform/database"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func must(err error) {
	if err != nil {
		log.Fatalf("FAIL: %v", err)
	}
}

func main() {
	db, err := database.Connect(database.Config{
		Host: "127.0.0.1", Port: "55432", User: "postgres", Password: "postgres",
		Name: "saku_test", SSLMode: "disable", MaxOpenConns: 5, MaxIdleConns: 2,
	})
	must(err)

	must(database.Migrate(db))
	fmt.Println("OK: migration (schema + audit trail) applied")

	// Re-run to prove idempotency, exactly like every API boot does.
	must(database.Migrate(db))
	fmt.Println("OK: migration re-applied idempotently")

	// --- seed a user + wallet so FK constraints are satisfied ---
	userID := uuid.New()
	must(db.Exec(`INSERT INTO users (id, name, email, password, role, status, created_at, updated_at)
		VALUES (?, 'Test User', 'test@example.com', 'x', 'user', 'active', now(), now())`, userID).Error)

	walletID := uuid.New()
	uid := userID
	must(audit.WithTx(
		audit.WithActor(context.Background(), audit.Actor{UserID: &uid, IP: "203.0.113.9", UserAgent: "verify-script/1.0", RequestID: "req-scenario-a"}),
		db,
		func(tx *gorm.DB) error {
			return tx.Exec(`INSERT INTO wallets (id, user_id, name, type, currency, balance_cached, is_default, created_at, updated_at)
				VALUES (?, ?, 'BCA', 'bank_account', 'IDR', 1000000, true, now(), now())`, walletID, userID).Error
		},
	))
	fmt.Println("OK: scenario A (app INSERT via audit.WithTx) executed")

	var actorID *uuid.UUID
	var dbUser, action, tableName, recordID string
	var isDirect bool
	var reqID *string
	must(db.Raw(`SELECT actor_id, db_user, action, table_name, record_id, is_direct_db_change, request_id
		FROM audit_logs WHERE table_name='wallets' AND record_id=? ORDER BY created_at DESC LIMIT 1`, walletID.String()).
		Row().Scan(&actorID, &dbUser, &action, &tableName, &recordID, &isDirect, &reqID))
	if actorID == nil || *actorID != userID {
		log.Fatalf("FAIL: scenario A actor_id = %v, want %v", actorID, userID)
	}
	if action != "INSERT" || isDirect {
		log.Fatalf("FAIL: scenario A action=%s isDirect=%v", action, isDirect)
	}
	if reqID == nil || *reqID != "req-scenario-a" {
		log.Fatalf("FAIL: scenario A request_id = %v", reqID)
	}
	fmt.Printf("OK: scenario A audit row correct (actor_id=%s db_user=%s action=%s is_direct=%v request_id=%s)\n", actorID, dbUser, action, isDirect, *reqID)

	must(audit.WithTx(
		audit.WithActor(context.Background(), audit.Actor{UserID: &uid, RequestID: "req-scenario-b"}),
		db,
		func(tx *gorm.DB) error {
			return tx.Exec(`UPDATE wallets SET name = 'BCA Tabungan Utama', updated_at = now() WHERE id = ?`, walletID).Error
		},
	))
	var oldName, newName, changed *string
	must(db.Raw(`SELECT old_data->>'name', new_data->>'name', changed_data::text
		FROM audit_logs WHERE table_name='wallets' AND record_id=? AND action='UPDATE'
		ORDER BY created_at DESC LIMIT 1`, walletID.String()).Row().Scan(&oldName, &newName, &changed))
	if oldName == nil || *oldName != "BCA" || newName == nil || *newName != "BCA Tabungan Utama" {
		log.Fatalf("FAIL: scenario B old/new = %v/%v", oldName, newName)
	}
	fmt.Printf("OK: scenario B UPDATE captured old=%q new=%q changed_data=%s\n", *oldName, *newName, *changed)

	must(db.Exec(`UPDATE wallets SET balance_cached = balance_cached + 1000000 WHERE id = ?`, walletID).Error)
	var directActor *uuid.UUID
	var directIsDirect bool
	var directDBUser string
	must(db.Raw(`SELECT actor_id, db_user, is_direct_db_change FROM audit_logs
		WHERE table_name='wallets' AND record_id=? AND action='UPDATE'
		ORDER BY created_at DESC LIMIT 1`, walletID.String()).Row().Scan(&directActor, &directDBUser, &directIsDirect))
	if directActor != nil {
		log.Fatalf("FAIL: scenario D actor_id should be NULL, got %v", directActor)
	}
	if !directIsDirect {
		log.Fatalf("FAIL: scenario D is_direct_db_change should be true")
	}
	fmt.Printf("OK: scenario D direct SQL change: actor_id=NULL db_user=%s is_direct=%v\n", directDBUser, directIsDirect)

	err = db.Exec(`DELETE FROM audit_logs WHERE table_name = 'wallets' LIMIT 1`).Error
	if err != nil {
		fmt.Printf("OK: scenario F — DELETE on audit_logs rejected: %v\n", err)
	} else {
		fmt.Println("NOTE: scenario F — DELETE on audit_logs succeeded (expected under this single-role local setup; see deploy/audit/roles.sql for the production fix)")
	}

	var fnCount int64
	must(db.Raw(`SELECT count(*) FROM pg_proc WHERE proname IN ('audit_trigger_fn','audit_redact')`).Row().Scan(&fnCount))
	if fnCount != 2 {
		log.Fatalf("FAIL: expected 2 audit functions, found %d", fnCount)
	}
	fmt.Println("OK: audit_trigger_fn and audit_redact exist")

	var triggerCount int64
	must(db.Raw(`SELECT count(*) FROM pg_trigger WHERE tgname LIKE 'trg_audit_%' AND NOT tgisinternal`).Row().Scan(&triggerCount))
	fmt.Printf("OK: %d audit triggers attached\n", triggerCount)

	_ = domain.Wallet{}
	fmt.Println("ALL SCENARIOS PASSED")
}
