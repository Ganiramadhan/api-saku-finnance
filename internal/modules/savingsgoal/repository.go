package savingsgoal

import (
	"github.com/ganiramadhan/starter-go/internal/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Repository interface {
	List(userID uuid.UUID) ([]domain.SavingsGoal, error)
	FindByID(userID, id uuid.UUID) (*domain.SavingsGoal, error)
	Create(g *domain.SavingsGoal) error
	Update(g *domain.SavingsGoal) error
	Delete(userID, id uuid.UUID) error
	DB() *gorm.DB

	ListContributions(goalID uuid.UUID) ([]domain.SavingsGoalContribution, error)
	CreateContribution(c *domain.SavingsGoalContribution) error
}

type repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) Repository { return &repository{db: db} }

func (r *repository) DB() *gorm.DB { return r.db }

func (r *repository) List(userID uuid.UUID) ([]domain.SavingsGoal, error) {
	var rows []domain.SavingsGoal
	err := r.db.Where("user_id = ?", userID).Order("created_at DESC").Find(&rows).Error
	return rows, err
}

func (r *repository) FindByID(userID, id uuid.UUID) (*domain.SavingsGoal, error) {
	var g domain.SavingsGoal
	if err := r.db.Where("id = ? AND user_id = ?", id, userID).First(&g).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &g, nil
}

func (r *repository) Create(g *domain.SavingsGoal) error { return r.db.Create(g).Error }
func (r *repository) Update(g *domain.SavingsGoal) error { return r.db.Save(g).Error }

func (r *repository) Delete(userID, id uuid.UUID) error {
	res := r.db.Where("id = ? AND user_id = ?", id, userID).Delete(&domain.SavingsGoal{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *repository) ListContributions(goalID uuid.UUID) ([]domain.SavingsGoalContribution, error) {
	var rows []domain.SavingsGoalContribution
	err := r.db.Where("goal_id = ?", goalID).Order("created_at DESC").Find(&rows).Error
	return rows, err
}

func (r *repository) CreateContribution(c *domain.SavingsGoalContribution) error {
	return r.db.Create(c).Error
}
