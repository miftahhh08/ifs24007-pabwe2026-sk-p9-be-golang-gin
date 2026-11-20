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

// PublicPost adalah bentuk artikel untuk publik (tanpa user_id / data internal).
type PublicPost struct {
	ID             uint        `json:"id"`
	Title          string      `json:"title"`
	Slug           string      `json:"slug"`
	Category       string      `json:"category"`
	Excerpt        string      `json:"excerpt"`
	Content        string      `json:"content"`
	CoverImagePath string      `json:"cover_image_path"`
	IsPublished    bool        `json:"is_published"`
	PublishedAt    *time.Time  `json:"published_at"`
	CreatedAt      time.Time   `json:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at"`
	ReadingTime    int         `json:"reading_time"`
	Author         *AuthorInfo `json:"author,omitempty"`
}

// ToPublic mengubah Post menjadi PublicPost.
func ToPublic(p *Post) PublicPost {
	return PublicPost{
		ID: p.ID, Title: p.Title, Slug: p.Slug, Category: p.Category,
		Excerpt: p.Excerpt, Content: p.Content, CoverImagePath: p.CoverImagePath,
		IsPublished: p.IsPublished, PublishedAt: p.PublishedAt,
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
		ReadingTime: p.ReadingTime, Author: p.Author,
	}
}

// PostFilter adalah filter pencarian artikel publik (?q=...&category=...).
type PostFilter struct {
	Query    string
	Category string
}

// Stats adalah ringkasan 3 kartu statistik penulis.
type Stats struct {
	Total        int64 `json:"total"`
	Published    int64 `json:"published"`
	Draft        int64 `json:"draft"`
	TotalArticle int64 `json:"total_articles"`
	TotalSelesai int64 `json:"total_selesai"`
	BelumSelesai int64 `json:"belum_selesai"`
}

// AdminUser adalah ringkasan pengguna untuk panel admin.
type AdminUser struct {
	ID             uint      `json:"id"`
	Username       string    `json:"username"`
	Email          string    `json:"email"`
	Role           string    `json:"role"`
	TotalPosts     int64     `json:"total_posts"`
	TotalPublished int64     `json:"total_published"`
	TotalArticles  int64     `json:"total_articles"`
	TotalSelesai   int64     `json:"total_selesai"`
	BelumSelesai   int64     `json:"belum_selesai"`
	CreatedAt      time.Time `json:"created_at"`
}
