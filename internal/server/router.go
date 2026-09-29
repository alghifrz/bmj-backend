package server

import (
	"net/http"

	"bmj-backend/internal/analytics"
	"bmj-backend/internal/auth"
	"bmj-backend/internal/categories"
	"bmj-backend/internal/dashboard"
	"bmj-backend/internal/health"
	"bmj-backend/internal/products"
	"bmj-backend/internal/reviews"
	"bmj-backend/internal/storage"
	"bmj-backend/internal/store"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

func registerRoutes(engine *gin.Engine, pool *pgxpool.Pool, jwtSecret string, objects *storage.Client) {
	engine.GET("/health", health.Handler)

	v1 := engine.Group("/api/v1")
	v1.GET("/health", health.Handler)
	v1.GET("/health/db", databaseHealth(pool))
	v1.GET("/products", products.ListHandler(pool))
	v1.GET("/products/:slug", products.GetHandler(pool))
	v1.GET("/categories", categories.ListHandler(pool))
	v1.GET("/categories/:slug", categories.GetHandler(pool))
	v1.GET("/reviews", reviews.ListHandler(pool))
	v1.GET("/store", store.GetHandler(pool))
	v1.POST("/analytics/events", analytics.RecordHandler(pool))

	v1.POST("/admin/login", auth.LoginHandler(pool, jwtSecret))
	admin := v1.Group("/admin")
	admin.Use(auth.RequireAdmin(pool, jwtSecret))
	admin.GET("/me", auth.MeHandler())
	admin.PATCH("/me", auth.UpdateHandler(pool, jwtSecret))
	admin.GET("/dashboard", dashboard.AdminHandler(pool))
	admin.POST("/logout", auth.LogoutHandler())
	admin.GET("/products", products.AdminListHandler(pool))
	admin.POST("/products", products.AdminCreateHandler(pool))
	admin.PATCH("/products/:id", products.AdminUpdateHandler(pool))
	admin.DELETE("/products/:id", products.AdminDeleteHandler(pool, objects))
	admin.GET("/products/:id/images", products.AdminImageListHandler(pool))
	admin.POST("/products/:id/images", products.AdminImageCreateHandler(pool, objects))
	admin.PATCH("/products/:id/images/:imageId", products.AdminImageUpdateHandler(pool))
	admin.DELETE("/products/:id/images/:imageId", products.AdminImageDeleteHandler(pool, objects))
	admin.GET("/categories", categories.AdminListHandler(pool))
	admin.POST("/categories", categories.AdminCreateHandler(pool))
	admin.PATCH("/categories/:id", categories.AdminUpdateHandler(pool))
	admin.DELETE("/categories/:id", categories.AdminDeleteHandler(pool, objects))
	admin.POST("/categories/:id/image", categories.AdminImageHandler(pool, objects))
	admin.DELETE("/categories/:id/image", categories.AdminImageDeleteHandler(pool, objects))
	admin.GET("/reviews", reviews.AdminListHandler(pool))
	admin.POST("/reviews", reviews.AdminCreateHandler(pool))
	admin.PATCH("/reviews/:id", reviews.AdminUpdateHandler(pool))
	admin.DELETE("/reviews/:id", reviews.AdminDeleteHandler(pool, objects))
	admin.POST("/reviews/:id/image", reviews.AdminImageHandler(pool, objects))
	admin.DELETE("/reviews/:id/image", reviews.AdminImageDeleteHandler(pool, objects))
	admin.GET("/store", store.AdminGetHandler(pool))
	admin.PATCH("/store", store.AdminUpdateHandler(pool))
	admin.POST("/store/locations", store.AdminCreateLocationHandler(pool))
	admin.PATCH("/store/locations/:id", store.AdminUpdateLocationHandler(pool))
	admin.DELETE("/store/locations/:id", store.AdminDeleteLocationHandler(pool))

	engine.NoRoute(func(c *gin.Context) {
		JSONError(c, http.StatusNotFound, codeNotFound, "Not found")
	})
}
