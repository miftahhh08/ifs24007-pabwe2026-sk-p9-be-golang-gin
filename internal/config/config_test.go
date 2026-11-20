package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadDefaults(t *testing.T) {
	for _, k := range []string{"APP_PORT", "PORT", "DB_HOST", "OPENAI_MODEL"} {
		t.Setenv(k, "")
	}
	cfg := Load()
	assert.Equal(t, "8081", cfg.AppPort)
	assert.Equal(t, "127.0.0.1", cfg.DBHost)
	assert.Equal(t, "nama-model", cfg.OpenAIModel)
}

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("APP_PORT", "9999")
	t.Setenv("DB_HOST", "db.local")
	cfg := Load()
	assert.Equal(t, "9999", cfg.AppPort)
	assert.Equal(t, "db.local", cfg.DBHost)
}

func TestPostgresDSN(t *testing.T) {
	cfg := &Config{DBHost: "h", DBPort: "1", DBUsername: "u", DBPassword: "p", DBDatabase: "d", DBSSLMode: "disable"}
	assert.Equal(t, "host=h port=1 user=u password=p dbname=d sslmode=disable TimeZone=Asia/Jakarta", cfg.PostgresDSN())
}

func TestLoadPortFallback(t *testing.T) {
	t.Setenv("APP_PORT", "")
	t.Setenv("PORT", "7777")
	assert.Equal(t, "7777", Load().AppPort)
}
