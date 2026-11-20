package post

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"app/internal/middleware"
	"app/internal/response"
)

// Handler menangani /api/public/*, /api/posts/* dan /api/admin/*.
type Handler struct{ svc *Service }

// NewHandler membuat handler artikel.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func parseID(c *gin.Context) (uint, bool) {
	n, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || n == 0 {
		response.Fail(c, http.StatusBadRequest, "Invalid ID")
		return 0, false
	}
	return uint(n), true
}

func uid(c *gin.Context) uint { return middleware.GetClaims(c).UserID }

// fail memetakan error service ke status HTTP.
func fail(c *gin.Context, err error, msg string) {
	switch {
	case errors.Is(err, ErrNotFound), errors.Is(err, ErrUserNotFound):
		response.Fail(c, http.StatusNotFound, err.Error())
	case errors.Is(err, ErrForbidden):
		response.Fail(c, http.StatusForbidden, err.Error())
	case errors.Is(err, ErrInvalidCategory):
		response.Fail(c, http.StatusUnprocessableEntity, "Invalid category, allowed: Teknologi, Pendidikan, Gaya Hidup, Bisnis, Opini, Umum")
	default:
		response.Error(c, msg)
	}
}

// ---- Publik ----

func (h *Handler) Categories(c *gin.Context) {
	response.Success(c, http.StatusOK, "Categories loaded", h.svc.ListCategories())
}

func (h *Handler) PublicList(c *gin.Context) {
	posts, err := h.svc.PublicList(PostFilter{Query: c.Query("q"), Category: c.Query("category")})
	if err != nil {
		fail(c, err, "Failed to load posts")
		return
	}
	response.Success(c, http.StatusOK, "Posts loaded", posts)
}

func (h *Handler) PublicDetail(c *gin.Context) {
	p, err := h.svc.PublicDetail(c.Param("slug"))
	if err != nil {
		fail(c, err, "Failed to load post")
		return
	}
	response.Success(c, http.StatusOK, "Post loaded", p)
}

// ---- Penulis ----

func (h *Handler) MyList(c *gin.Context) {
	posts, err := h.svc.ListMine(uid(c))
	if err != nil {
		fail(c, err, "Failed to load posts")
		return
	}
	response.Success(c, http.StatusOK, "Posts loaded", posts)
}

func (h *Handler) Stats(c *gin.Context) {
	st, err := h.svc.Stats(uid(c))
	if err != nil {
		fail(c, err, "Failed to load stats")
		return
	}
	response.Success(c, http.StatusOK, "Stats loaded", st)
}

func (h *Handler) Get(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	p, err := h.svc.Get(uid(c), id)
	if err != nil {
		fail(c, err, "Failed to load post")
		return
	}
	response.Success(c, http.StatusOK, "Post loaded", p)
}

func (h *Handler) Create(c *gin.Context) {
	var req PostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Invalid request: "+err.Error())
		return
	}
	p, err := h.svc.Create(uid(c), req)
	if err != nil {
		fail(c, err, "Failed to create post")
		return
	}
	response.Success(c, http.StatusCreated, "Post created", p)
}

func (h *Handler) Update(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	// Cek keberadaan (404) dan kepemilikan (403) sebelum validasi body (400).
	if _, err := h.svc.Get(uid(c), id); err != nil {
		fail(c, err, "Failed to update post")
		return
	}
	var req PostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Invalid request: "+err.Error())
		return
	}
	p, err := h.svc.Update(uid(c), id, req)
	if err != nil {
		fail(c, err, "Failed to update post")
		return
	}
	response.Success(c, http.StatusOK, "Post updated", p)
}

func (h *Handler) TogglePublish(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	p, err := h.svc.TogglePublish(uid(c), id)
	if err != nil {
		fail(c, err, "Failed to toggle publish status")
		return
	}
	response.Success(c, http.StatusOK, "Publish status updated", p)
}

func (h *Handler) Delete(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.svc.Delete(uid(c), id); err != nil {
		fail(c, err, "Failed to delete post")
		return
	}
	response.Success(c, http.StatusOK, "Post deleted", nil)
}

// ---- Admin ----

func (h *Handler) AdminUsers(c *gin.Context) {
	users, err := h.svc.AdminUsers()
	if err != nil {
		fail(c, err, "Failed to load users")
		return
	}
	response.Success(c, http.StatusOK, "Users loaded", users)
}

func (h *Handler) AdminPosts(c *gin.Context) {
	posts, err := h.svc.AdminPosts()
	if err != nil {
		fail(c, err, "Failed to load posts")
		return
	}
	response.Success(c, http.StatusOK, "Posts loaded", posts)
}

func (h *Handler) AdminUserPosts(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	posts, err := h.svc.AdminUserPosts(id)
	if err != nil {
		fail(c, err, "Failed to load posts")
		return
	}
	response.Success(c, http.StatusOK, "Posts loaded", posts)
}
