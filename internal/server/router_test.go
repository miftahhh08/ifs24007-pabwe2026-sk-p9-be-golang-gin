package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"app/internal/config"
	"app/internal/database"
	"app/internal/features/ai"
	"app/internal/server"
)

type fakeSearch struct{}

func (fakeSearch) Search(context.Context, string) ([]ai.Source, error) {
	return []ai.Source{{Title: "Berita", URL: "https://berita.test/1", Snippet: "fakta"}}, nil
}

type fakeLLM struct{}

func (fakeLLM) Complete(context.Context, string, string) (string, error) {
	return `{"title":"Artikel AI","category":"Teknologi","excerpt":"Ringkas","content":"Isi [1]"}`, nil
}

func newServer(t *testing.T) *gin.Engine {
	db, err := database.NewTestDB()
	require.NoError(t, err)
	cfg := &config.Config{AppEnv: "production", AppKey: "k", AdminUsername: "admin", AdminPassword: "admin"}
	require.NoError(t, database.Seed(db, cfg))
	return server.NewWithDeps(cfg, db, fakeSearch{}, fakeLLM{})
}

func call(r *gin.Engine, method, path, token string, body interface{}) (*httptest.ResponseRecorder, map[string]interface{}) {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var out map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w, out
}

func login(t *testing.T, r *gin.Engine, u, p string) string {
	w, out := call(r, "POST", "/api/auth/login", "", gin.H{"username": u, "password": p})
	require.Equal(t, 200, w.Code)
	return out["data"].(map[string]interface{})["token"].(string)
}

func TestEndToEnd(t *testing.T) {
	r := newServer(t)

	// register + login penulis
	w, _ := call(r, "POST", "/api/auth/register", "", gin.H{"username": "sari", "email": "sari@mail.com", "password": "rahasia"})
	require.Equal(t, 201, w.Code)
	tok := login(t, r, "sari", "rahasia")

	// buat & terbitkan artikel
	w, out := call(r, "POST", "/api/posts", tok, gin.H{"title": "Artikel Pertama", "content": "isi", "category": "Opini", "is_published": true})
	require.Equal(t, 201, w.Code)
	assert.Equal(t, "artikel-pertama", out["data"].(map[string]interface{})["slug"])

	// publik bisa membaca tanpa login
	w, out = call(r, "GET", "/api/public/posts/artikel-pertama", "", nil)
	assert.Equal(t, 200, w.Code)
	assert.Equal(t, "success", out["status"])

	// AI generate (search engine + OpenAI palsu)
	w, out = call(r, "POST", "/api/posts/ai-generate", tok, gin.H{"topic": "Tren teknologi", "include_web_search": true})
	require.Equal(t, 200, w.Code)
	d := out["data"].(map[string]interface{})
	assert.Equal(t, "Artikel AI", d["title"])
	assert.Len(t, d["sources"], 1)

	// admin only
	w, _ = call(r, "GET", "/api/admin/users", tok, nil)
	assert.Equal(t, 403, w.Code)
	adminTok := login(t, r, "admin", "admin")
	w, out = call(r, "GET", "/api/admin/users", adminTok, nil)
	assert.Equal(t, 200, w.Code)
	assert.Len(t, out["data"], 2)

	// auth wajib untuk rute penulis
	w, _ = call(r, "GET", "/api/posts", "", nil)
	assert.Equal(t, 401, w.Code)
}

func TestDocsAndNoRoute(t *testing.T) {
	r := newServer(t)

	w, _ := call(r, "GET", "/api.json", "", nil)
	assert.Equal(t, 200, w.Code)
	var spec map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &spec))
	assert.Equal(t, "2.0", spec["swagger"])

	req := httptest.NewRequest(http.MethodGet, "/swagger/index.html", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, 200, rec.Code)

	req = httptest.NewRequest(http.MethodGet, "/swagger/doc.json", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, 200, rec.Code)

	w, out := call(r, "GET", "/api/tidak-ada", "", nil)
	assert.Equal(t, 404, w.Code)
	assert.Equal(t, "Endpoint not found", out["message"])
}

func TestNewUsesRealClients(t *testing.T) {
	db, err := database.NewTestDB()
	require.NoError(t, err)
	r := server.New(&config.Config{AppKey: "k", SearchEngineEndpoint: "http://127.0.0.1:1", OpenAIEndpoint: "http://127.0.0.1:1"}, db)
	assert.NotNil(t, r)
}
