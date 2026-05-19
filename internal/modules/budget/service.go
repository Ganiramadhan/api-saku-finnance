package budget

import (
	"context"
	"time"

	"github.com/ganiramadhan/starter-go/internal/domain"
	"github.com/ganiramadhan/starter-go/internal/dto"
	"github.com/ganiramadhan/starter-go/internal/modules/category"
	"github.com/ganiramadhan/starter-go/internal/modules/wallet"
	"github.com/google/uuid"
)

type Service interface {
	List(ctx context.Context, userID uuid.UUID) ([]dto.BudgetResponse, error)
	Get(ctx context.Context, userID, id uuid.UUID) (*dto.BudgetResponse, error)
	Create(ctx context.Context, userID uuid.UUID, req dto.CreateBudgetRequest) (*dto.BudgetResponse, error)
	Update(ctx context.Context, userID, id uuid.UUID, req dto.UpdateBudgetRequest) (*dto.BudgetResponse, error)
	Delete(ctx context.Context, userID, id uuid.UUID) error
}

type service struct {
	repo    Repository
	wallets wallet.Repository
	cats    category.Repository
}

func NewService(r Repository, w wallet.Repository, cats category.Repository) Service {
	return &service{repo: r, wallets: w, cats: cats}
}

func periodWindow(period string, now time.Time) (time.Time, time.Time) {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		loc = now.Location()
	}
	n := now.In(loc)
	switch period {
	case "daily":
		start := time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, loc)
		end := time.Date(n.Year(), n.Month(), n.Day(), 23, 59, 59, int(time.Second-time.Nanosecond), loc)
		return start, end
	case "weekly":
		// ISO week — Monday as start of week
		wd := int(n.Weekday())
		if wd == 0 {
			wd = 7
		}
		start := time.Date(n.Year(), n.Month(), n.Day()-(wd-1), 0, 0, 0, 0, loc)
		end := start.AddDate(0, 0, 7).Add(-time.Second)
		return start, end
	default: // monthly
		start := time.Date(n.Year(), n.Month(), 1, 0, 0, 0, 0, loc)
		end := start.AddDate(0, 1, 0).Add(-time.Second)
		return start, end
	}
}

func (s *service) toResp(b domain.Budget) dto.BudgetResponse {
	start, end := periodWindow(b.Period, time.Now())
	spent, _ := s.repo.SumSpent(b.UserID, b.WalletID, b.CategoryID, start, end)
	remaining := b.LimitAmount - spent
	return dto.BudgetResponse{
		ID: b.ID, UserID: b.UserID, WalletID: b.WalletID, CategoryID: b.CategoryID,
		LimitAmount: b.LimitAmount, Period: b.Period,
		Spent: spent, Remaining: remaining,
		PeriodStart: start, PeriodEnd: end,
		CreatedAt: b.CreatedAt, UpdatedAt: b.UpdatedAt,
	}
}

func (s *service) List(_ context.Context, userID uuid.UUID) ([]dto.BudgetResponse, error) {
	rows, err := s.repo.List(userID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.BudgetResponse, 0, len(rows))
	for _, b := range rows {
		out = append(out, s.toResp(b))
	}
	return out, nil
}

func (s *service) Get(_ context.Context, userID, id uuid.UUID) (*dto.BudgetResponse, error) {
	b, err := s.repo.FindByID(userID, id)
	if err != nil {
		return nil, err
	}
	r := s.toResp(*b)
	return &r, nil
}

func (s *service) Create(_ context.Context, userID uuid.UUID, req dto.CreateBudgetRequest) (*dto.BudgetResponse, error) {
	if _, err := s.wallets.FindByID(userID, req.WalletID); err != nil {
		return nil, err
	}
	if _, err := s.cats.FindAccessible(userID, req.CategoryID); err != nil {
		return nil, err
	}
	b := domain.Budget{
		ID:          uuid.New(),
		UserID:      userID,
		WalletID:    req.WalletID,
		CategoryID:  req.CategoryID,
		LimitAmount: req.LimitAmount,
		Period:      req.Period,
	}
	if err := s.repo.Create(&b); err != nil {
		return nil, err
	}
	r := s.toResp(b)
	return &r, nil
}

func (s *service) Update(_ context.Context, userID, id uuid.UUID, req dto.UpdateBudgetRequest) (*dto.BudgetResponse, error) {
	b, err := s.repo.FindByID(userID, id)
	if err != nil {
		return nil, err
	}
	if req.LimitAmount != nil {
		b.LimitAmount = *req.LimitAmount
	}
	if req.Period != "" {
		b.Period = req.Period
	}
	if err := s.repo.Update(b); err != nil {
		return nil, err
	}
	r := s.toResp(*b)
	return &r, nil
}

func (s *service) Delete(_ context.Context, userID, id uuid.UUID) error {
	return s.repo.Delete(userID, id)
}
