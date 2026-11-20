package auth

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"app/internal/middleware"
	"app/internal/response"
)

// Handler menangani endpoint /api/auth/*.
type Handler struct{ svc *Service }

// NewHandler membuat handler autentikasi.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Invalid request: "+err.Error())
		return
	}
	u, err := h.svc.Register(req)
	switch {
	case errors.Is(err, ErrDuplicate):
		response.Fail(c, http.StatusConflict, err.Error())
	case err != nil:
		response.Error(c, "Failed to register user")
	default:
		response.Success(c, http.StatusCreated, "Registration successful", u)
	}
}

func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Invalid request: "+err.Error())
		return
	}
	res, err := h.svc.Login(req)
	switch {
	case errors.Is(err, ErrInvalidCredentials):
		response.Fail(c, http.StatusUnauthorized, err.Error())
	case err != nil:
		response.Error(c, "Failed to login")
	default:
		response.Success(c, http.StatusOK, "Login successful", res)
	}
}

// Logout bersifat stateless: klien cukup membuang token.
func (h *Handler) Logout(c *gin.Context) {
	response.Success(c, http.StatusOK, "Logout successful", nil)
}

func (h *Handler) GetProfile(c *gin.Context) {
	u, err := h.svc.Profile(middleware.GetClaims(c).UserID)
	switch {
	case errors.Is(err, ErrNotFound):
		response.Fail(c, http.StatusNotFound, err.Error())
	case err != nil:
		response.Error(c, "Failed to load profile")
	default:
		response.Success(c, http.StatusOK, "Profile loaded", u)
	}
}

func (h *Handler) UpdateProfile(c *gin.Context) {
	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Invalid request: "+err.Error())
		return
	}
	u, err := h.svc.UpdateProfile(middleware.GetClaims(c).UserID, req)
	switch {
	case errors.Is(err, ErrNotFound):
		response.Fail(c, http.StatusNotFound, err.Error())
	case errors.Is(err, ErrDuplicate):
		response.Fail(c, http.StatusConflict, err.Error())
	case err != nil:
		response.Error(c, "Failed to update profile")
	default:
		response.Success(c, http.StatusOK, "Profile updated", u)
	}
}
