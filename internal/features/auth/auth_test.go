package auth_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"app/internal/database"
	"app/internal/features/auth"
	"app/internal/middleware"
)

const secret = "unit-secret"

var errBoom = errors.New("boom")

// failingRepo selalu mengembalikan error (untuk menguji cabang 500).
type failingRepo struct{}

func (failingRepo) Create(*auth.User) error                   { return errBoom }
func (failingRepo) FindByID(uint) (*auth.User, error)         { return nil, errBoom }
func (failingRepo) FindByLogin(string) (*auth.User, error)    { return nil, errBoom }
func (failingRepo) Exists(string, string, uint) (bool, error) { return false, errBoom }
func (failingRepo) Update(*auth.User) error                   { return errBoom }

func newEngine(repo auth.Repository) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := auth.NewHandler(auth.NewService(repo, secret))
	r := gin.New()
	r.POST("/register", h.Register)
	r.POST("/login", h.Login)
	r.POST("/logout", middleware.Auth(secret), h.Logout)
	r.GET("/profile", middleware.Auth(secret), h.GetProfile)
	r.PUT("/profile", middleware.Auth(secret), h.UpdateProfile)
	return r
}

func call(r *gin.Engine, method, path, token string, body interface{}) (*httptest.ResponseRecorder, map[string]interface{}) {
	var buf bytes.Buffer
	if s, ok := body.(string); ok {
		buf.WriteString(s)
	} else if body != nil {
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

func newDB(t *testing.T) *gorm.DB {
	db, err := database.NewTestDB()
	require.NoError(t, err)
	return db
}

func TestRegisterLoginProfileFlow(t *testing.T) {
	r := newEngine(auth.NewRepository(newDB(t)))

	w, regOut := call(r, "POST", "/register", "", gin.H{"username": "budi", "email": "Budi@Mail.com", "password": "rahasia"})
	assert.Equal(t, http.StatusCreated, w.Code)
	regData := regOut["data"].(map[string]interface{})
	assert.Len(t, strings.Split(regData["token"].(string), "."), 3)
	assert.Equal(t, "budi", regData["user"].(map[string]interface{})["username"])
	assert.Equal(t, "member", regData["user"].(map[string]interface{})["role"])

	w, regAdminOut := call(r, "POST", "/register", "", gin.H{"username": "adminuji", "email": "adminuji@mail.com", "password": "rahasia", "role": "admin"})
	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "admin", regAdminOut["data"].(map[string]interface{})["user"].(map[string]interface{})["role"])

	w, _ = call(r, "POST", "/register", "", gin.H{"username": "budi", "email": "lain@mail.com", "password": "rahasia"})
	assert.Equal(t, http.StatusBadRequest, w.Code)

	w, _ = call(r, "POST", "/register", "", "{bad json")
	assert.Equal(t, http.StatusBadRequest, w.Code)

	w, _ = call(r, "POST", "/login", "", gin.H{"username": "budi", "password": "salah"})
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	w, _ = call(r, "POST", "/login", "", gin.H{"username": "tidakada", "password": "x"})
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	w, _ = call(r, "POST", "/login", "", "{}")
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// login dengan email (case-insensitive karena email disimpan lowercase)
	w, out := call(r, "POST", "/login", "", gin.H{"username": "budi@mail.com", "password": "rahasia"})
	require.Equal(t, http.StatusOK, w.Code)
	token := out["data"].(map[string]interface{})["token"].(string)
	require.NotEmpty(t, token)

	w, out = call(r, "GET", "/profile", token, nil)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "budi", out["data"].(map[string]interface{})["username"])
	assert.NotContains(t, w.Body.String(), "password")

	w, _ = call(r, "PUT", "/profile", token, gin.H{"username": "budi2", "password": "barubaru"})
	assert.Equal(t, http.StatusOK, w.Code)
	w, _ = call(r, "PUT", "/profile", token, "{bad")
	assert.Equal(t, http.StatusBadRequest, w.Code)
	w, _ = call(r, "POST", "/login", "", gin.H{"username": "budi2", "password": "barubaru"})
	assert.Equal(t, http.StatusOK, w.Code)

	w, _ = call(r, "POST", "/logout", token, nil)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestUpdateProfileDuplicateAndNotFound(t *testing.T) {
	db := newDB(t)
	repo := auth.NewRepository(db)
	svc := auth.NewService(repo, secret)
	r := newEngine(repo)

	a, err := svc.Register(auth.RegisterRequest{Username: "alice", Email: "a@mail.com", Password: "123456"})
	require.NoError(t, err)
	_, err = svc.Register(auth.RegisterRequest{Username: "bob", Email: "b@mail.com", Password: "123456"})
	require.NoError(t, err)

	tok, _ := middleware.NewToken(secret, a.ID, a.Username, a.Role, time.Hour)
	w, _ := call(r, "PUT", "/profile", tok, gin.H{"username": "bob"})
	assert.Equal(t, http.StatusBadRequest, w.Code)

	ghost, _ := middleware.NewToken(secret, 9999, "ghost", "member", time.Hour)
	w, _ = call(r, "GET", "/profile", ghost, nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
	w, _ = call(r, "PUT", "/profile", ghost, gin.H{"username": "x123"})
	assert.Equal(t, http.StatusNotFound, w.Code)

	_, err = svc.Profile(9999)
	assert.ErrorIs(t, err, auth.ErrNotFound)
}

func TestHandlersReturn500OnRepoError(t *testing.T) {
	r := newEngine(failingRepo{})
	tok, _ := middleware.NewToken(secret, 1, "u", "member", time.Hour)

	w, _ := call(r, "POST", "/register", "", gin.H{"username": "budi", "email": "b@mail.com", "password": "rahasia"})
	assert.Equal(t, 500, w.Code)
	w, _ = call(r, "POST", "/login", "", gin.H{"username": "budi", "password": "x"})
	assert.Equal(t, 500, w.Code)
	w, _ = call(r, "GET", "/profile", tok, nil)
	assert.Equal(t, 500, w.Code)
	w, _ = call(r, "PUT", "/profile", tok, gin.H{"username": "newname"})
	assert.Equal(t, 500, w.Code)
}

func TestServiceCreateAndUpdateErrors(t *testing.T) {
	// Exists OK tetapi Create gagal
	svc := auth.NewService(createFailRepo{}, secret)
	_, err := svc.Register(auth.RegisterRequest{Username: "u123", Email: "u@mail.com", Password: "123456"})
	assert.ErrorIs(t, err, errBoom)

	_, err = svc.UpdateProfile(1, auth.UpdateProfileRequest{Username: "newname", Password: "654321"})
	assert.ErrorIs(t, err, errBoom)
}

// createFailRepo: pencarian berhasil, tetapi penulisan gagal.
type createFailRepo struct{}

func (createFailRepo) Create(*auth.User) error { return errBoom }
func (createFailRepo) FindByID(id uint) (*auth.User, error) {
	return &auth.User{ID: id, Username: "old", Email: "old@mail.com"}, nil
}
func (createFailRepo) FindByLogin(string) (*auth.User, error)    { return nil, gorm.ErrRecordNotFound }
func (createFailRepo) Exists(string, string, uint) (bool, error) { return false, nil }
func (createFailRepo) Update(*auth.User) error                   { return errBoom }

// existsFailRepo: FindByID berhasil, tetapi Exists gagal.
type existsFailRepo struct{ createFailRepo }

func (existsFailRepo) Exists(string, string, uint) (bool, error) { return false, errBoom }

func TestServiceRemainingBranches(t *testing.T) {
	long := strings.Repeat("a", 80) // bcrypt menolak > 72 byte

	svc := auth.NewService(createFailRepo{}, secret)
	_, err := svc.Register(auth.RegisterRequest{Username: "u123", Email: "u@mail.com", Password: long})
	assert.Error(t, err)
	_, err = svc.UpdateProfile(1, auth.UpdateProfileRequest{Password: long})
	assert.Error(t, err)

	_, err = auth.NewService(existsFailRepo{}, secret).UpdateProfile(1, auth.UpdateProfileRequest{Username: "newname"})
	assert.ErrorIs(t, err, errBoom)

	// kegagalan penerbitan token
	real := auth.NewService(auth.NewRepository(newDB(t)), secret)
	_, err = real.Register(auth.RegisterRequest{Username: "tokoh", Email: "t@mail.com", Password: "rahasia"})
	require.NoError(t, err)
	auth.SetSigner(real, func(string, uint, string, string, time.Duration) (string, error) { return "", errBoom })
	_, err = real.Login(auth.LoginRequest{Username: "tokoh", Password: "rahasia"})
	assert.ErrorIs(t, err, errBoom)
}
