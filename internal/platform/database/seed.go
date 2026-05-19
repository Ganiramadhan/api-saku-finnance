package database

import (
	"log"

	"github.com/ganiramadhan/starter-go/internal/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func SeedSystemCategories(db *gorm.DB) error {
	systemCategories := []domain.Category{
		// ──── EXPENSE CATEGORIES ────
		{
			ID:       uuid.MustParse("10000000-0000-0000-0000-000000000001"),
			UserID:   nil,
			Name:     "Makanan & Minuman",
			Type:     domain.TxnTypeExpense,
			Icon:     "utensils",
			Color:    "#F59E0B",
			IsSystem: true,
		},
		{
			ID:       uuid.MustParse("10000000-0000-0000-0000-000000000002"),
			UserID:   nil,
			Name:     "Transportasi",
			Type:     domain.TxnTypeExpense,
			Icon:     "car",
			Color:    "#3B82F6",
			IsSystem: true,
		},
		{
			ID:       uuid.MustParse("10000000-0000-0000-0000-000000000003"),
			UserID:   nil,
			Name:     "Belanja",
			Type:     domain.TxnTypeExpense,
			Icon:     "shopping-bag",
			Color:    "#EC4899",
			IsSystem: true,
		},
		{
			ID:       uuid.MustParse("10000000-0000-0000-0000-000000000004"),
			UserID:   nil,
			Name:     "Tagihan",
			Type:     domain.TxnTypeExpense,
			Icon:     "receipt",
			Color:    "#EF4444",
			IsSystem: true,
		},
		{
			ID:       uuid.MustParse("10000000-0000-0000-0000-000000000005"),
			UserID:   nil,
			Name:     "Hiburan",
			Type:     domain.TxnTypeExpense,
			Icon:     "film",
			Color:    "#8B5CF6",
			IsSystem: true,
		},
		{
			ID:       uuid.MustParse("10000000-0000-0000-0000-000000000006"),
			UserID:   nil,
			Name:     "Kesehatan",
			Type:     domain.TxnTypeExpense,
			Icon:     "heart",
			Color:    "#10B981",
			IsSystem: true,
		},
		{
			ID:       uuid.MustParse("10000000-0000-0000-0000-000000000007"),
			UserID:   nil,
			Name:     "Pendidikan",
			Type:     domain.TxnTypeExpense,
			Icon:     "book",
			Color:    "#6366F1",
			IsSystem: true,
		},
		{
			ID:       uuid.MustParse("10000000-0000-0000-0000-000000000008"),
			UserID:   nil,
			Name:     "Rumah",
			Type:     domain.TxnTypeExpense,
			Icon:     "home",
			Color:    "#14B8A6",
			IsSystem: true,
		},
		{
			ID:       uuid.MustParse("10000000-0000-0000-0000-000000000009"),
			UserID:   nil,
			Name:     "Pakaian",
			Type:     domain.TxnTypeExpense,
			Icon:     "shirt",
			Color:    "#F97316",
			IsSystem: true,
		},
		{
			ID:       uuid.MustParse("10000000-0000-0000-0000-000000000010"),
			UserID:   nil,
			Name:     "Lainnya",
			Type:     domain.TxnTypeExpense,
			Icon:     "ellipsis-h",
			Color:    "#64748B",
			IsSystem: true,
		},

		// ──── INCOME CATEGORIES ────
		{
			ID:       uuid.MustParse("10000000-0000-0000-0000-000000000101"),
			UserID:   nil,
			Name:     "Gaji",
			Type:     domain.TxnTypeIncome,
			Icon:     "wallet",
			Color:    "#10B981",
			IsSystem: true,
		},
		{
			ID:       uuid.MustParse("10000000-0000-0000-0000-000000000102"),
			UserID:   nil,
			Name:     "Bonus",
			Type:     domain.TxnTypeIncome,
			Icon:     "gift",
			Color:    "#F59E0B",
			IsSystem: true,
		},
		{
			ID:       uuid.MustParse("10000000-0000-0000-0000-000000000103"),
			UserID:   nil,
			Name:     "Freelance",
			Type:     domain.TxnTypeIncome,
			Icon:     "briefcase",
			Color:    "#3B82F6",
			IsSystem: true,
		},
		{
			ID:       uuid.MustParse("10000000-0000-0000-0000-000000000104"),
			UserID:   nil,
			Name:     "Investasi",
			Type:     domain.TxnTypeIncome,
			Icon:     "trending-up",
			Color:    "#8B5CF6",
			IsSystem: true,
		},
		{
			ID:       uuid.MustParse("10000000-0000-0000-0000-000000000105"),
			UserID:   nil,
			Name:     "Hadiah",
			Type:     domain.TxnTypeIncome,
			Icon:     "gift",
			Color:    "#EC4899",
			IsSystem: true,
		},
		{
			ID:       uuid.MustParse("10000000-0000-0000-0000-000000000106"),
			UserID:   nil,
			Name:     "Penjualan",
			Type:     domain.TxnTypeIncome,
			Icon:     "shopping-cart",
			Color:    "#14B8A6",
			IsSystem: true,
		},
		{
			ID:       uuid.MustParse("10000000-0000-0000-0000-000000000107"),
			UserID:   nil,
			Name:     "Bunga Bank",
			Type:     domain.TxnTypeIncome,
			Icon:     "bank",
			Color:    "#6366F1",
			IsSystem: true,
		},
		{
			ID:       uuid.MustParse("10000000-0000-0000-0000-000000000108"),
			UserID:   nil,
			Name:     "Cashback",
			Type:     domain.TxnTypeIncome,
			Icon:     "credit-card",
			Color:    "#F97316",
			IsSystem: true,
		},
		{
			ID:       uuid.MustParse("10000000-0000-0000-0000-000000000109"),
			UserID:   nil,
			Name:     "Lainnya",
			Type:     domain.TxnTypeIncome,
			Icon:     "ellipsis-h",
			Color:    "#64748B",
			IsSystem: true,
		},
	}

	for _, cat := range systemCategories {
		var existing domain.Category
		err := db.Where("id = ?", cat.ID).First(&existing).Error
		if err == gorm.ErrRecordNotFound {
			if err := db.Create(&cat).Error; err != nil {
				log.Printf("Failed to create system category %s: %v", cat.Name, err)
				return err
			}
			log.Printf("✓ Created system category: %s", cat.Name)
		} else if err != nil {
			return err
		}
		// else: already exists, skip
	}

	log.Println("✓ System categories seeded successfully")
	return nil
}
