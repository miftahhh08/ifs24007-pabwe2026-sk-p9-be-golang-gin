package database

import (
	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"app/internal/config"
	"app/internal/features/auth"
	"app/internal/features/post"
)

// openDialector dipisah sebagai variabel agar dapat diganti saat pengujian.
var openDialector = func(cfg *config.Config) gorm.Dialector {
	return postgres.Open(cfg.PostgresDSN())
}

// Connect membuka koneksi PostgreSQL untuk runtime.
func Connect(cfg *config.Config) (*gorm.DB, error) {
	return gorm.Open(openDialector(cfg), &gorm.Config{Logger: logger.Default.LogMode(logger.Warn)})
}

// sqliteDSN dipisah sebagai variabel agar kegagalan pembukaan dapat diuji.
var sqliteDSN = "file::memory:"

// NewTestDB membuat database SQLite in-memory (untuk TDD) lengkap dengan migrasi.
func NewTestDB() (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(sqliteDSN), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return nil, err
	}
	sqlDB, _ := db.DB()      // tidak pernah gagal untuk driver SQLite
	sqlDB.SetMaxOpenConns(1) // satu koneksi = satu database in-memory yang konsisten
	return db, Migrate(db)
}

// Migrate memigrasikan tabel users dan posts.
func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&auth.User{}, &post.Post{})
}

// Seed membuat akun admin dari ADMIN_USERNAME / ADMIN_PASSWORD bila belum ada.
func Seed(db *gorm.DB, cfg *config.Config) error {
	var n int64
	if err := db.Model(&auth.User{}).Where("username = ?", cfg.AdminUsername).Count(&n).Error; err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	hash, err := auth.HashPassword(cfg.AdminPassword)
	if err != nil {
		return err
	}
	return db.Create(&auth.User{
		Username: cfg.AdminUsername,
		Email:    cfg.AdminUsername + "@delcom.org",
		Password: hash,
		Role:     auth.RoleAdmin,
	}).Error
}
