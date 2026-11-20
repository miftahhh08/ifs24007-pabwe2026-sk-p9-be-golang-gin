package post_test

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
	"app/internal/features/post"
	"app/internal/middleware"
)

const secret = "unit-secret"

func TestSlugify(t *testing.T) {
	assert.Equal(t, "belajar-go-gin-2026", post.Slugify("  Belajar Go & Gin 2026!! "))
	assert.Equal(t, "artikel", post.Slugify("!!!"))
}

func TestReadingTime(t *testing.T) {
	assert.Equal(t, 1, post.ReadingTime(""))
	assert.Equal(t, 1, post.ReadingTime(strings.Repeat("kata ", 200)))
	assert.Equal(t, 2, post.ReadingTime(strings.Repeat("kata ", 201)))
	assert.Equal(t, 5, post.ReadingTime(strings.Repeat("kata ", 1000)))
}

func TestNormalizeCategory(t *testing.T) {
	c, err := post.NormalizeCategory("")
	assert.NoError(t, err)
	assert.Equal(t, "Umum", c)
	c, err = post.NormalizeCategory("  gaya hidup ")
	assert.NoError(t, err)
	assert.Equal(t, "Gaya Hidup", c)
	_, err = post.NormalizeCategory("Olahraga")
	assert.ErrorIs(t, err, post.ErrInvalidCategory)
}

type env struct {
	r      *gin.Engine
	db     *gorm.DB
	author string
	other  string
	admin  string
	authID uint
}

func setup(t *testing.T) *env {
	gin.SetMode(gin.TestMode)
	db, err := database.NewTestDB()
	require.NoError(t, err)
	mk := func(name, role string) (uint, string) {
		u := &auth.User{Username: name, Email: name + "@mail.com", Password: "x", Role: role}
		require.NoError(t, db.Create(u).Error)
		tok, _ := middleware.NewToken(secret, u.ID, u.Username, u.Role, time.Hour)
		return u.ID, tok
	}
	aid, at := mk("penulis", "member")
	_, ot := mk("lain", "member")
	_, adm := mk("admin", "admin")

	h := post.NewHandler(post.NewService(post.NewRepository(db)))
	r := gin.New()
	r.GET("/public/categories", h.Categories)
	r.GET("/public/posts", h.PublicList)
	r.GET("/public/posts/:slug", h.PublicDetail)
	p := r.Group("/posts", middleware.Auth(secret))
	p.GET("", h.MyList)
	p.POST("", h.Create)
	p.GET("/stats", h.Stats)
	p.GET("/:id", h.Get)
	p.PUT("/:id", h.Update)
	p.PATCH("/:id/toggle-publish", h.TogglePublish)
	p.DELETE("/:id", h.Delete)
	a := r.Group("/admin", middleware.Auth(secret), middleware.RequireRole("admin"))
	a.GET("/users", h.AdminUsers)
	a.GET("/posts", h.AdminPosts)
	a.GET("/users/:id/posts", h.AdminUserPosts)
	return &env{r: r, db: db, author: at, other: ot, admin: adm, authID: aid}
}

func (e *env) call(method, path, token string, body interface{}) (*httptest.ResponseRecorder, map[string]interface{}) {
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
	e.r.ServeHTTP(w, req)
	var out map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w, out
}

func data(m map[string]interface{}) map[string]interface{} { return m["data"].(map[string]interface{}) }

func TestAuthorCRUDFlow(t *testing.T) {
	e := setup(t)

	// create: slug unik untuk judul kembar, draf secara default
	w, out := e.call("POST", "/posts", e.author, gin.H{"title": "Halo Dunia", "content": "# Isi\n\nTeks **tebal**.", "category": "teknologi"})
	require.Equal(t, 201, w.Code)
	d := data(out)
	assert.Equal(t, "halo-dunia", d["slug"])
	assert.Equal(t, "Teknologi", d["category"])
	assert.Equal(t, false, d["is_published"])
	assert.EqualValues(t, 1, d["reading_time"])
	assert.NotEmpty(t, d["excerpt"])
	assert.Equal(t, "penulis", d["author"].(map[string]interface{})["username"])
	id := int(d["id"].(float64))

	w, out = e.call("POST", "/posts", e.author, gin.H{"title": "Halo Dunia", "content": "x", "is_published": true})
	require.Equal(t, 201, w.Code)
	assert.Equal(t, "halo-dunia-2", data(out)["slug"])
	assert.NotNil(t, data(out)["published_at"])

	// validasi
	w, _ = e.call("POST", "/posts", e.author, gin.H{"title": "", "content": ""})
	assert.Equal(t, 400, w.Code)
	w, _ = e.call("POST", "/posts", e.author, gin.H{"title": "A", "content": "b", "category": "Olahraga"})
	assert.Equal(t, 422, w.Code)
	w, _ = e.call("POST", "/posts", "", gin.H{})
	assert.Equal(t, 401, w.Code)

	// stats: total 2, terbit 1, draf 1
	_, out = e.call("GET", "/posts/stats", e.author, nil)
	assert.EqualValues(t, 2, data(out)["total"])
	assert.EqualValues(t, 1, data(out)["published"])
	assert.EqualValues(t, 1, data(out)["draft"])
	assert.EqualValues(t, 2, data(out)["total_articles"])
	assert.EqualValues(t, 1, data(out)["total_selesai"])
	assert.EqualValues(t, 1, data(out)["belum_selesai"])

	// list & get
	_, out = e.call("GET", "/posts", e.author, nil)
	assert.Len(t, out["data"], 2)
	w, _ = e.call("GET", "/posts/"+itoa(id), e.author, nil)
	assert.Equal(t, 200, w.Code)
	w, _ = e.call("GET", "/posts/"+itoa(id), e.other, nil) // bukan pemilik
	assert.Equal(t, 403, w.Code)
	w, _ = e.call("GET", "/posts/abc", e.author, nil)
	assert.Equal(t, 400, w.Code)

	// update: judul berubah -> slug berubah; excerpt kosong -> otomatis
	w, out = e.call("PUT", "/posts/"+itoa(id), e.author, gin.H{"title": "Judul Baru", "content": "Konten baru", "category": "Bisnis", "is_published": true})
	require.Equal(t, 200, w.Code)
	assert.Equal(t, "judul-baru", data(out)["slug"])
	assert.Equal(t, "Bisnis", data(out)["category"])
	assert.Equal(t, true, data(out)["is_published"])
	// update tanpa mengubah judul -> slug tetap
	_, out = e.call("PUT", "/posts/"+itoa(id), e.author, gin.H{"title": "Judul Baru", "content": "Konten baru", "excerpt": "Ringkas"})
	assert.Equal(t, "judul-baru", data(out)["slug"])
	assert.Equal(t, "Ringkas", data(out)["excerpt"])
	w, _ = e.call("PUT", "/posts/"+itoa(id), e.author, "{bad")
	assert.Equal(t, 400, w.Code)
	w, _ = e.call("PUT", "/posts/"+itoa(id), e.author, gin.H{"title": "x", "content": "y", "category": "Zzz"})
	assert.Equal(t, 422, w.Code)
	w, _ = e.call("PUT", "/posts/"+itoa(id), e.other, gin.H{"title": "x", "content": "y"})
	assert.Equal(t, 403, w.Code)
	w, _ = e.call("PUT", "/posts/99999999", e.author, gin.H{"title": "x"})
	assert.Equal(t, 404, w.Code)
	w, _ = e.call("PUT", "/posts/0", e.author, gin.H{"title": "x", "content": "y"})
	assert.Equal(t, 400, w.Code)

	// toggle: terbit -> draf -> terbit
	_, out = e.call("PATCH", "/posts/"+itoa(id)+"/toggle-publish", e.author, nil)
	assert.Equal(t, false, data(out)["is_published"])
	assert.Nil(t, data(out)["published_at"])
	_, out = e.call("PATCH", "/posts/"+itoa(id)+"/toggle-publish", e.author, nil)
	assert.Equal(t, true, data(out)["is_published"])
	assert.NotNil(t, data(out)["published_at"])
	w, _ = e.call("PATCH", "/posts/999/toggle-publish", e.author, nil)
	assert.Equal(t, 404, w.Code)
	w, _ = e.call("PATCH", "/posts/x/toggle-publish", e.author, nil)
	assert.Equal(t, 400, w.Code)

	// delete
	w, _ = e.call("DELETE", "/posts/"+itoa(id), e.other, nil)
	assert.Equal(t, 403, w.Code)
	w, _ = e.call("DELETE", "/posts/"+itoa(id), e.author, nil)
	assert.Equal(t, 200, w.Code)
	w, _ = e.call("DELETE", "/posts/x", e.author, nil)
	assert.Equal(t, 400, w.Code)
	w, _ = e.call("GET", "/posts/"+itoa(id), e.author, nil)
	assert.Equal(t, 404, w.Code)
}

func TestPublicEndpoints(t *testing.T) {
	e := setup(t)
	e.call("POST", "/posts", e.author, gin.H{"title": "Go untuk Pemula", "content": "belajar golang", "category": "Teknologi", "is_published": true})
	e.call("POST", "/posts", e.author, gin.H{"title": "Resep Nasi", "content": "memasak nasi", "category": "Gaya Hidup", "is_published": true})
	e.call("POST", "/posts", e.author, gin.H{"title": "Draf Rahasia", "content": "belum terbit"})

	_, out := e.call("GET", "/public/categories", "", nil)
	assert.Len(t, out["data"], 6)

	_, out = e.call("GET", "/public/posts", "", nil)
	assert.Len(t, out["data"], 2) // draf tidak tampil
	_, out = e.call("GET", "/public/posts?q=GOLANG", "", nil)
	assert.Len(t, out["data"], 1)
	_, out = e.call("GET", "/public/posts?category=Gaya%20Hidup", "", nil)
	assert.Len(t, out["data"], 1)
	_, out = e.call("GET", "/public/posts?category=Semua", "", nil)
	assert.Len(t, out["data"], 2)
	_, out = e.call("GET", "/public/posts?q=tidak-ada", "", nil)
	assert.Len(t, out["data"], 0)

	w, out := e.call("GET", "/public/posts/go-untuk-pemula", "", nil)
	assert.Equal(t, 200, w.Code)
	assert.Equal(t, "penulis", data(out)["author"].(map[string]interface{})["username"])
	assert.Equal(t, "penulis", data(out)["author_name"])
	assert.NotContains(t, w.Body.String(), "user_id")
	w, out = e.call("GET", "/public/posts", "", nil)
	assert.Equal(t, "penulis", out["data"].([]interface{})[0].(map[string]interface{})["author_name"])
	assert.NotContains(t, w.Body.String(), "user_id")
	w, _ = e.call("GET", "/public/posts/draf-rahasia", "", nil)
	assert.Equal(t, 404, w.Code)
}

func TestAdminEndpoints(t *testing.T) {
	e := setup(t)
	e.call("POST", "/posts", e.author, gin.H{"title": "A", "content": "x", "is_published": true})
	e.call("POST", "/posts", e.author, gin.H{"title": "B", "content": "x"})

	w, _ := e.call("GET", "/admin/users", e.author, nil)
	assert.Equal(t, 403, w.Code)

	_, out := e.call("GET", "/admin/users", e.admin, nil)
	users := out["data"].([]interface{})
	require.Len(t, users, 3)
	first := users[0].(map[string]interface{})
	assert.Equal(t, "penulis", first["username"])
	assert.EqualValues(t, 2, first["total_posts"])
	assert.EqualValues(t, 1, first["total_published"])

	_, out = e.call("GET", "/admin/posts", e.admin, nil)
	assert.Len(t, out["data"], 2)

	_, out = e.call("GET", "/admin/users/"+itoa(int(e.authID))+"/posts", e.admin, nil)
	assert.Len(t, out["data"], 2)
	w, _ = e.call("GET", "/admin/users/9999/posts", e.admin, nil)
	assert.Equal(t, 200, w.Code) // pengguna tak ada -> daftar kosong
	w, _ = e.call("GET", "/admin/users/x/posts", e.admin, nil)
	assert.Equal(t, 400, w.Code)
}

func TestAutoExcerptTruncates(t *testing.T) {
	e := setup(t)
	_, out := e.call("POST", "/posts", e.author, gin.H{"title": "Panjang", "content": strings.Repeat("kata ", 100)})
	ex := data(out)["excerpt"].(string)
	assert.True(t, strings.HasSuffix(ex, "..."))
	assert.LessOrEqual(t, len([]rune(ex)), 163)
}

// ---- cabang error repository -> 500 ----

var errBoom = errors.New("boom")

type failRepo struct{}

func (failRepo) Create(*post.Post) error                         { return errBoom }
func (failRepo) Save(*post.Post) error                           { return errBoom }
func (failRepo) Delete(*post.Post) error                         { return errBoom }
func (failRepo) FindByID(uint) (*post.Post, error)               { return nil, errBoom }
func (failRepo) FindByIDAndUser(uint, uint) (*post.Post, error)  { return nil, errBoom }
func (failRepo) FindPublicBySlug(string) (*post.Post, error)     { return nil, errBoom }
func (failRepo) ListPublic(post.PostFilter) ([]post.Post, error) { return nil, errBoom }
func (failRepo) ListByUser(uint) ([]post.Post, error)            { return nil, errBoom }
func (failRepo) ListAll() ([]post.Post, error)                   { return nil, errBoom }
func (failRepo) SlugExists(string, uint) (bool, error)           { return false, errBoom }
func (failRepo) CountByUser(uint) (int64, int64, error)          { return 0, 0, errBoom }
func (failRepo) AdminUsers() ([]post.AdminUser, error)           { return nil, errBoom }
func (failRepo) UserExists(uint) (bool, error)                   { return false, errBoom }

func TestHandlersReturn500OnRepoError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := post.NewHandler(post.NewService(failRepo{}))
	tok, _ := middleware.NewToken(secret, 1, "u", "admin", time.Hour)
	r := gin.New()
	r.GET("/public/posts", h.PublicList)
	r.GET("/public/posts/:slug", h.PublicDetail)
	g := r.Group("/x", middleware.Auth(secret))
	g.GET("/mine", h.MyList)
	g.GET("/stats", h.Stats)
	g.GET("/p/:id", h.Get)
	g.POST("/p", h.Create)
	g.PUT("/p/:id", h.Update)
	g.PATCH("/p/:id", h.TogglePublish)
	g.DELETE("/p/:id", h.Delete)
	g.GET("/au", h.AdminUsers)
	g.GET("/ap", h.AdminPosts)
	g.GET("/aup/:id", h.AdminUserPosts)

	body := `{"title":"t","content":"c"}`
	cases := []struct{ m, p, b string }{
		{"GET", "/public/posts", ""}, {"GET", "/public/posts/a", ""},
		{"GET", "/x/mine", ""}, {"GET", "/x/stats", ""}, {"GET", "/x/p/1", ""},
		{"POST", "/x/p", body}, {"PUT", "/x/p/1", body}, {"PATCH", "/x/p/1", ""},
		{"DELETE", "/x/p/1", ""}, {"GET", "/x/au", ""}, {"GET", "/x/ap", ""}, {"GET", "/x/aup/1", ""},
	}
	for _, c := range cases {
		req := httptest.NewRequest(c.m, c.p, strings.NewReader(c.b))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+tok)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code, c.m+" "+c.p)
	}
}

func itoa(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}

func TestRepositoryErrorsWhenTablesMissing(t *testing.T) {
	db, err := database.NewTestDB()
	require.NoError(t, err)
	repo := post.NewRepository(db)

	require.NoError(t, db.Migrator().DropTable(&post.Post{}))
	_, err = repo.AdminUsers() // query agregat posts gagal
	assert.Error(t, err)

	require.NoError(t, db.Migrator().DropTable(&auth.User{}))
	_, err = repo.AdminUsers() // query users gagal
	assert.Error(t, err)
}

// flaky membungkus repository asli dan menyuntikkan error pada operasi tertentu.
type flaky struct {
	post.Repository
	slugErr, saveErr, createErr error
}

func (f flaky) SlugExists(s string, id uint) (bool, error) {
	if f.slugErr != nil {
		return false, f.slugErr
	}
	return f.Repository.SlugExists(s, id)
}

func (f flaky) Save(p *post.Post) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	return f.Repository.Save(p)
}

func (f flaky) Create(p *post.Post) error {
	if f.createErr != nil {
		return f.createErr
	}
	return f.Repository.Create(p)
}

func TestServiceWriteErrors(t *testing.T) {
	db, err := database.NewTestDB()
	require.NoError(t, err)
	u := &auth.User{Username: "u", Email: "u@mail.com", Password: "x", Role: "member"}
	require.NoError(t, db.Create(u).Error)
	repo := post.NewRepository(db)

	p, err := post.NewService(repo).Create(u.ID, post.PostRequest{Title: "Judul", Content: "isi"})
	require.NoError(t, err)

	_, err = post.NewService(flaky{Repository: repo, createErr: errBoom}).Create(u.ID, post.PostRequest{Title: "Lain", Content: "x"})
	assert.ErrorIs(t, err, errBoom)

	// judul berubah -> uniqueSlug gagal
	_, err = post.NewService(flaky{Repository: repo, slugErr: errBoom}).Update(u.ID, p.ID, post.PostRequest{Title: "Judul Baru", Content: "x"})
	assert.ErrorIs(t, err, errBoom)

	// judul sama -> Save gagal
	_, err = post.NewService(flaky{Repository: repo, saveErr: errBoom}).Update(u.ID, p.ID, post.PostRequest{Title: "Judul", Content: "x"})
	assert.ErrorIs(t, err, errBoom)

	_, err = post.NewService(flaky{Repository: repo, saveErr: errBoom}).TogglePublish(u.ID, p.ID)
	assert.ErrorIs(t, err, errBoom)
}
func TestRepositoryFindByIDNotFound(t *testing.T) {
	db, err := database.NewTestDB()
	require.NoError(t, err)
	_, err = post.NewRepository(db).FindByID(9999)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}
