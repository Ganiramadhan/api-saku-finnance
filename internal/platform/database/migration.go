package database

import (
	"fmt"

	"github.com/ganiramadhan/starter-go/internal/domain"
	"gorm.io/gorm"
)

func Migrate(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("db is nil")
	}

	if err := ensureEnums(db); err != nil {
		return fmt.Errorf("ensure enums: %w", err)
	}

	if err := relaxAILogSchema(db); err != nil {
		return fmt.Errorf("relax ai_log schema: %w", err)
	}

	if err := dropLegacyAttachments(db); err != nil {
		return fmt.Errorf("drop legacy attachments: %w", err)
	}

	if err := db.AutoMigrate(
		&domain.User{},
		&domain.Wallet{},
		&domain.Category{},
		&domain.Transaction{},
		&domain.AIProcessingLog{},
		&domain.Budget{},
		&domain.SavingsGoal{},
		&domain.SavingsGoalContribution{},
		&domain.UpcomingBilling{},
		&domain.Plan{},
		&domain.Subscription{},
		&domain.Notification{},
		&domain.SplitBill{},
		&domain.SplitBillParticipant{},
	); err != nil {
		return fmt.Errorf("auto migrate: %w", err)
	}

	for _, idx := range indexStatements {
		if err := db.Exec(idx).Error; err != nil {
			return fmt.Errorf("create index: %w", err)
		}
	}
	return nil
}

func dropLegacyAttachments(db *gorm.DB) error {
	stmts := []string{
		`ALTER TABLE IF EXISTS ai_processing_logs DROP CONSTRAINT IF EXISTS fk_ai_processing_logs_attachment`,
		`ALTER TABLE IF EXISTS ai_processing_logs DROP COLUMN IF EXISTS attachment_id`,
		`DROP TABLE IF EXISTS attachments CASCADE`,
	}
	for _, s := range stmts {
		_ = db.Exec(s).Error
	}
	return nil
}

func relaxAILogSchema(db *gorm.DB) error {
	if !db.Migrator().HasTable("ai_processing_logs") {
		return nil
	}

	stmts := []string{
		`ALTER TABLE IF EXISTS ai_processing_logs ADD COLUMN IF NOT EXISTS user_id uuid`,
		`ALTER TABLE IF EXISTS ai_processing_logs ADD COLUMN IF NOT EXISTS feature varchar(32) DEFAULT 'categorize'`,
		`ALTER TABLE IF EXISTS ai_processing_logs ADD COLUMN IF NOT EXISTS latency_ms int DEFAULT 0`,
		`ALTER TABLE IF EXISTS ai_processing_logs ADD COLUMN IF NOT EXISTS error_message text`,
	}
	for _, s := range stmts {
		_ = db.Exec(s).Error
	}
	return nil
}

func ensureEnums(db *gorm.DB) error {
	stmts := []string{
		`DO $$ BEGIN
			CREATE TYPE transaction_type_enum AS ENUM ('income','expense');
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,

		`DO $$ BEGIN
			CREATE TYPE transaction_source_enum AS ENUM ('manual','ai_ocr','import','api');
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,

		`DO $$ BEGIN
			CREATE TYPE ai_status_enum AS ENUM ('pending','success','failed');
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,

		`DO $$ BEGIN
			CREATE TYPE wallet_type_enum AS ENUM ('personal','business','shared');
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,

		`DO $$ BEGIN
			CREATE TYPE budget_period_enum AS ENUM ('daily','weekly','monthly');
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
	}
	for _, s := range stmts {
		if err := db.Exec(s).Error; err != nil {
			return err
		}
	}
	return nil
}

var indexStatements = []string{
	`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email
		ON users (email) WHERE deleted_at IS NULL`,
	`CREATE INDEX IF NOT EXISTS idx_users_status ON users (status)`,

	`CREATE UNIQUE INDEX IF NOT EXISTS idx_wallets_user_default
		ON wallets (user_id) WHERE is_default = true AND deleted_at IS NULL`,
	`CREATE INDEX IF NOT EXISTS idx_wallets_user_type
		ON wallets (user_id, type) WHERE deleted_at IS NULL`,

	`CREATE INDEX IF NOT EXISTS idx_categories_user_type
		ON categories (user_id, type) WHERE deleted_at IS NULL`,
	`CREATE INDEX IF NOT EXISTS idx_categories_system
		ON categories (is_system) WHERE deleted_at IS NULL`,

	`CREATE INDEX IF NOT EXISTS idx_transactions_wallet_date
		ON transactions (wallet_id, transaction_date DESC) WHERE deleted_at IS NULL`,
	`CREATE INDEX IF NOT EXISTS idx_transactions_category_date
		ON transactions (category_id, transaction_date DESC) WHERE deleted_at IS NULL`,
	`CREATE INDEX IF NOT EXISTS idx_transactions_date
		ON transactions (transaction_date DESC) WHERE deleted_at IS NULL`,
	`CREATE INDEX IF NOT EXISTS idx_transactions_type
		ON transactions (type) WHERE deleted_at IS NULL`,

	`CREATE INDEX IF NOT EXISTS idx_budgets_user_period
		ON budgets (user_id, period) WHERE deleted_at IS NULL`,
	`CREATE INDEX IF NOT EXISTS idx_budgets_wallet_category
		ON budgets (wallet_id, category_id) WHERE deleted_at IS NULL`,
	`CREATE INDEX IF NOT EXISTS idx_notifications_user_created
		ON notifications (user_id, created_at DESC)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS idx_notifications_user_ref_type
		ON notifications (user_id, ref_type, ref_id, type)
		WHERE ref_type <> '' AND ref_id <> ''`,
}
