package auth

import (
	"context"
	"errors"

	"github.com/ganiramadhan/starter-go/internal/domain"
	"github.com/ganiramadhan/starter-go/internal/dto"
	"github.com/ganiramadhan/starter-go/internal/modules/user"
	"github.com/ganiramadhan/starter-go/pkg/jwt"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type Service interface {
	Login(ctx context.Context, req dto.LoginRequest) (*dto.AuthResponse, error)
	Register(ctx context.Context, req dto.RegisterRequest) (*dto.AuthResponse, error)
	ChangePassword(ctx context.Context, userID uuid.UUID, req dto.ChangePasswordRequest) error
}

type service struct {
	users user.Repository
	jwt   *jwt.Manager
}

func NewService(users user.Repository, j *jwt.Manager) Service {
	return &service{users: users, jwt: j}
}

func (s *service) Login(_ context.Context, req dto.LoginRequest) (*dto.AuthResponse, error) {
	u, err := s.users.FindByEmail(req.Email)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.ErrInvalidCredentials
		}
		return nil, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(req.Password)); err != nil {
		return nil, domain.ErrInvalidCredentials
	}
	token, err := s.jwt.Generate(u.ID, u.Email, u.Role)
	if err != nil {
		return nil, err
	}
	return &dto.AuthResponse{
		Token: token,
		User:  dto.UserResponse{ID: u.ID, Name: u.Name, Email: u.Email, Role: u.Role},
	}, nil
}

func (s *service) Register(_ context.Context, req dto.RegisterRequest) (*dto.AuthResponse, error) {
	if existing, _ := s.users.FindByEmail(req.Email); existing != nil {
		return nil, domain.ErrAlreadyExists
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	u := domain.User{
		Name:     req.Name,
		Email:    req.Email,
		Password: string(hashed),
		Role:     "user",
	}
	if err := s.users.Create(&u); err != nil {
		return nil, err
	}
	token, err := s.jwt.Generate(u.ID, u.Email, u.Role)
	if err != nil {
		return nil, err
	}
	return &dto.AuthResponse{
		Token: token,
		User:  dto.UserResponse{ID: u.ID, Name: u.Name, Email: u.Email, Role: u.Role},
	}, nil
}

func (s *service) ChangePassword(_ context.Context, userID uuid.UUID, req dto.ChangePasswordRequest) error {
	u, err := s.users.FindByID(userID)
	if err != nil {
		return err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(req.CurrentPassword)); err != nil {
		return domain.ErrInvalidCredentials
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.Password = string(hashed)
	return s.users.Update(u)
}
