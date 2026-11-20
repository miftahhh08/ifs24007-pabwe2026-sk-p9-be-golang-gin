package auth

import (
	"time"

	"gorm.io/gorm"
)

// User adalah model tabel users.
type User struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Username  string    `gorm:"size:100;uniqueIndex;not null" json:"username"`
	Email     string    `gorm:"size:255;uniqueIndex;not null" json:"email"`
	Password  string    `gorm:"not null" json:"-"`
	Role      string    `gorm:"size:20;not null;default:member" json:"role"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Repository mengabstraksi akses data pengguna.
type Repository interface {
	Create(u *User) error
	FindByID(id uint) (*User, error)
	FindByLogin(login string) (*User, error)
	Exists(username, email string, excludeID uint) (bool, error)
	Update(u *User) error
}

type gormRepository struct{ db *gorm.DB }

// NewRepository membuat repository berbasis GORM.
func NewRepository(db *gorm.DB) Repository { return &gormRepository{db: db} }

func (r *gormRepository) Create(u *User) error { return r.db.Create(u).Error }

func (r *gormRepository) FindByID(id uint) (*User, error) {
	var u User
	if err := r.db.First(&u, id).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *gormRepository) FindByLogin(login string) (*User, error) {
	var u User
	if err := r.db.Where("username = ? OR email = ?", login, login).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *gormRepository) Exists(username, email string, excludeID uint) (bool, error) {
	var n int64
	err := r.db.Model(&User{}).
		Where("(username = ? OR email = ?) AND id <> ?", username, email, excludeID).
		Count(&n).Error
	return n > 0, err
}

func (r *gormRepository) Update(u *User) error { return r.db.Save(u).Error }
