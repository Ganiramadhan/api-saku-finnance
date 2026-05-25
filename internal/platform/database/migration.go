package database

import (
	"fmt"

	"github.com/ganiramadhan/starter-go/internal/domain"
	"gorm.io/gorm"
)

type foreignKeySpec struct {
	Name      string
	Table     string
	Column    string
	RefTable  string
	RefColumn string
	OnUpdate  string
	OnDelete  string
}

func Migrate(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("db is nil")
	}

	if err := autoMigrate(db); err != nil {
		return fmt.Errorf("auto migrate: %w", err)
	}

	if err := ensureIndexes(db); err != nil {
		return fmt.Errorf("ensure indexes: %w", err)
	}

	if err := ensureForeignKeys(db); err != nil {
		return fmt.Errorf("ensure foreign keys: %w", err)
	}

	return nil
}

func autoMigrate(db *gorm.DB) error {
	prevFKAutoMigrate := db.Config.DisableForeignKeyConstraintWhenMigrating
	db.Config.DisableForeignKeyConstraintWhenMigrating = true
	defer func() {
		db.Config.DisableForeignKeyConstraintWhenMigrating = prevFKAutoMigrate
	}()

	return db.AutoMigrate(
		&domain.User{},
		&domain.UserOTP{},
		&domain.UserReferral{},
		&domain.Wallet{},
		&domain.WalletTarget{},
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
	)
}

func ensureIndexes(db *gorm.DB) error {
	return execAll(db, indexStatements)
}

func ensureForeignKeys(db *gorm.DB) error {
	for _, fk := range foreignKeys {
		if err := db.Exec(foreignKeySQL(fk)).Error; err != nil {
			return fmt.Errorf("%s: %w", fk.Name, err)
		}
	}
	return nil
}

func execAll(db *gorm.DB, stmts []string) error {
	for _, stmt := range stmts {
		if err := db.Exec(stmt).Error; err != nil {
			return err
		}
	}
	return nil
}

func foreignKeySQL(fk foreignKeySpec) string {
	return fmt.Sprintf(`DO $$
	DECLARE
		existing RECORD;
	BEGIN
		IF EXISTS (
			SELECT 1
			FROM pg_constraint con
			JOIN pg_class tbl ON tbl.oid = con.conrelid
			JOIN pg_namespace ns ON ns.oid = tbl.relnamespace
			WHERE con.contype = 'f'
				AND con.conname = '%[1]s'
				AND ns.nspname = current_schema()
				AND tbl.relname = '%[2]s'
		) THEN
			RETURN;
		END IF;

		FOR existing IN
			SELECT con.conname
			FROM pg_constraint con
			JOIN pg_class tbl ON tbl.oid = con.conrelid
			JOIN pg_namespace ns ON ns.oid = tbl.relnamespace
			JOIN pg_attribute att ON att.attrelid = tbl.oid AND att.attnum = ANY(con.conkey)
			WHERE con.contype = 'f'
				AND ns.nspname = current_schema()
				AND tbl.relname = '%[2]s'
				AND att.attname = '%[3]s'
		LOOP
			EXECUTE format('ALTER TABLE %%I DROP CONSTRAINT %%I', '%[2]s', existing.conname);
		END LOOP;

		EXECUTE format(
			'ALTER TABLE %%I ADD CONSTRAINT %%I FOREIGN KEY (%%I) REFERENCES %%I(%%I) ON UPDATE %[6]s ON DELETE %[7]s',
			'%[2]s',
			'%[1]s',
			'%[3]s',
			'%[4]s',
			'%[5]s'
		);
	END $$;`, fk.Name, fk.Table, fk.Column, fk.RefTable, fk.RefColumn, fk.OnUpdate, fk.OnDelete)
}

var foreignKeys = []foreignKeySpec{
	{Name: "fk_user_otps_user_cascade", Table: "user_otps", Column: "user_id", RefTable: "users", RefColumn: "id", OnUpdate: "CASCADE", OnDelete: "CASCADE"},
	{Name: "fk_user_referrals_user_cascade", Table: "user_referrals", Column: "user_id", RefTable: "users", RefColumn: "id", OnUpdate: "CASCADE", OnDelete: "CASCADE"},

	{Name: "fk_wallets_user_cascade", Table: "wallets", Column: "user_id", RefTable: "users", RefColumn: "id", OnUpdate: "CASCADE", OnDelete: "CASCADE"},
	{Name: "fk_wallet_targets_wallet_cascade", Table: "wallet_targets", Column: "wallet_id", RefTable: "wallets", RefColumn: "id", OnUpdate: "CASCADE", OnDelete: "CASCADE"},

	{Name: "fk_categories_user_cascade", Table: "categories", Column: "user_id", RefTable: "users", RefColumn: "id", OnUpdate: "CASCADE", OnDelete: "CASCADE"},
	{Name: "fk_transactions_wallet_cascade", Table: "transactions", Column: "wallet_id", RefTable: "wallets", RefColumn: "id", OnUpdate: "CASCADE", OnDelete: "CASCADE"},
	{Name: "fk_transactions_category_cascade", Table: "transactions", Column: "category_id", RefTable: "categories", RefColumn: "id", OnUpdate: "CASCADE", OnDelete: "CASCADE"},

	{Name: "fk_budgets_user_cascade", Table: "budgets", Column: "user_id", RefTable: "users", RefColumn: "id", OnUpdate: "CASCADE", OnDelete: "CASCADE"},
	{Name: "fk_budgets_wallet_cascade", Table: "budgets", Column: "wallet_id", RefTable: "wallets", RefColumn: "id", OnUpdate: "CASCADE", OnDelete: "CASCADE"},
	{Name: "fk_budgets_category_cascade", Table: "budgets", Column: "category_id", RefTable: "categories", RefColumn: "id", OnUpdate: "CASCADE", OnDelete: "CASCADE"},

	{Name: "fk_savings_goals_user_cascade", Table: "savings_goals", Column: "user_id", RefTable: "users", RefColumn: "id", OnUpdate: "CASCADE", OnDelete: "CASCADE"},
	{Name: "fk_savings_goals_wallet_cascade", Table: "savings_goals", Column: "wallet_id", RefTable: "wallets", RefColumn: "id", OnUpdate: "CASCADE", OnDelete: "CASCADE"},
	{Name: "fk_savings_goal_contributions_goal_cascade", Table: "savings_goal_contributions", Column: "goal_id", RefTable: "savings_goals", RefColumn: "id", OnUpdate: "CASCADE", OnDelete: "CASCADE"},

	{Name: "fk_upcoming_billings_user_cascade", Table: "upcoming_billings", Column: "user_id", RefTable: "users", RefColumn: "id", OnUpdate: "CASCADE", OnDelete: "CASCADE"},
	{Name: "fk_notifications_user_cascade", Table: "notifications", Column: "user_id", RefTable: "users", RefColumn: "id", OnUpdate: "CASCADE", OnDelete: "CASCADE"},
	{Name: "fk_ai_processing_logs_user_cascade", Table: "ai_processing_logs", Column: "user_id", RefTable: "users", RefColumn: "id", OnUpdate: "CASCADE", OnDelete: "CASCADE"},

	{Name: "fk_subscriptions_user_cascade", Table: "subscriptions", Column: "user_id", RefTable: "users", RefColumn: "id", OnUpdate: "CASCADE", OnDelete: "CASCADE"},
	{Name: "fk_subscriptions_plan_cascade", Table: "subscriptions", Column: "plan_id", RefTable: "plans", RefColumn: "id", OnUpdate: "CASCADE", OnDelete: "CASCADE"},
	{Name: "fk_subscriptions_referrer_set_null", Table: "subscriptions", Column: "referrer_id", RefTable: "users", RefColumn: "id", OnUpdate: "CASCADE", OnDelete: "SET NULL"},

	{Name: "fk_split_bills_owner_user_cascade", Table: "split_bills", Column: "owner_user_id", RefTable: "users", RefColumn: "id", OnUpdate: "CASCADE", OnDelete: "CASCADE"},
	{Name: "fk_split_bill_participants_split_bill_cascade", Table: "split_bill_participants", Column: "split_bill_id", RefTable: "split_bills", RefColumn: "id", OnUpdate: "CASCADE", OnDelete: "CASCADE"},
}

var indexStatements = []string{
	`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email
		ON users (email) WHERE deleted_at IS NULL`,
	`CREATE INDEX IF NOT EXISTS idx_users_status ON users (status)`,
	`CREATE INDEX IF NOT EXISTS idx_users_auth_provider ON users (auth_provider)`,

	`CREATE UNIQUE INDEX IF NOT EXISTS idx_user_referrals_code ON user_referrals (code)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS idx_user_referrals_user ON user_referrals (user_id)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS idx_user_otps_user_purpose ON user_otps (user_id, purpose)`,
	`CREATE INDEX IF NOT EXISTS idx_user_otps_expires_at ON user_otps (expires_at)`,

	`CREATE UNIQUE INDEX IF NOT EXISTS idx_wallets_user_default
		ON wallets (user_id) WHERE is_default = true AND deleted_at IS NULL`,
	`CREATE INDEX IF NOT EXISTS idx_wallets_user_type
		ON wallets (user_id, type) WHERE deleted_at IS NULL`,
	`CREATE INDEX IF NOT EXISTS idx_wallets_user_id
		ON wallets (user_id, id) WHERE deleted_at IS NULL`,
	`CREATE UNIQUE INDEX IF NOT EXISTS idx_wallet_targets_wallet ON wallet_targets (wallet_id)`,

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
	`CREATE INDEX IF NOT EXISTS idx_transactions_date_wallet
		ON transactions (transaction_date DESC, wallet_id) WHERE deleted_at IS NULL`,
	`CREATE INDEX IF NOT EXISTS idx_transactions_type
		ON transactions (type) WHERE deleted_at IS NULL`,
	`CREATE INDEX IF NOT EXISTS idx_transactions_source
		ON transactions (source) WHERE deleted_at IS NULL`,

	`CREATE INDEX IF NOT EXISTS idx_budgets_user_period
		ON budgets (user_id, period) WHERE deleted_at IS NULL`,
	`CREATE INDEX IF NOT EXISTS idx_budgets_wallet_category
		ON budgets (wallet_id, category_id) WHERE deleted_at IS NULL`,

	`CREATE INDEX IF NOT EXISTS idx_savings_goals_user
		ON savings_goals (user_id) WHERE deleted_at IS NULL`,
	`CREATE INDEX IF NOT EXISTS idx_savings_goals_wallet
		ON savings_goals (wallet_id) WHERE wallet_id IS NOT NULL AND deleted_at IS NULL`,
	`CREATE INDEX IF NOT EXISTS idx_savings_goal_contributions_goal
		ON savings_goal_contributions (goal_id)`,

	`CREATE INDEX IF NOT EXISTS idx_upcoming_billings_user_due
		ON upcoming_billings (user_id, due_date) WHERE deleted_at IS NULL`,
	`CREATE INDEX IF NOT EXISTS idx_notifications_user_created
		ON notifications (user_id, created_at DESC)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS idx_notifications_user_ref_type
		ON notifications (user_id, ref_type, ref_id, type)
		WHERE ref_type <> '' AND ref_id <> ''`,

	`CREATE INDEX IF NOT EXISTS idx_subscriptions_user ON subscriptions (user_id)`,
	`CREATE INDEX IF NOT EXISTS idx_subscriptions_plan ON subscriptions (plan_id)`,
	`CREATE INDEX IF NOT EXISTS idx_subscriptions_status ON subscriptions (status)`,
	`CREATE INDEX IF NOT EXISTS idx_subscriptions_trial_ends ON subscriptions (trial_ends_at) WHERE trial_ends_at IS NOT NULL`,
	`CREATE INDEX IF NOT EXISTS idx_subscriptions_referrer
		ON subscriptions (referrer_id) WHERE referrer_id IS NOT NULL`,

	`CREATE INDEX IF NOT EXISTS idx_split_bills_owner
		ON split_bills (owner_user_id) WHERE deleted_at IS NULL`,
	`CREATE INDEX IF NOT EXISTS idx_split_bill_participants_bill
		ON split_bill_participants (split_bill_id)`,
}
