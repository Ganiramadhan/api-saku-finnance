package user

import (
	"strings"
	"time"

	"github.com/ganiramadhan/starter-go/internal/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Repository interface {
	FindAll(page, limit int, search string) ([]domain.User, int64, error)
	FindByID(id uuid.UUID) (*domain.User, error)
	FindByEmail(email string) (*domain.User, error)
	FindByReferralCode(code string) (*domain.User, error)
	Create(u *domain.User) error
	Update(u *domain.User) error
	UpsertResetOTP(userID uuid.UUID, codeHash string, expiresAt time.Time) error
	UpsertOTP(userID uuid.UUID, purpose, codeHash string, expiresAt time.Time) error
	FindOTP(userID uuid.UUID, purpose string) (*domain.UserOTP, error)
	ClearResetOTP(userID uuid.UUID) error
	ClearOTP(userID uuid.UUID, purpose string) error
	ListPasswordHistory(userID uuid.UUID, limit int) ([]domain.UserPasswordHistory, error)
	AddPasswordHistory(userID uuid.UUID, passwordHash string) error
	EnsureReferralCode(userID uuid.UUID, code string) (*domain.UserReferral, error)
	AddReferralReward(id uuid.UUID, amount int64) error
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
	q := r.db.Model(&domain.User{}).Preload("Referral")
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
	if err := r.db.Preload("OTP").Preload("Referral").Where("id = ?", id).First(&u).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}

func (r *repository) FindByEmail(email string) (*domain.User, error) {
	var u domain.User
	if err := r.db.Preload("OTP").Preload("Referral").Where("email = ?", email).First(&u).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}

func (r *repository) FindByReferralCode(code string) (*domain.User, error) {
	var ref domain.UserReferral
	if err := r.db.Preload("User").Where("code = ?", strings.ToUpper(strings.TrimSpace(code))).First(&ref).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	if ref.User == nil {
		return nil, domain.ErrNotFound
	}
	ref.User.Referral = &ref
	return ref.User, nil
}

func (r *repository) Create(u *domain.User) error { return r.db.Create(u).Error }

func (r *repository) Update(u *domain.User) error { return r.db.Save(u).Error }

func (r *repository) UpsertResetOTP(userID uuid.UUID, codeHash string, expiresAt time.Time) error {
	return r.UpsertOTP(userID, "password_reset", codeHash, expiresAt)
}

func (r *repository) UpsertOTP(userID uuid.UUID, purpose, codeHash string, expiresAt time.Time) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ? AND purpose = ?", userID, purpose).Delete(&domain.UserOTP{}).Error; err != nil {
			return err
		}
		return tx.Create(&domain.UserOTP{
			UserID:    userID,
			Purpose:   purpose,
			CodeHash:  codeHash,
			ExpiresAt: expiresAt,
		}).Error
	})
}

func (r *repository) FindOTP(userID uuid.UUID, purpose string) (*domain.UserOTP, error) {
	var otp domain.UserOTP
	if err := r.db.Where("user_id = ? AND purpose = ?", userID, purpose).First(&otp).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &otp, nil
}

func (r *repository) ClearResetOTP(userID uuid.UUID) error {
	return r.ClearOTP(userID, "password_reset")
}

func (r *repository) ClearOTP(userID uuid.UUID, purpose string) error {
	return r.db.Where("user_id = ? AND purpose = ?", userID, purpose).Delete(&domain.UserOTP{}).Error
}

func (r *repository) ListPasswordHistory(userID uuid.UUID, limit int) ([]domain.UserPasswordHistory, error) {
	if limit <= 0 {
		limit = 5
	}
	var rows []domain.UserPasswordHistory
	err := r.db.Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(limit).
		Find(&rows).Error
	return rows, err
}

func (r *repository) AddPasswordHistory(userID uuid.UUID, passwordHash string) error {
	passwordHash = strings.TrimSpace(passwordHash)
	if passwordHash == "" {
		return nil
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&domain.UserPasswordHistory{
			UserID:       userID,
			PasswordHash: passwordHash,
		}).Error; err != nil {
			return err
		}

		var oldRows []domain.UserPasswordHistory
		if err := tx.Where("user_id = ?", userID).
			Order("created_at DESC").
			Offset(5).
			Find(&oldRows).Error; err != nil {
			return err
		}
		if len(oldRows) == 0 {
			return nil
		}
		ids := make([]uuid.UUID, 0, len(oldRows))
		for _, row := range oldRows {
			ids = append(ids, row.ID)
		}
		return tx.Delete(&domain.UserPasswordHistory{}, "id IN ?", ids).Error
	})
}

func (r *repository) EnsureReferralCode(userID uuid.UUID, code string) (*domain.UserReferral, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	var ref domain.UserReferral
	err := r.db.Where("user_id = ?", userID).First(&ref).Error
	if err == nil {
		if ref.Code == "" && code != "" {
			ref.Code = code
			if err := r.db.Save(&ref).Error; err != nil {
				return nil, err
			}
		}
		return &ref, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}
	ref = domain.UserReferral{UserID: userID, Code: code}
	if err := r.db.Create(&ref).Error; err != nil {
		return nil, err
	}
	return &ref, nil
}

func (r *repository) AddReferralReward(id uuid.UUID, amount int64) error {
	return r.db.Model(&domain.UserReferral{}).
		Where("user_id = ?", id).
		UpdateColumn("reward", gorm.Expr("reward + ?", amount)).
		Error
}

func (r *repository) Delete(id uuid.UUID) error {
	res := r.db.Where("id = ?", id).Delete(&domain.User{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}
