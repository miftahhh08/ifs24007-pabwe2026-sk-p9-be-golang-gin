package database

import (
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"app/internal/config"
	"app/internal/features/auth"
)

func TestNewTestDBAndSeed(t *testing.T) {
	db, err := NewTestDB()
	require.NoError(t, err)
	cfg := &config.Config{AdminUsername: "admin", AdminPassword: "admin"}

	require.NoError(t, Seed(db, cfg))
	require.NoError(t, Seed(db, cfg)) // idempoten

	var users []auth.User
	require.NoError(t, db.Find(&users).Error)
	require.Len(t, users, 1)
	assert.Equal(t, auth.RoleAdmin, users[0].Role)
}

func TestSeedErrorWithoutTable(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	require.NoError(t, err)
	assert.Error(t, Seed(db, &config.Config{AdminUsername: "a", AdminPassword: "b"}))
}

func TestConnect(t *testing.T) {
	orig := openDialector
	defer func() { openDialector = orig }()

	openDialector = func(*config.Config) gorm.Dialector { return sqlite.Open("file::memory:") }
	db, err := Connect(&config.Config{})
	require.NoError(t, err)
	assert.NotNil(t, db)

	openDialector = orig
	_, err = Connect(&config.Config{DBHost: "127.0.0.1", DBPort: "1", DBUsername: "x", DBPassword: "x", DBDatabase: "x", DBSSLMode: "disable"})
	assert.Error(t, err)
}

func TestNewTestDBOpenError(t *testing.T) {
	orig := sqliteDSN
	defer func() { sqliteDSN = orig }()
	sqliteDSN = "file:/nonexistent-dir-xyz/test.db?mode=ro"
	_, err := NewTestDB()
	assert.Error(t, err)
}

func TestSeedHashError(t *testing.T) {
	db, err := NewTestDB()
	require.NoError(t, err)
	// bcrypt menolak password > 72 byte
	err = Seed(db, &config.Config{AdminUsername: "x", AdminPassword: strings.Repeat("a", 80)})
	assert.Error(t, err)
}
