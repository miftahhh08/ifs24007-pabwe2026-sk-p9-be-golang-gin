package middleware

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"app/internal/response"
)

const claimsKey = "claims"

// Claims adalah payload JWT Delcom Blog.
type Claims struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// NewToken menerbitkan JWT HS256.
func NewToken(secret string, userID uint, username, role string, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:   userID,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

// CORS mengizinkan akses lintas origin (frontend dideploy terpisah).
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("Access-Control-Allow-Origin", "*")
		h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// Auth memvalidasi Bearer Token JWT.
func Auth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		parts := strings.SplitN(c.GetHeader("Authorization"), " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			response.Fail(c, http.StatusUnauthorized, "Missing or malformed Authorization header")
			c.Abort()
			return
		}
		claims := &Claims{}
		tok, err := jwt.ParseWithClaims(parts[1], claims, func(*jwt.Token) (interface{}, error) {
			return []byte(secret), nil
		}, jwt.WithValidMethods([]string{"HS256"}))
		if err != nil || !tok.Valid {
			response.Fail(c, http.StatusUnauthorized, "Invalid or expired token")
			c.Abort()
			return
		}
		c.Set(claimsKey, claims)
		c.Next()
	}
}

// GetClaims mengambil claims JWT dari context (nil jika belum terautentikasi).
func GetClaims(c *gin.Context) *Claims {
	v, ok := c.Get(claimsKey)
	if !ok {
		return nil
	}
	cl, _ := v.(*Claims)
	return cl
}

// RequireRole membatasi akses berdasarkan role (mis. "admin").
func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		cl := GetClaims(c)
		if cl == nil {
			response.Fail(c, http.StatusUnauthorized, "Unauthenticated")
			c.Abort()
			return
		}
		for _, r := range roles {
			if cl.Role == r {
				c.Next()
				return
			}
		}
		response.Fail(c, http.StatusForbidden, "Forbidden: insufficient role")
		c.Abort()
	}
}

type hitEntry struct {
	count int
	reset time.Time
}

// RateLimiter membatasi jumlah request per IP dalam satu jendela waktu.
func RateLimiter(limit int, window time.Duration) gin.HandlerFunc {
	var mu sync.Mutex
	hits := map[string]*hitEntry{}
	return func(c *gin.Context) {
		ip := c.ClientIP()
		now := time.Now()
		mu.Lock()
		e, ok := hits[ip]
		if !ok || now.After(e.reset) {
			e = &hitEntry{reset: now.Add(window)}
			hits[ip] = e
		}
		e.count++
		allowed := e.count <= limit
		mu.Unlock()
		if !allowed {
			response.Fail(c, http.StatusTooManyRequests, "Too many requests, please try again later")
			c.Abort()
			return
		}
		c.Next()
	}
}
