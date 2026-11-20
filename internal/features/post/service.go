package post

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"gorm.io/gorm"
)

var (
	ErrNotFound        = errors.New("post not found")
	ErrInvalidCategory = errors.New("invalid category")
	ErrUserNotFound    = errors.New("user not found")
	ErrForbidden       = errors.New("forbidden: you do not own this post")
)

// Categories adalah daftar kategori artikel yang valid.
var Categories = []string{"Teknologi", "Pendidikan", "Gaya Hidup", "Bisnis", "Opini", "Umum"}

// DefaultCategory dipakai bila kategori tidak diisi.
const DefaultCategory = "Umum"

const wordsPerMinute = 200

var (
	nonAlnum   = regexp.MustCompile(`[^a-z0-9]+`)
	mdNoise    = regexp.MustCompile("[#*_>`\\[\\]()~|-]+")
	whitespace = regexp.MustCompile(`\s+`)
)

// Slugify mengubah judul menjadi slug URL.
func Slugify(s string) string {
	s = nonAlnum.ReplaceAllString(strings.ToLower(strings.TrimSpace(s)), "-")
	s = strings.Trim(s, "-")
	if s == "" {
		return "artikel"
	}
	return s
}

// ReadingTime menghitung estimasi menit baca (kata / 200 wpm, minimal 1 menit).
func ReadingTime(content string) int {
	words := len(strings.Fields(content))
	if words == 0 {
		return 1
	}
	return (words + wordsPerMinute - 1) / wordsPerMinute
}

// NormalizeCategory memvalidasi kategori (case-insensitive); kosong -> Umum.
func NormalizeCategory(c string) (string, error) {
	c = strings.TrimSpace(c)
	if c == "" {
		return DefaultCategory, nil
	}
	for _, v := range Categories {
		if strings.EqualFold(v, c) {
			return v, nil
		}
	}
	return "", ErrInvalidCategory
}

func autoExcerpt(content string) string {
	s := whitespace.ReplaceAllString(mdNoise.ReplaceAllString(content, " "), " ")
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) > 160 {
		return strings.TrimSpace(string(r[:160])) + "..."
	}
	return s
}

func setPublished(p *Post, v bool) {
	if v {
		if !p.IsPublished || p.PublishedAt == nil {
			now := time.Now()
			p.PublishedAt = &now
		}
	} else {
		p.PublishedAt = nil
	}
	p.IsPublished = v
}

// Service berisi logika bisnis artikel.
type Service struct{ repo Repository }

// NewService membuat service artikel.
func NewService(repo Repository) *Service { return &Service{repo: repo} }

func mapNotFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	return err
}

func (s *Service) uniqueSlug(title string, excludeID uint) (string, error) {
	base := Slugify(title)
	slug := base
	for i := 2; ; i++ {
		exists, err := s.repo.SlugExists(slug, excludeID)
		if err != nil {
			return "", err
		}
		if !exists {
			return slug, nil
		}
		slug = fmt.Sprintf("%s-%d", base, i)
	}
}

// ListCategories mengembalikan seluruh kategori.
func (s *Service) ListCategories() []string { return Categories }

// PublicList mengambil artikel terbit dengan filter kata kunci & kategori.
func (s *Service) PublicList(f PostFilter) ([]PublicPost, error) {
	f.Query = strings.TrimSpace(f.Query)
	f.Category = strings.TrimSpace(f.Category)
	if strings.EqualFold(f.Category, "semua") || strings.EqualFold(f.Category, "all") {
		f.Category = ""
	}
	posts, err := s.repo.ListPublic(f)
	if err != nil {
		return nil, err
	}
	out := make([]PublicPost, 0, len(posts))
	for i := range posts {
		out = append(out, ToPublic(&posts[i]))
	}
	return out, nil
}

// PublicDetail mengambil artikel terbit berdasarkan slug.
func (s *Service) PublicDetail(slug string) (*PublicPost, error) {
	p, err := s.repo.FindPublicBySlug(slug)
	if err != nil {
		return nil, mapNotFound(err)
	}
	pub := ToPublic(p)
	return &pub, nil
}

// ListMine mengambil seluruh artikel milik penulis (terbit & draf).
func (s *Service) ListMine(userID uint) ([]Post, error) { return s.repo.ListByUser(userID) }

// Get mengambil satu artikel milik penulis.
// Artikel tidak ada -> ErrNotFound (404); milik orang lain -> ErrForbidden (403).
func (s *Service) Get(userID, id uint) (*Post, error) {
	p, err := s.repo.FindByID(id)
	if err != nil {
		return nil, mapNotFound(err)
	}
	if p.UserID != userID {
		return nil, ErrForbidden
	}
	return p, nil
}

// Create membuat artikel baru dengan slug unik.
func (s *Service) Create(userID uint, req PostRequest) (*Post, error) {
	cat, err := NormalizeCategory(req.Category)
	if err != nil {
		return nil, err
	}
	slug, err := s.uniqueSlug(req.Title, 0)
	if err != nil {
		return nil, err
	}
	excerpt := strings.TrimSpace(req.Excerpt)
	if excerpt == "" {
		excerpt = autoExcerpt(req.Content)
	}
	p := &Post{
		UserID:         userID,
		Title:          strings.TrimSpace(req.Title),
		Slug:           slug,
		Category:       cat,
		Excerpt:        excerpt,
		Content:        req.Content,
		CoverImagePath: req.CoverImagePath,
	}
	if req.IsPublished != nil {
		setPublished(p, *req.IsPublished)
	}
	if err := s.repo.Create(p); err != nil {
		return nil, err
	}
	return s.repo.FindByID(p.ID)
}

// Update memperbarui artikel milik penulis; slug ikut berubah bila judul berubah.
func (s *Service) Update(userID, id uint, req PostRequest) (*Post, error) {
	p, err := s.Get(userID, id)
	if err != nil {
		return nil, err
	}
	cat, err := NormalizeCategory(req.Category)
	if err != nil {
		return nil, err
	}
	title := strings.TrimSpace(req.Title)
	if title != p.Title {
		slug, err := s.uniqueSlug(title, p.ID)
		if err != nil {
			return nil, err
		}
		p.Slug = slug
	}
	p.Title, p.Category, p.Content, p.CoverImagePath = title, cat, req.Content, req.CoverImagePath
	p.Excerpt = strings.TrimSpace(req.Excerpt)
	if p.Excerpt == "" {
		p.Excerpt = autoExcerpt(req.Content)
	}
	if req.IsPublished != nil {
		setPublished(p, *req.IsPublished)
	}
	if err := s.repo.Save(p); err != nil {
		return nil, err
	}
	return s.repo.FindByID(p.ID)
}

// TogglePublish membalik status Draf <-> Terbit beserta published_at.
func (s *Service) TogglePublish(userID, id uint) (*Post, error) {
	p, err := s.Get(userID, id)
	if err != nil {
		return nil, err
	}
	setPublished(p, !p.IsPublished)
	if err := s.repo.Save(p); err != nil {
		return nil, err
	}
	return s.repo.FindByID(p.ID)
}

// Delete menghapus artikel milik penulis.
func (s *Service) Delete(userID, id uint) error {
	p, err := s.Get(userID, id)
	if err != nil {
		return err
	}
	return s.repo.Delete(p)
}

// Stats menghitung 3 kartu statistik: Total, Terbit, Draf.
func (s *Service) Stats(userID uint) (*Stats, error) {
	total, published, err := s.repo.CountByUser(userID)
	if err != nil {
		return nil, err
	}
	return &Stats{
		Total: total, Published: published, Draft: total - published,
		TotalArticle: total, TotalSelesai: published, BelumSelesai: total - published,
	}, nil
}

// AdminUsers memberi ringkasan seluruh pengguna beserta jumlah artikel.
func (s *Service) AdminUsers() ([]AdminUser, error) { return s.repo.AdminUsers() }

// AdminPosts memberi arsip seluruh artikel dari semua penulis.
func (s *Service) AdminPosts() ([]Post, error) { return s.repo.ListAll() }

// AdminUserPosts memberi seluruh artikel milik satu pengguna.
// Pengguna yang tidak ada menghasilkan daftar kosong (bukan error).
func (s *Service) AdminUserPosts(userID uint) ([]Post, error) {
	return s.repo.ListByUser(userID)
}
