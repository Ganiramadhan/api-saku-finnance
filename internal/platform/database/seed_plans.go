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
			Name:     "Basic",
			Price:    0,
			Currency: "IDR",
			Period:   domain.PlanPeriodMonthly,
			Features: `["Transaksi manual tanpa batas","Scan struk 3x per hari","Chat with AI 5x per hari","2 wallet","Budget target","Upcoming billing 3 item","AI insight basic"]`,
			IsActive: true,
			SortKey:  10,
		},
		{
			ID:       uuid.MustParse("20000000-0000-0000-0000-000000000002"),
			Code:     "pro",
			Name:     "Pro",
			Price:    29000,
			Currency: "IDR",
			Period:   domain.PlanPeriodMonthly,
			Features: `["Semua fitur Basic","Chat with AI 300 prompt/bulan","Scan struk 90x/bulan","10 wallet","Upcoming billing 20 item","Split bill","Insight dashboard lebih lengkap"]`,
			IsActive: true,
			SortKey:  20,
		},
		{
			ID:       uuid.MustParse("20000000-0000-0000-0000-000000000004"),
			Code:     "pro_yearly",
			Name:     "Pro Yearly",
			Price:    278400,
			Currency: "IDR",
			Period:   domain.PlanPeriodYearly,
			Features: `["Semua fitur Basic","Chat with AI 300 prompt/bulan","Scan struk 90x/bulan","10 wallet","Upcoming billing 20 item","Split bill","Insight dashboard lebih lengkap"]`,
			IsActive: true,
			SortKey:  21,
		},
		{
			ID:       uuid.MustParse("20000000-0000-0000-0000-000000000003"),
			Code:     "premium",
			Name:     "Premium",
			Price:    99000,
			Currency: "IDR",
			Period:   domain.PlanPeriodMonthly,
			Features: `["Semua fitur Pro","Chat with AI 1.200 prompt/bulan","Scan struk 300x/bulan","50 wallet","Upcoming billing 100 item","Export dan laporan lanjutan","Prioritas support"]`,
			IsActive: true,
			SortKey:  30,
		},
		{
			ID:       uuid.MustParse("20000000-0000-0000-0000-000000000005"),
			Code:     "premium_yearly",
			Name:     "Premium Yearly",
			Price:    950400,
			Currency: "IDR",
			Period:   domain.PlanPeriodYearly,
			Features: `["Semua fitur Pro","Chat with AI 1.200 prompt/bulan","Scan struk 300x/bulan","50 wallet","Upcoming billing 100 item","Export dan laporan lanjutan","Prioritas support"]`,
			IsActive: true,
			SortKey:  31,
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
