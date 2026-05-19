package category

import (
	"context"

	"github.com/ganiramadhan/starter-go/internal/domain"
	"github.com/ganiramadhan/starter-go/internal/dto"
	"github.com/google/uuid"
)

type Service interface {
	List(ctx context.Context, userID uuid.UUID, txType string) ([]dto.CategoryResponse, error)
	Get(ctx context.Context, userID, id uuid.UUID) (*dto.CategoryResponse, error)
	Create(ctx context.Context, userID uuid.UUID, req dto.CreateCategoryRequest) (*dto.CategoryResponse, error)
	Update(ctx context.Context, userID, id uuid.UUID, req dto.UpdateCategoryRequest) (*dto.CategoryResponse, error)
	Delete(ctx context.Context, userID, id uuid.UUID) error
}

type service struct{ repo Repository }

func NewService(r Repository) Service { return &service{repo: r} }

func toResp(c domain.Category) dto.CategoryResponse {
	return dto.CategoryResponse{
		ID: c.ID, UserID: c.UserID, Name: c.Name, Type: c.Type,
		Icon: c.Icon, Color: c.Color, IsSystem: c.IsSystem,
	}
}

func (s *service) List(_ context.Context, userID uuid.UUID, txType string) ([]dto.CategoryResponse, error) {
	rows, err := s.repo.List(userID, txType)
	if err != nil {
		return nil, err
	}
	out := make([]dto.CategoryResponse, 0, len(rows))
	for _, c := range rows {
		out = append(out, toResp(c))
	}
	return out, nil
}

func (s *service) Get(_ context.Context, userID, id uuid.UUID) (*dto.CategoryResponse, error) {
	c, err := s.repo.FindAccessible(userID, id)
	if err != nil {
		return nil, err
	}
	r := toResp(*c)
	return &r, nil
}

func (s *service) Create(_ context.Context, userID uuid.UUID, req dto.CreateCategoryRequest) (*dto.CategoryResponse, error) {
	uid := userID
	c := domain.Category{
		ID: uuid.New(), UserID: &uid, Name: req.Name, Type: req.Type,
		Icon: req.Icon, Color: req.Color, IsSystem: false,
	}
	if err := s.repo.Create(&c); err != nil {
		return nil, err
	}
	r := toResp(c)
	return &r, nil
}

func (s *service) Update(_ context.Context, userID, id uuid.UUID, req dto.UpdateCategoryRequest) (*dto.CategoryResponse, error) {
	c, err := s.repo.FindForUser(userID, id) // system categories not editable
	if err != nil {
		return nil, err
	}
	if req.Name != "" {
		c.Name = req.Name
	}
	if req.Type != "" {
		c.Type = req.Type
	}
	if req.Icon != "" {
		c.Icon = req.Icon
	}
	if req.Color != "" {
		c.Color = req.Color
	}
	if err := s.repo.Update(c); err != nil {
		return nil, err
	}
	r := toResp(*c)
	return &r, nil
}

func (s *service) Delete(_ context.Context, userID, id uuid.UUID) error {
	return s.repo.Delete(userID, id)
}
