package savingsgoal

import (
	"context"
	"math"
	"time"

	"github.com/ganiramadhan/starter-go/internal/domain"
	"github.com/ganiramadhan/starter-go/internal/dto"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Service interface {
	List(ctx context.Context, userID uuid.UUID) ([]dto.SavingsGoalResponse, error)
	Get(ctx context.Context, userID, id uuid.UUID) (*dto.SavingsGoalResponse, error)
	Create(ctx context.Context, userID uuid.UUID, req dto.CreateSavingsGoalRequest) (*dto.SavingsGoalResponse, error)
	Update(ctx context.Context, userID, id uuid.UUID, req dto.UpdateSavingsGoalRequest) (*dto.SavingsGoalResponse, error)
	Delete(ctx context.Context, userID, id uuid.UUID) error

	Contribute(ctx context.Context, userID, id uuid.UUID, req dto.ContributeSavingsGoalRequest) (*dto.SavingsGoalResponse, error)
	ListContributions(ctx context.Context, userID, id uuid.UUID) ([]dto.SavingsGoalContributionResponse, error)
}

type service struct{ repo Repository }

func NewService(r Repository) Service { return &service{repo: r} }

func toResp(g domain.SavingsGoal) dto.SavingsGoalResponse {
	remaining := g.TargetAmount - g.CurrentAmount
	if remaining < 0 {
		remaining = 0
	}
	pct := 0.0
	if g.TargetAmount > 0 {
		pct = math.Min(100, (g.CurrentAmount/g.TargetAmount)*100)
	}
	var daysLeft *int
	if g.Deadline != nil {
		d := int(math.Ceil(time.Until(*g.Deadline).Hours() / 24))
		if d < 0 {
			d = 0
		}
		daysLeft = &d
	}
	return dto.SavingsGoalResponse{
		ID: g.ID, UserID: g.UserID, WalletID: g.WalletID,
		Name: g.Name, Description: g.Description,
		TargetAmount: g.TargetAmount, CurrentAmount: g.CurrentAmount,
		Remaining: remaining, ProgressPct: pct,
		Deadline: g.Deadline, DaysLeft: daysLeft,
		Icon: g.Icon, Color: g.Color,
		CompletedAt: g.CompletedAt,
		CreatedAt:   g.CreatedAt, UpdatedAt: g.UpdatedAt,
	}
}

func (s *service) List(_ context.Context, userID uuid.UUID) ([]dto.SavingsGoalResponse, error) {
	rows, err := s.repo.List(userID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.SavingsGoalResponse, 0, len(rows))
	for _, g := range rows {
		out = append(out, toResp(g))
	}
	return out, nil
}

func (s *service) Get(_ context.Context, userID, id uuid.UUID) (*dto.SavingsGoalResponse, error) {
	g, err := s.repo.FindByID(userID, id)
	if err != nil {
		return nil, err
	}
	r := toResp(*g)
	return &r, nil
}

func (s *service) Create(_ context.Context, userID uuid.UUID, req dto.CreateSavingsGoalRequest) (*dto.SavingsGoalResponse, error) {
	g := domain.SavingsGoal{
		ID: uuid.New(), UserID: userID, WalletID: req.WalletID,
		Name: req.Name, Description: req.Description,
		TargetAmount: req.TargetAmount, CurrentAmount: 0,
		Deadline: req.Deadline, Icon: req.Icon, Color: req.Color,
	}
	if err := s.repo.Create(&g); err != nil {
		return nil, err
	}
	r := toResp(g)
	return &r, nil
}

func (s *service) Update(_ context.Context, userID, id uuid.UUID, req dto.UpdateSavingsGoalRequest) (*dto.SavingsGoalResponse, error) {
	g, err := s.repo.FindByID(userID, id)
	if err != nil {
		return nil, err
	}
	if req.Name != nil {
		g.Name = *req.Name
	}
	if req.Description != nil {
		g.Description = *req.Description
	}
	if req.TargetAmount != nil {
		g.TargetAmount = *req.TargetAmount
	}
	if req.Deadline != nil {
		g.Deadline = req.Deadline
	}
	if req.WalletID != nil {
		g.WalletID = req.WalletID
	}
	if req.Icon != nil {
		g.Icon = *req.Icon
	}
	if req.Color != nil {
		g.Color = *req.Color
	}
	// Re-evaluate completion
	if g.CurrentAmount >= g.TargetAmount && g.CompletedAt == nil {
		now := time.Now()
		g.CompletedAt = &now
	}
	if g.CurrentAmount < g.TargetAmount {
		g.CompletedAt = nil
	}
	if err := s.repo.Update(g); err != nil {
		return nil, err
	}
	r := toResp(*g)
	return &r, nil
}

func (s *service) Delete(_ context.Context, userID, id uuid.UUID) error {
	return s.repo.Delete(userID, id)
}

func (s *service) Contribute(_ context.Context, userID, id uuid.UUID, req dto.ContributeSavingsGoalRequest) (*dto.SavingsGoalResponse, error) {
	g, err := s.repo.FindByID(userID, id)
	if err != nil {
		return nil, err
	}
	err = s.repo.DB().Transaction(func(tx *gorm.DB) error {
		c := domain.SavingsGoalContribution{
			ID: uuid.New(), GoalID: g.ID,
			Amount: req.Amount, Source: "manual", Note: req.Note,
		}
		if err := tx.Create(&c).Error; err != nil {
			return err
		}
		g.CurrentAmount += req.Amount
		if g.CurrentAmount >= g.TargetAmount && g.CompletedAt == nil {
			now := time.Now()
			g.CompletedAt = &now
		}
		return tx.Save(g).Error
	})
	if err != nil {
		return nil, err
	}
	r := toResp(*g)
	return &r, nil
}

func (s *service) ListContributions(_ context.Context, userID, id uuid.UUID) ([]dto.SavingsGoalContributionResponse, error) {
	if _, err := s.repo.FindByID(userID, id); err != nil {
		return nil, err
	}
	rows, err := s.repo.ListContributions(id)
	if err != nil {
		return nil, err
	}
	out := make([]dto.SavingsGoalContributionResponse, 0, len(rows))
	for _, c := range rows {
		out = append(out, dto.SavingsGoalContributionResponse{
			ID: c.ID, GoalID: c.GoalID,
			Amount: c.Amount, Source: c.Source, Note: c.Note,
			CreatedAt: c.CreatedAt,
		})
	}
	return out, nil
}
