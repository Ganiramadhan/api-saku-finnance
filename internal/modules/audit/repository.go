package audit

import (
	"time"

	"github.com/ganiramadhan/starter-go/internal/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ListFilter struct {
	Table       string     // e.g. "wallets", "transactions"
	RecordID    string     // primary key of the audited row
	ActorID     *uuid.UUID // who (application user)
	RequestID   string     // correlate every row a single HTTP request touched
	Action      string     // INSERT | UPDATE | DELETE
	DirectOnly  bool       // only rows with no application context (direct SQL)
	From        *time.Time
	To          *time.Time
	Page, Limit int
}

type Repository interface {
	List(f ListFilter) ([]domain.AuditLog, int64, error)
	FindByID(id uuid.UUID) (*domain.AuditLog, error)
	ForRecord(table, recordID string, page, limit int) ([]domain.AuditLog, int64, error)
}

type repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) Repository { return &repository{db: db} }

func paged(page, limit int) (int, int) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 200 {
		limit = 20
	}
	return page, limit
}

func (r *repository) List(f ListFilter) ([]domain.AuditLog, int64, error) {
	q := r.db.Model(&domain.AuditLog{})
	if f.Table != "" {
		q = q.Where("table_name = ?", f.Table)
	}
	if f.RecordID != "" {
		q = q.Where("record_id = ?", f.RecordID)
	}
	if f.ActorID != nil {
		q = q.Where("actor_id = ?", *f.ActorID)
	}
	if f.RequestID != "" {
		q = q.Where("request_id = ?", f.RequestID)
	}
	if f.Action != "" {
		q = q.Where("action = ?", f.Action)
	}
	if f.DirectOnly {
		q = q.Where("is_direct_db_change = true")
	}
	if f.From != nil {
		q = q.Where("created_at >= ?", *f.From)
	}
	if f.To != nil {
		q = q.Where("created_at <= ?", *f.To)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	page, limit := paged(f.Page, f.Limit)
	var rows []domain.AuditLog
	err := q.Order("created_at DESC").
		Limit(limit).Offset((page - 1) * limit).
		Find(&rows).Error
	return rows, total, err
}

func (r *repository) FindByID(id uuid.UUID) (*domain.AuditLog, error) {
	var row domain.AuditLog
	if err := r.db.Where("id = ?", id).First(&row).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &row, nil
}

func (r *repository) ForRecord(table, recordID string, page, limit int) ([]domain.AuditLog, int64, error) {
	return r.List(ListFilter{Table: table, RecordID: recordID, Page: page, Limit: limit})
}
