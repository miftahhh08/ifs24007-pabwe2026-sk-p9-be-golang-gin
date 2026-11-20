package post

import "time"

// PostRequest adalah payload membuat/memperbarui artikel.
type PostRequest struct {
	Title          string `json:"title" binding:"required,max=255"`
	Category       string `json:"category"`
	Excerpt        string `json:"excerpt"`
	Content        string `json:"content" binding:"required"`
	CoverImagePath string `json:"cover_image_path"`
	IsPublished    *bool  `json:"is_published"`
}

// PostFilter adalah filter pencarian artikel publik (?q=...&category=...).
type PostFilter struct {
	Query    string
	Category string
}

// Stats adalah ringkasan 3 kartu statistik penulis.
type Stats struct {
	Total     int64 `json:"total"`
	Published int64 `json:"published"`
	Draft     int64 `json:"draft"`
}

// AdminUser adalah ringkasan pengguna untuk panel admin.
type AdminUser struct {
	ID             uint      `json:"id"`
	Username       string    `json:"username"`
	Email          string    `json:"email"`
	Role           string    `json:"role"`
	TotalPosts     int64     `json:"total_posts"`
	TotalPublished int64     `json:"total_published"`
	CreatedAt      time.Time `json:"created_at"`
}
