package support

import (
	"github.com/ganiramadhan/starter-go/internal/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Repository interface {
	List(userID *uuid.UUID, status string) ([]domain.SupportTicket, error)
	Find(id uuid.UUID) (*domain.SupportTicket, error)
	Create(ticket *domain.SupportTicket, message *domain.SupportMessage) error
	AddMessage(message *domain.SupportMessage, status string) error
	UpdateStatus(id uuid.UUID, status string) error
}

type repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) Repository { return &repository{db: db} }

func (r *repository) List(userID *uuid.UUID, status string) ([]domain.SupportTicket, error) {
	var rows []domain.SupportTicket
	q := r.db.Preload("User").Preload("Messages", func(db *gorm.DB) *gorm.DB {
		return db.Order("created_at ASC")
	})
	if userID != nil {
		q = q.Where("user_id = ?", *userID)
	}
	if status != "" && status != "all" {
		q = q.Where("status = ?", status)
	}
	err := q.Order("updated_at DESC").Find(&rows).Error
	return rows, err
}

func (r *repository) Find(id uuid.UUID) (*domain.SupportTicket, error) {
	var row domain.SupportTicket
	err := r.db.Preload("User").Preload("Messages", func(db *gorm.DB) *gorm.DB {
		return db.Order("created_at ASC")
	}).Where("id = ?", id).First(&row).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &row, nil
}

func (r *repository) Create(ticket *domain.SupportTicket, message *domain.SupportMessage) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(ticket).Error; err != nil {
			return err
		}
		message.TicketID = ticket.ID
		return tx.Create(message).Error
	})
}

func (r *repository) AddMessage(message *domain.SupportMessage, status string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(message).Error; err != nil {
			return err
		}
		return tx.Model(&domain.SupportTicket{}).
			Where("id = ?", message.TicketID).
			Updates(map[string]any{"status": status, "updated_at": gorm.Expr("NOW()")}).Error
	})
}

func (r *repository) UpdateStatus(id uuid.UUID, status string) error {
	return r.db.Model(&domain.SupportTicket{}).Where("id = ?", id).Update("status", status).Error
}
