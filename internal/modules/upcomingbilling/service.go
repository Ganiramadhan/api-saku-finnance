package upcomingbilling

import (
	"context"

	"github.com/ganiramadhan/starter-go/internal/domain"
	"github.com/ganiramadhan/starter-go/internal/dto"
	"github.com/google/uuid"
)

type Service interface {
	List(ctx context.Context, userID uuid.UUID) ([]dto.UpcomingBillingResponse, error)
	Create(ctx context.Context, userID uuid.UUID, req dto.UpcomingBillingRequest) (*dto.UpcomingBillingResponse, error)
	Update(ctx context.Context, userID, id uuid.UUID, req dto.UpdateUpcomingBillingRequest) (*dto.UpcomingBillingResponse, error)
	Delete(ctx context.Context, userID, id uuid.UUID) error
}

type service struct{ repo Repository }

func NewService(repo Repository) Service { return &service{repo: repo} }

func toResp(b domain.UpcomingBilling) dto.UpcomingBillingResponse {
	return dto.UpcomingBillingResponse{
		ID:        b.ID,
		UserID:    b.UserID,
		Name:      b.Name,
		Provider:  b.Provider,
		Amount:    b.Amount,
		Currency:  b.Currency,
		Cycle:     b.Cycle,
		DueDate:   b.DueDate,
		Status:    b.Status,
		Notes:     b.Notes,
		CreatedAt: b.CreatedAt,
		UpdatedAt: b.UpdatedAt,
	}
}

func (s *service) List(_ context.Context, userID uuid.UUID) ([]dto.UpcomingBillingResponse, error) {
	rows, err := s.repo.List(userID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.UpcomingBillingResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, toResp(row))
	}
	return out, nil
}

func (s *service) Create(_ context.Context, userID uuid.UUID, req dto.UpcomingBillingRequest) (*dto.UpcomingBillingResponse, error) {
	currency := req.Currency
	if currency == "" {
		currency = "IDR"
	}
	status := req.Status
	if status == "" {
		status = domain.BillingStatusActive
	}
	row := domain.UpcomingBilling{
		UserID:   userID,
		Name:     req.Name,
		Provider: req.Provider,
		Amount:   req.Amount,
		Currency: currency,
		Cycle:    req.Cycle,
		DueDate:  req.DueDate,
		Status:   status,
		Notes:    req.Notes,
	}
	if err := s.repo.Create(&row); err != nil {
		return nil, err
	}
	resp := toResp(row)
	return &resp, nil
}

func (s *service) Update(_ context.Context, userID, id uuid.UUID, req dto.UpdateUpcomingBillingRequest) (*dto.UpcomingBillingResponse, error) {
	row, err := s.repo.FindByID(userID, id)
	if err != nil {
		return nil, err
	}
	if req.Name != nil {
		row.Name = *req.Name
	}
	if req.Provider != nil {
		row.Provider = *req.Provider
	}
	if req.Amount != nil {
		row.Amount = *req.Amount
	}
	if req.Currency != nil {
		row.Currency = *req.Currency
	}
	if req.Cycle != nil {
		row.Cycle = *req.Cycle
	}
	if req.DueDate != nil {
		row.DueDate = *req.DueDate
	}
	if req.Status != nil {
		row.Status = *req.Status
	}
	if req.Notes != nil {
		row.Notes = *req.Notes
	}
	if err := s.repo.Update(row); err != nil {
		return nil, err
	}
	resp := toResp(*row)
	return &resp, nil
}

func (s *service) Delete(_ context.Context, userID, id uuid.UUID) error {
	return s.repo.Delete(userID, id)
}
