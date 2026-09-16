package main

import (
	"context"
	"log"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"url-shortner/utilities"
	"url-shortner/base"
)

func main() {
	ctx := context.Background()
	cfg := utilities.LoadConfig()

	pool, err := base.NewDBPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to connect to db: %v", err)
	}
	defer pool.Close()

	rdb, err := base.NewRedisClient(ctx, cfg.RedisAddr)
	if err != nil {
		log.Fatalf("failed to connect to redis: %v", err)
	}
	defer rdb.Close()

	keygen := utilities.NewKeyGenerator()

	deps := &utilities.AppDeps{
		DB:      pool,
		Redis:   rdb,
		KeyGen:  keygen,
		BaseURL: cfg.BaseURL,
	}

	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// Step 3: write path.
	e.POST("/api/shorten", utilities.ShortenHandler(deps))

	// Step 4: read path. Registered after the more specific routes
	// below — Echo's router matches static paths first regardless,
	// but keeping it last here avoids any ambiguity as routes grow.
	e.GET("/:code", utilities.RedirectHandler(deps))

	// Step 2 debug endpoint: generates a code without persisting it.
	e.GET("/internal/keygen", func(c echo.Context) error {
		code, err := keygen.NextCode()
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]string{"short_code": code})
	})

	e.GET("/healthz", func(c echo.Context) error {
		if err := rdb.Ping(c.Request().Context()).Err(); err != nil {
			return c.String(http.StatusServiceUnavailable, "redis unavailable")
		}
		return c.String(http.StatusOK, "ok")
	})

	log.Println("server starting on :8080")
	e.Logger.Fatal(e.Start(":8080"))
}