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
			Features: `["Unlimited Transactions","2 Wallets","Budget Targets","AI Chat 20/month","OCR 10/month","Basic AI Insights","3 Upcoming Billings"]`,
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
			Features: `["Everything in Free","Unlimited Wallets","AI Chat 300/month","OCR 100/month","Unlimited Upcoming Billings","Split Bill","Recurring Transactions","Export CSV/Excel","Rich AI Insights"]`,
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
			Features: `["Everything in Free","Unlimited Wallets","AI Chat 300/month","OCR 100/month","Unlimited Upcoming Billings","Split Bill","Recurring Transactions","Export CSV/Excel","Rich AI Insights"]`,
			IsActive: true,
			SortKey:  21,
		},
		{
			ID:       uuid.MustParse("20000000-0000-0000-0000-000000000003"),
			Code:     "premium",
			Name:     "Premium",
			Price:    59000,
			Currency: "IDR",
			Period:   domain.PlanPeriodMonthly,
			Features: `["Everything in Pro","AI Chat 1,200/month","OCR 300/month","Advanced Reports","PDF Export","Priority Support"]`,
			IsActive: true,
			SortKey:  30,
		},
		{
			ID:       uuid.MustParse("20000000-0000-0000-0000-000000000005"),
			Code:     "premium_yearly",
			Name:     "Premium Yearly",
			Price:    566400,
			Currency: "IDR",
			Period:   domain.PlanPeriodYearly,
			Features: `["Everything in Pro","AI Chat 1,200/month","OCR 300/month","Advanced Reports","PDF Export","Priority Support"]`,
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
