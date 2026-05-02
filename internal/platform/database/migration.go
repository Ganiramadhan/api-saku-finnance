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

	if err := db.AutoMigrate(
		&domain.User{},
	); err != nil {
		return fmt.Errorf("auto migrate: %w", err)
	}

	indexes := []string{
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email ON users(email) WHERE deleted_at IS NULL`,
		`CREATE INDEX IF NOT EXISTS idx_users_deleted_at ON users(deleted_at)`,
	}
	for _, idx := range indexes {
		if err := db.Exec(idx).Error; err != nil {
			return fmt.Errorf("create index: %w", err)
		}
	}
	return nil
}
