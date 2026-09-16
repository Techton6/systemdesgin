package utilities

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)
type AppDeps struct {
	DB      *pgxpool.Pool
	Redis   *redis.Client
	KeyGen  *KeyGenerator
	BaseURL string
}
type shortenRequest struct {
	URL             string `json:"url"`
	Alias           string `json:"alias,omitempty"`
	ExpiresInSeconds *int  `json:"expires_in_seconds,omitempty"`
}
 
type shortenResponse struct {
	ShortCode string `json:"short_code"`
	ShortURL  string `json:"short_url"`
}

const maxKeygenAttempts = 3 
var errKeygenExhausted = errors.New("failed to generate a unique code after retries")
 
// ShortenHandler is the write path: validate the long URL,
// generate or validate a short code, persist the mapping.
func ShortenHandler(deps *AppDeps) echo.HandlerFunc {
	return func(c echo.Context) error {
		var req shortenRequest
		if err := c.Bind(&req); err != nil {
			return errJSON(c, http.StatusBadRequest, "invalid request body")
		}
 
		if err := validateLongURL(req.URL); err != nil {
			return errJSON(c, http.StatusBadRequest, err.Error())
		}
 
		expiresAt := computeExpiry(req.ExpiresInSeconds)
		ctx := c.Request().Context()
 
		var shortCode string
		var err error
		if req.Alias != "" {
			shortCode, err = createWithAlias(ctx, deps, req.Alias, req.URL, expiresAt)
		} else {
			shortCode, err = createWithGeneratedCode(ctx, deps, req.URL, expiresAt)
		}
 
		switch {
		case errors.Is(err, ErrShortCodeTaken):
			return errJSON(c, http.StatusConflict, "alias already taken")
		case errors.Is(err, errKeygenExhausted):
			return errJSON(c, http.StatusInternalServerError, "failed to generate a unique code")
		case err != nil:
			return errJSON(c, http.StatusInternalServerError, "internal error")
		}
 
		return c.JSON(http.StatusCreated, shortenResponse{
			ShortCode: shortCode,
			ShortURL:  deps.BaseURL + "/" + shortCode,
		})
	}
}
func errJSON(c echo.Context, status int, msg string) error {
	return c.JSON(status, map[string]string{"error": msg})
}
 
// validateLongURL ensures the given string is an absolute http(s) URL.
func validateLongURL(raw string) error {
	parsed, err := url.ParseRequestURI(raw)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return errors.New("url must be a valid absolute http(s) URL")
	}
	return nil
}
func computeExpiry(seconds *int) *time.Time {
	if seconds == nil {
		return nil
	}
	t := time.Now().Add(time.Duration(*seconds) * time.Second)
	return &t
}

// createWithAlias persists a user-chosen short code after checking
// availability. Returns ErrShortCodeTaken if it's unavailable —
// whether caught by the pre-check or the insert itself.
func createWithAlias(ctx context.Context, deps *AppDeps, alias, longURL string, expiresAt *time.Time) (string, error) {
	taken, err := CodeExists(ctx, deps.DB, alias)
	if err != nil {
		return "", fmt.Errorf("check alias: %w", err)
	}
	if taken {
		return "", ErrShortCodeTaken
	}
	if err := CreateURL(ctx, deps.DB, alias, longURL, expiresAt); err != nil {
		return "", err 
	}
	return alias, nil
}

func createWithGeneratedCode(ctx context.Context, deps *AppDeps, longURL string, expiresAt *time.Time) (string, error) {
	for attempt := 0; attempt < maxKeygenAttempts; attempt++ {
		code, err := deps.KeyGen.NextCode()
		if err != nil {
			return "", fmt.Errorf("generate code: %w", err)
		}

		if err := CreateURL(ctx, deps.DB, code, longURL, expiresAt); err != nil {
			if errors.Is(err, ErrShortCodeTaken) {
				continue 
			}
			return "", err 
		}

		return code, nil
	}
	return "", errKeygenExhausted
}
// RedirectHandler is the read path check Redis first, fall back to the DB on a
// miss, then populate the cache before responding.
func RedirectHandler(deps *AppDeps) echo.HandlerFunc {
	return func(c echo.Context) error {
		code := c.Param("code")
		ctx := c.Request().Context()
 
		if longURL, err := getCachedURL(ctx, deps.Redis, code); err == nil {
			return c.Redirect(http.StatusFound, longURL)
		}

		longURL, expiresAt, err := GetLongURL(ctx, deps.DB, code)
		if err != nil {
			if errors.Is(err, ErrURLNotFound) {
				return errJSON(c, http.StatusNotFound, "short URL not found")
			}
			return errJSON(c, http.StatusInternalServerError, "internal error")
		}
 
		cacheURL(ctx, deps.Redis, code, longURL, expiresAt)
 
		return c.Redirect(http.StatusFound, longURL)
	}
}