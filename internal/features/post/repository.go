package post

import (
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"app/internal/features/auth"
)

// AuthorInfo adalah data penulis yang aman ditampilkan ke publik.
type AuthorInfo struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
}

// Post adalah model tabel posts.
type Post struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	UserID         uint       `gorm:"not null;index" json:"user_id"`
	User           auth.User  `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"-"`
	Title          string     `gorm:"size:255;not null" json:"title"`
	Slug           string     `gorm:"size:255;uniqueIndex;not null" json:"slug"`
	Category       string     `gorm:"size:100;not null" json:"category"`
	Excerpt        string     `gorm:"type:text" json:"excerpt"`
	Content        string     `gorm:"type:text" json:"content"`
	CoverImagePath string     `gorm:"type:text" json:"cover_image_path"`
	IsPublished    bool       `gorm:"not null" json:"is_published"`
	PublishedAt    *time.Time `json:"published_at"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`

	ReadingTime int         `gorm:"-" json:"reading_time"`
	Author      *AuthorInfo `gorm:"-" json:"author,omitempty"`
}

// Fill mengisi field turunan (reading time & author).
func (p *Post) Fill() {
	p.ReadingTime = ReadingTime(p.Content)
	if p.User.ID != 0 {
		p.Author = &AuthorInfo{ID: p.User.ID, Username: p.User.Username}
	}
}

// AfterFind otomatis menghitung reading time setiap artikel dibaca dari DB.
func (p *Post) AfterFind(_ *gorm.DB) error {
	p.Fill()
	return nil
}

// Repository mengabstraksi akses data artikel.
type Repository interface {
	Create(p *Post) error
	Save(p *Post) error
	Delete(p *Post) error
	FindByID(id uint) (*Post, error)
	FindByIDAndUser(id, userID uint) (*Post, error)
	FindPublicBySlug(slug string) (*Post, error)
	ListPublic(f PostFilter) ([]Post, error)
	ListByUser(userID uint) ([]Post, error)
	ListAll() ([]Post, error)
	SlugExists(slug string, excludeID uint) (bool, error)
	CountByUser(userID uint) (total, published int64, err error)
	AdminUsers() ([]AdminUser, error)
	UserExists(id uint) (bool, error)
}

type gormRepository struct{ db *gorm.DB }

// NewRepository membuat repository berbasis GORM.
func NewRepository(db *gorm.DB) Repository { return &gormRepository{db: db} }

func (r *gormRepository) Create(p *Post) error {
	return r.db.Omit(clause.Associations).Create(p).Error
}

func (r *gormRepository) Save(p *Post) error {
	return r.db.Omit(clause.Associations).Save(p).Error
}

func (r *gormRepository) Delete(p *Post) error { return r.db.Delete(p).Error }

func (r *gormRepository) FindByID(id uint) (*Post, error) {
	var p Post
	if err := r.db.Preload("User").First(&p, id).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *gormRepository) FindByIDAndUser(id, userID uint) (*Post, error) {
	var p Post
	if err := r.db.Preload("User").Where("id = ? AND user_id = ?", id, userID).First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *gormRepository) FindPublicBySlug(slug string) (*Post, error) {
	var p Post
	if err := r.db.Preload("User").Where("slug = ? AND is_published = ?", slug, true).First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *gormRepository) ListPublic(f PostFilter) ([]Post, error) {
	q := r.db.Preload("User").Where("is_published = ?", true)
	if f.Category != "" {
		q = q.Where("LOWER(category) = ?", strings.ToLower(f.Category))
	}
	if f.Query != "" {
		like := "%" + strings.ToLower(f.Query) + "%"
		q = q.Where("(LOWER(title) LIKE ? OR LOWER(excerpt) LIKE ? OR LOWER(content) LIKE ?)", like, like, like)
	}
	posts := make([]Post, 0)
	err := q.Order("published_at DESC, id DESC").Find(&posts).Error
	return posts, err
}

func (r *gormRepository) ListByUser(userID uint) ([]Post, error) {
	posts := make([]Post, 0)
	err := r.db.Preload("User").Where("user_id = ?", userID).Order("created_at DESC, id DESC").Find(&posts).Error
	return posts, err
}

func (r *gormRepository) ListAll() ([]Post, error) {
	posts := make([]Post, 0)
	err := r.db.Preload("User").Order("created_at DESC, id DESC").Find(&posts).Error
	return posts, err
}

func (r *gormRepository) SlugExists(slug string, excludeID uint) (bool, error) {
	var n int64
	err := r.db.Model(&Post{}).Where("slug = ? AND id <> ?", slug, excludeID).Count(&n).Error
	return n > 0, err
}

func (r *gormRepository) CountByUser(userID uint) (int64, int64, error) {
	var res struct {
		Total     int64
		Published int64
	}
	err := r.db.Model(&Post{}).
		Select("COUNT(id) AS total, COALESCE(SUM(CASE WHEN is_published THEN 1 ELSE 0 END), 0) AS published").
		Where("user_id = ?", userID).
		Scan(&res).Error
	return res.Total, res.Published, err
}

func (r *gormRepository) AdminUsers() ([]AdminUser, error) {
	var users []auth.User
	if err := r.db.Order("id").Find(&users).Error; err != nil {
		return nil, err
	}
	type counter struct {
		UserID    uint
		Total     int64
		Published int64
	}
	var counts []counter
	err := r.db.Raw(`
		SELECT user_id AS user_id, COUNT(id) AS total,
		       COALESCE(SUM(CASE WHEN is_published THEN 1 ELSE 0 END), 0) AS published
		FROM posts GROUP BY user_id`).Scan(&counts).Error
	if err != nil {
		return nil, err
	}
	byUser := make(map[uint]counter, len(counts))
	for _, c := range counts {
		byUser[c.UserID] = c
	}
	out := make([]AdminUser, 0, len(users))
	for _, u := range users {
		c := byUser[u.ID]
		out = append(out, AdminUser{
			ID: u.ID, Username: u.Username, Email: u.Email, Role: u.Role,
			TotalPosts: c.Total, TotalPublished: c.Published, CreatedAt: u.CreatedAt,
			TotalArticles: c.Total, TotalSelesai: c.Published, BelumSelesai: c.Total - c.Published,
		})
	}
	return out, nil
}

func (r *gormRepository) UserExists(id uint) (bool, error) {
	var n int64
	err := r.db.Model(&auth.User{}).Where("id = ?", id).Count(&n).Error
	return n > 0, err
}
