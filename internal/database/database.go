package database

import (
	"log"
	"strings"

	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/bcrypt"
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

// ConnectWithFallback membuka PostgreSQL; bila gagal (atau DB_CONNECTION=sqlite)
// aplikasi tetap hidup dengan SQLite berkas lokal agar server tidak berhenti.
func ConnectWithFallback(cfg *config.Config) (*gorm.DB, error) {
	if !strings.EqualFold(cfg.DBConnection, "sqlite") {
		db, err := Connect(cfg)
		if err == nil {
			return db, nil
		}
		log.Printf("PostgreSQL tidak tersedia (%v), beralih ke SQLite", err)
	}
	return openSQLite(cfg.SQLitePath)
}

// openSQLite mencoba berkas yang diminta, lalu memori sebagai pilihan terakhir.
func openSQLite(path string) (*gorm.DB, error) {
	if path == "" {
		path = "app.db"
	}
	var lastErr error
	for _, dsn := range []string{path, "file::memory:"} {
		db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Warn)})
		if err != nil {
			lastErr = err
			continue
		}
		if sqlDB, e := db.DB(); e == nil {
			sqlDB.SetMaxOpenConns(1)
		}
		return db, nil
	}
	return nil, lastErr
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

// Seed memastikan akun admin dari ADMIN_USERNAME / ADMIN_PASSWORD selalu ada,
// berperan admin, dan kata sandinya sesuai konfigurasi (idempoten).
func Seed(db *gorm.DB, cfg *config.Config) error {
	var admin auth.User
	err := db.Where("username = ?", cfg.AdminUsername).Limit(1).Find(&admin).Error
	if err != nil {
		return err
	}
	if admin.ID == 0 {
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
	updates := map[string]interface{}{}
	if admin.Role != auth.RoleAdmin {
		updates["role"] = auth.RoleAdmin
	}
	if bcrypt.CompareHashAndPassword([]byte(admin.Password), []byte(cfg.AdminPassword)) != nil {
		hash, err := auth.HashPassword(cfg.AdminPassword)
		if err != nil {
			return err
		}
		updates["password"] = hash
	}
	if len(updates) == 0 {
		return nil
	}
	return db.Model(&auth.User{}).Where("id = ?", admin.ID).Updates(updates).Error
}
