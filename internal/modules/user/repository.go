package user

import (
	"strings"

	"github.com/ganiramadhan/starter-go/internal/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Repository interface {
	FindAll(page, limit int, search string) ([]domain.User, int64, error)
	FindByID(id uuid.UUID) (*domain.User, error)
	FindByEmail(email string) (*domain.User, error)
	Create(u *domain.User) error
	Update(u *domain.User) error
	Delete(id uuid.UUID) error
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) FindAll(page, limit int, search string) ([]domain.User, int64, error) {
	var users []domain.User
	var total int64
	q := r.db.Model(&domain.User{})
	if s := strings.TrimSpace(search); s != "" {
		like := "%" + s + "%"
		q = q.Where("name ILIKE ? OR email ILIKE ?", like, like)
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * limit
	err := q.Order("created_at DESC").Limit(limit).Offset(offset).Find(&users).Error
	return users, total, err
}

func (r *repository) FindByID(id uuid.UUID) (*domain.User, error) {
	var u domain.User
	if err := r.db.Where("id = ?", id).First(&u).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}

func (r *repository) FindByEmail(email string) (*domain.User, error) {
	var u domain.User
	if err := r.db.Where("email = ?", email).First(&u).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}

func (r *repository) Create(u *domain.User) error { return r.db.Create(u).Error }

func (r *repository) Update(u *domain.User) error { return r.db.Save(u).Error }

func (r *repository) Delete(id uuid.UUID) error {
	return r.db.Where("id = ?", id).Delete(&domain.User{}).Error
}
