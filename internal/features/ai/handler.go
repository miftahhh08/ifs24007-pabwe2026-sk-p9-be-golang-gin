package ai

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"app/internal/features/post"
	"app/internal/response"
)

// Handler menangani POST /api/posts/ai-generate.
type Handler struct{ svc *Service }

// NewHandler membuat handler AI.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Generate menghasilkan draf artikel (tidak menyimpan ke database).
func (h *Handler) Generate(c *gin.Context) {
	var req GenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Invalid request: "+err.Error())
		return
	}
	res, err := h.svc.Generate(c.Request.Context(), req)
	switch {
	case errors.Is(err, post.ErrInvalidCategory):
		response.Fail(c, http.StatusUnprocessableEntity, "Invalid category")
	case err != nil:
		response.Fail(c, http.StatusBadGateway, "AI service failed: "+err.Error())
	default:
		response.Success(c, http.StatusOK, "Article generated", res)
	}
}
