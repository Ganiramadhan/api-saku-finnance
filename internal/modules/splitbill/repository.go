package splitbill

import (
	"github.com/ganiramadhan/starter-go/internal/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Repository interface {
	List(ownerID uuid.UUID) ([]domain.SplitBill, error)
	FindByID(ownerID, id uuid.UUID) (*domain.SplitBill, error)
	Create(b *domain.SplitBill) error
	Update(b *domain.SplitBill) error
	ReplaceParticipants(billID uuid.UUID, participants []domain.SplitBillParticipant) error
	Delete(ownerID, id uuid.UUID) error
	MarkParticipantPaid(ownerID, billID, partID uuid.UUID, paid bool) error
}

type repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) Repository { return &repository{db: db} }

func (r *repository) List(ownerID uuid.UUID) ([]domain.SplitBill, error) {
	var rows []domain.SplitBill
	err := r.db.
		Preload("Participants").
		Where("owner_user_id = ?", ownerID).
		Order("created_at DESC").
		Find(&rows).Error
	return rows, err
}

func (r *repository) FindByID(ownerID, id uuid.UUID) (*domain.SplitBill, error) {
	var b domain.SplitBill
	if err := r.db.
		Preload("Participants").
		Where("id = ? AND owner_user_id = ?", id, ownerID).
		First(&b).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &b, nil
}

func (r *repository) Create(b *domain.SplitBill) error {
	return r.db.Create(b).Error
}

func (r *repository) Update(b *domain.SplitBill) error {
	return r.db.Model(b).
		Select("Title", "TotalAmount", "Currency", "Notes").
		Updates(b).Error
}

func (r *repository) ReplaceParticipants(billID uuid.UUID, participants []domain.SplitBillParticipant) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("split_bill_id = ?", billID).Delete(&domain.SplitBillParticipant{}).Error; err != nil {
			return err
		}
		if len(participants) == 0 {
			return nil
		}
		for i := range participants {
			participants[i].SplitBillID = billID
		}
		return tx.Create(&participants).Error
	})
}

func (r *repository) Delete(ownerID, id uuid.UUID) error {
	res := r.db.Where("id = ? AND owner_user_id = ?", id, ownerID).Delete(&domain.SplitBill{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *repository) MarkParticipantPaid(ownerID, billID, partID uuid.UUID, paid bool) error {
	// Ensure ownership first.
	var bill domain.SplitBill
	if err := r.db.Select("id").Where("id = ? AND owner_user_id = ?", billID, ownerID).First(&bill).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return domain.ErrNotFound
		}
		return err
	}
	updates := map[string]any{"paid_at": nil}
	if paid {
		updates["paid_at"] = gorm.Expr("NOW()")
	}
	res := r.db.Model(&domain.SplitBillParticipant{}).
		Where("id = ? AND split_bill_id = ?", partID, billID).
		Updates(updates)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}
