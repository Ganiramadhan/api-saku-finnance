package notification

import (
	"context"
	"time"

	"github.com/ganiramadhan/starter-go/internal/domain"
	"github.com/google/uuid"
)

type Response struct {
	ID        uuid.UUID  `json:"id"`
	Type      string     `json:"type"`
	Title     string     `json:"title"`
	Message   string     `json:"message"`
	RefType   string     `json:"ref_type"`
	RefID     string     `json:"ref_id"`
	ReadAt    *time.Time `json:"read_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

type Service interface {
	List(ctx context.Context, userID uuid.UUID, limit int) ([]Response, error)
	Create(ctx context.Context, n domain.Notification) error
	MarkRead(ctx context.Context, userID, id uuid.UUID) error
	MarkAllRead(ctx context.Context, userID uuid.UUID) error
}

type service struct{ repo Repository }

func NewService(repo Repository) Service { return &service{repo: repo} }

func toResponse(n domain.Notification) Response {
	return Response{
		ID:        n.ID,
		Type:      n.Type,
		Title:     n.Title,
		Message:   n.Message,
		RefType:   n.RefType,
		RefID:     n.RefID,
		ReadAt:    n.ReadAt,
		CreatedAt: n.CreatedAt,
	}
}

func (s *service) List(_ context.Context, userID uuid.UUID, limit int) ([]Response, error) {
	rows, err := s.repo.List(userID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]Response, 0, len(rows))
	for _, row := range rows {
		out = append(out, toResponse(row))
	}
	return out, nil
}

func (s *service) Create(_ context.Context, n domain.Notification) error {
	return s.repo.Create(&n)
}

func (s *service) MarkRead(_ context.Context, userID, id uuid.UUID) error {
	return s.repo.MarkRead(userID, id)
}

func (s *service) MarkAllRead(_ context.Context, userID uuid.UUID) error {
	return s.repo.MarkAllRead(userID)
}
