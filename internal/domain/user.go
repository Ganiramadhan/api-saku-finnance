package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type User struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey"`
	Name         string    `gorm:"type:varchar(255);not null"`
	Email        string    `gorm:"type:varchar(255);not null"`
	Photo        string    `gorm:"type:varchar(500)"`
	Password     string    `gorm:"type:varchar(255);not null"`
	AuthProvider string    `gorm:"type:varchar(32);not null;default:'password';index"`
	Phone        string    `gorm:"type:varchar(32)"`
	Role         string    `gorm:"type:varchar(50);not null;default:'user'"`
	Status       string    `gorm:"type:varchar(20);not null;default:'active'"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    gorm.DeletedAt `gorm:"index"`

	OTP      *UserOTP      `gorm:"foreignKey:UserID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Referral *UserReferral `gorm:"foreignKey:UserID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

func (u *User) BeforeCreate(_ *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return nil
}

type UserOTP struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID    uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:idx_user_otps_user_purpose"`
	Purpose   string    `gorm:"type:varchar(32);not null;default:'password_reset';uniqueIndex:idx_user_otps_user_purpose;index"`
	CodeHash  string    `gorm:"type:varchar(255);not null"`
	ExpiresAt time.Time `gorm:"not null;index"`
	CreatedAt time.Time
	UpdatedAt time.Time

	User *User `gorm:"foreignKey:UserID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

func (o *UserOTP) BeforeCreate(_ *gorm.DB) error {
	if o.ID == uuid.Nil {
		o.ID = uuid.New()
	}
	return nil
}

type UserReferral struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID    uuid.UUID `gorm:"type:uuid;not null;uniqueIndex"`
	Code      string    `gorm:"type:varchar(32);not null;uniqueIndex"`
	Reward    int64     `gorm:"not null;default:0"`
	CreatedAt time.Time
	UpdatedAt time.Time

	User *User `gorm:"foreignKey:UserID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

func (r *UserReferral) BeforeCreate(_ *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	return nil
}
