package user

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/ganiramadhan/starter-go/internal/domain"
	"github.com/ganiramadhan/starter-go/internal/dto"
	"github.com/ganiramadhan/starter-go/internal/platform/storage"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const (
	photoURLTTL         = 24 * time.Hour
	uploadFolderUserTmp = "Temp/Users"
	userPhotoTemplate   = "Users/%s/%s"
)

type Service interface {
	List(ctx context.Context, page, limit int, search string) ([]dto.UserResponse, *dto.PaginationMeta, error)
	Get(ctx context.Context, id uuid.UUID) (*dto.UserResponse, error)
	Create(ctx context.Context, req dto.CreateUserRequest) (*dto.UserResponse, error)
	Update(ctx context.Context, id uuid.UUID, req dto.UpdateUserRequest) (*dto.UserResponse, error)
	Delete(ctx context.Context, id uuid.UUID) error
	DeletePhoto(ctx context.Context, id uuid.UUID) (*dto.UserResponse, error)
}

type service struct {
	repo    Repository
	storage storage.Storage
}

func NewService(r Repository, s storage.Storage) Service {
	return &service{repo: r, storage: s}
}

func (s *service) List(ctx context.Context, page, limit int, search string) ([]dto.UserResponse, *dto.PaginationMeta, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}
	users, total, err := s.repo.FindAll(page, limit, search)
	if err != nil {
		return nil, nil, err
	}
	out := make([]dto.UserResponse, 0, len(users))
	for _, u := range users {
		out = append(out, s.toResponse(ctx, u))
	}
	return out, dto.NewMeta(page, limit, total), nil
}

func (s *service) Get(ctx context.Context, id uuid.UUID) (*dto.UserResponse, error) {
	u, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
	}
	r := s.toResponse(ctx, *u)
	return &r, nil
}

func (s *service) Create(ctx context.Context, req dto.CreateUserRequest) (*dto.UserResponse, error) {
	if existing, _ := s.repo.FindByEmail(req.Email); existing != nil {
		return nil, domain.ErrAlreadyExists
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	role := req.Role
	if role == "" {
		role = "user"
	}
	status := req.Status
	if status == "" {
		status = "active"
	}
	u := domain.User{
		ID:       uuid.New(),
		Name:     req.Name,
		Email:    req.Email,
		Phone:    req.Phone,
		Status:   status,
		Photo:    req.Photo,
		Password: string(hashed),
		Role:     role,
	}

	moved := false
	if strings.HasPrefix(u.Photo, uploadFolderUserTmp+"/") {
		permanent := buildUserPhotoKey(u.ID, u.Photo)
		if err := s.storage.Move(ctx, u.Photo, permanent); err != nil {
			return nil, fmt.Errorf("user: promote photo: %w", err)
		}
		u.Photo = permanent
		moved = true
	}

	if err := s.repo.Create(&u); err != nil {
		if moved {
			if derr := s.storage.Delete(ctx, u.Photo); derr != nil {
				log.Printf("user: rollback promoted photo failed: %v", derr)
			}
		}
		return nil, err
	}

	r := s.toResponse(ctx, u)
	return &r, nil
}

func (s *service) Update(ctx context.Context, id uuid.UUID, req dto.UpdateUserRequest) (*dto.UserResponse, error) {
	u, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
	}
	if req.Name != "" {
		u.Name = req.Name
	}
	if req.Email != "" {
		if existing, _ := s.repo.FindByEmail(req.Email); existing != nil && existing.ID != id {
			return nil, domain.ErrAlreadyExists
		}
		u.Email = req.Email
	}
	if req.Password != "" {
		hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}
		u.Password = string(hashed)
	}
	if req.Role != "" {
		u.Role = req.Role
	}
	if req.Phone != "" {
		u.Phone = req.Phone
	}
	if req.Status != "" {
		u.Status = req.Status
	}

	oldPhoto := u.Photo
	newPhoto := req.Photo
	promoted := false
	if newPhoto != "" && strings.HasPrefix(newPhoto, uploadFolderUserTmp+"/") {
		permanent := buildUserPhotoKey(u.ID, newPhoto)
		if err := s.storage.Move(ctx, newPhoto, permanent); err != nil {
			return nil, fmt.Errorf("user: promote photo: %w", err)
		}
		newPhoto = permanent
		promoted = true
	}
	if newPhoto != "" {
		u.Photo = newPhoto
	}

	if err := s.repo.Update(u); err != nil {
		if promoted {
			if derr := s.storage.Delete(ctx, newPhoto); derr != nil {
				log.Printf("user: rollback promoted photo failed: %v", derr)
			}
		}
		return nil, err
	}

	if newPhoto != "" && oldPhoto != "" && oldPhoto != newPhoto {
		if err := s.storage.Delete(ctx, oldPhoto); err != nil {
			log.Printf("user: failed to delete old photo: %v", err)
		}
	}

	r := s.toResponse(ctx, *u)
	return &r, nil
}

func (s *service) Delete(ctx context.Context, id uuid.UUID) error {
	u, err := s.repo.FindByID(id)
	if err != nil {
		return err
	}
	if u.Photo != "" {
		if err := s.storage.Delete(ctx, u.Photo); err != nil {
			log.Printf("user: failed to delete photo: %v", err)
		}
	}
	return s.repo.Delete(id)
}

func (s *service) DeletePhoto(ctx context.Context, id uuid.UUID) (*dto.UserResponse, error) {
	u, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
	}
	if u.Photo == "" {
		r := s.toResponse(ctx, *u)
		return &r, nil
	}
	if err := s.storage.Delete(ctx, u.Photo); err != nil {
		log.Printf("user: delete photo from storage failed: %v", err)
	}
	u.Photo = ""
	if err := s.repo.Update(u); err != nil {
		return nil, err
	}
	r := s.toResponse(ctx, *u)
	return &r, nil
}

func (s *service) toResponse(ctx context.Context, u domain.User) dto.UserResponse {
	resp := dto.UserResponse{
		ID:        u.ID,
		Name:      u.Name,
		Email:     u.Email,
		Role:      u.Role,
		Phone:     u.Phone,
		Status:    u.Status,
		Photo:     u.Photo,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
	if u.Referral != nil {
		resp.ReferralCode = u.Referral.Code
		resp.ReferralReward = u.Referral.Reward
	}
	if u.Photo != "" {
		if strings.HasPrefix(u.Photo, "http://") || strings.HasPrefix(u.Photo, "https://") {
			resp.PhotoURL = u.Photo
		} else if url, err := s.storage.PresignedURL(ctx, u.Photo, photoURLTTL); err == nil {
			resp.PhotoURL = url
		}
	}
	return resp
}

func buildUserPhotoKey(id uuid.UUID, srcKey string) string {
	filename := srcKey[strings.LastIndex(srcKey, "/")+1:]
	return fmt.Sprintf(userPhotoTemplate, id.String(), filename)
}
