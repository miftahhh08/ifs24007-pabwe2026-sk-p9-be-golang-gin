package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const secret = "test-secret"

func do(r *gin.Engine, method, path, auth string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func newEngine() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORS())
	r.GET("/open", func(c *gin.Context) { c.String(200, "ok") })
	r.GET("/me", Auth(secret), func(c *gin.Context) { c.String(200, GetClaims(c).Username) })
	r.GET("/admin", Auth(secret), RequireRole("admin"), func(c *gin.Context) { c.String(200, "admin") })
	r.GET("/norole", RequireRole("admin"), func(c *gin.Context) { c.String(200, "x") })
	return r
}

func TestCORS(t *testing.T) {
	r := newEngine()
	w := do(r, http.MethodOptions, "/open", "")
	assert.Equal(t, 204, w.Code)
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
	w = do(r, http.MethodGet, "/open", "")
	assert.Equal(t, 200, w.Code)
}

func TestAuth(t *testing.T) {
	r := newEngine()
	assert.Equal(t, 401, do(r, "GET", "/me", "").Code)
	assert.Equal(t, 401, do(r, "GET", "/me", "Token abc").Code)
	assert.Equal(t, 401, do(r, "GET", "/me", "Bearer garbage").Code)

	bad, _ := NewToken("other", 1, "u", "member", time.Hour)
	assert.Equal(t, 401, do(r, "GET", "/me", "Bearer "+bad).Code)

	expired, _ := NewToken(secret, 1, "u", "member", -time.Hour)
	assert.Equal(t, 401, do(r, "GET", "/me", "Bearer "+expired).Code)

	none := jwt.NewWithClaims(jwt.SigningMethodNone, Claims{})
	ns, err := none.SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)
	assert.Equal(t, 401, do(r, "GET", "/me", "Bearer "+ns).Code)

	good, err := NewToken(secret, 7, "budi", "member", time.Hour)
	require.NoError(t, err)
	w := do(r, "GET", "/me", "Bearer "+good)
	assert.Equal(t, 200, w.Code)
	assert.Equal(t, "budi", w.Body.String())
}

func TestRequireRole(t *testing.T) {
	r := newEngine()
	member, _ := NewToken(secret, 1, "m", "member", time.Hour)
	admin, _ := NewToken(secret, 2, "a", "admin", time.Hour)
	assert.Equal(t, 403, do(r, "GET", "/admin", "Bearer "+member).Code)
	assert.Equal(t, 200, do(r, "GET", "/admin", "Bearer "+admin).Code)
	assert.Equal(t, 401, do(r, "GET", "/norole", "").Code)
}

func TestGetClaimsWrongType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	assert.Nil(t, GetClaims(c))
	c.Set(claimsKey, "bukan-claims")
	assert.Nil(t, GetClaims(c))
}

func TestRateLimiter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/", RateLimiter(2, 50*time.Millisecond), func(c *gin.Context) { c.String(200, "ok") })
	assert.Equal(t, 200, do(r, "GET", "/", "").Code)
	assert.Equal(t, 200, do(r, "GET", "/", "").Code)
	assert.Equal(t, 429, do(r, "GET", "/", "").Code)
	time.Sleep(80 * time.Millisecond)
	assert.Equal(t, 200, do(r, "GET", "/", "").Code)
}
