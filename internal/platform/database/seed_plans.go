package database

import (
	"log"

	"github.com/ganiramadhan/starter-go/internal/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func SeedPlans(db *gorm.DB) error {
	plans := []domain.Plan{
		{
			ID:       uuid.MustParse("20000000-0000-0000-0000-000000000001"),
			Code:     "free",
			Name:     "Free",
			Price:    0,
			Currency: "IDR",
			Period:   domain.PlanPeriodMonthly,
			Features: `["Pencatatan transaksi manual","2 dompet","Kategori dasar"]`,
			IsActive: true,
			SortKey:  10,
		},
		{
			ID:       uuid.MustParse("20000000-0000-0000-0000-000000000002"),
			Code:     "pro",
			Name:     "Pro",
			Price:    39000,
			Currency: "IDR",
			Period:   domain.PlanPeriodMonthly,
			Features: `["Semua fitur Free","Scan struk dengan AI","Catat via AI (free text)","Dompet & kategori tanpa batas","Kantong Tujuan","Anggaran bulanan"]`,
			IsActive: true,
			SortKey:  20,
		},
		// Business plan kept seeded but disabled — product scoped down to Free + Pro for now.
		{
			ID:       uuid.MustParse("20000000-0000-0000-0000-000000000003"),
			Code:     "business",
			Name:     "Business",
			Price:    99000,
			Currency: "IDR",
			Period:   domain.PlanPeriodMonthly,
			Features: `["Semua fitur Pro","Budget Tracker lanjutan","Lampiran & arsip","Export Excel","Prioritas support"]`,
			IsActive: false,
			SortKey:  30,
		},
	}
	for _, p := range plans {
		var existing domain.Plan
		err := db.Where("code = ?", p.Code).First(&existing).Error
		if err == gorm.ErrRecordNotFound {
			if err := db.Create(&p).Error; err != nil {
				return err
			}
			log.Printf("✓ Created plan: %s", p.Code)
			continue
		}
		if err != nil {
			return err
		}
		// Update existing — keep ID stable
		existing.Name = p.Name
		existing.Price = p.Price
		existing.Currency = p.Currency
		existing.Period = p.Period
		existing.Features = p.Features
		existing.IsActive = p.IsActive
		existing.SortKey = p.SortKey
		if err := db.Save(&existing).Error; err != nil {
			return err
		}
	}
	log.Println("✓ Subscription plans seeded")
	return nil
}
