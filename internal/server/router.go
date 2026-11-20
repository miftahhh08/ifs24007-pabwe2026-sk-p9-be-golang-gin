package server

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"gorm.io/gorm"

	"app/docs"
	"app/internal/config"
	"app/internal/features/ai"
	"app/internal/features/auth"
	"app/internal/features/post"
	"app/internal/middleware"
	"app/internal/response"
)

// New membuat router dengan client Search Engine & OpenAI sungguhan.
func New(cfg *config.Config, db *gorm.DB) *gin.Engine {
	return NewWithDeps(cfg, db,
		ai.NewSearchClient(cfg.SearchEngineEndpoint),
		ai.NewOpenAIClient(cfg.OpenAIEndpoint, cfg.OpenAIKey, cfg.OpenAIModel),
	)
}

// NewWithDeps memungkinkan injeksi Searcher/Completer (dipakai pengujian).
func NewWithDeps(cfg *config.Config, db *gorm.DB, searcher ai.Searcher, llm ai.Completer) *gin.Engine {
	if cfg.AppEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery(), middleware.CORS())

	authH := auth.NewHandler(auth.NewService(auth.NewRepository(db), cfg.AppKey))
	postH := post.NewHandler(post.NewService(post.NewRepository(db)))
	aiH := ai.NewHandler(ai.NewService(searcher, llm))

	requireAuth := middleware.Auth(cfg.AppKey)

	// Dokumentasi
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	r.GET("/api.json", func(c *gin.Context) {
		c.Data(http.StatusOK, "application/json; charset=utf-8", []byte(docs.Spec()))
	})

	api := r.Group("/api")

	// Publik
	pub := api.Group("/public")
	pub.GET("/categories", postH.Categories)
	pub.GET("/posts", postH.PublicList)
	pub.GET("/posts/:slug", postH.PublicDetail)

	// Auth
	a := api.Group("/auth")
	limiter := middleware.RateLimiter(600, time.Minute)
	a.POST("/register", limiter, authH.Register)
	a.POST("/login", limiter, authH.Login)
	a.POST("/logout", authH.Logout)
	a.GET("/profile", requireAuth, authH.GetProfile)
	a.PUT("/profile", requireAuth, authH.UpdateProfile)

	// Penulis
	p := api.Group("/posts", requireAuth)
	p.GET("", postH.MyList)
	p.POST("", postH.Create)
	p.GET("/stats", postH.Stats)
	p.POST("/ai-generate", middleware.RateLimiter(120, time.Minute), aiH.Generate)
	p.GET("/:id", postH.Get)
	p.PUT("/:id", postH.Update)
	p.PATCH("/:id/toggle-publish", postH.TogglePublish)
	p.DELETE("/:id", postH.Delete)

	// Admin
	adm := api.Group("/admin", requireAuth, middleware.RequireRole(auth.RoleAdmin))
	adm.GET("/users", postH.AdminUsers)
	adm.GET("/posts", postH.AdminPosts)
	adm.GET("/users/:id/posts", postH.AdminUserPosts)

	r.NoRoute(func(c *gin.Context) {
		response.Fail(c, http.StatusNotFound, "Endpoint not found")
	})
	return r
}
