package utilities
import (
	"context"
	"errors"
	"time"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)
var ErrShortCodeTaken=errors.New("short code already taken")
var ErrURLNotFound = errors.New("short code not found")

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505" // unique_violation
	}
	return false
}
func CodeExists(ctx context.Context, pool *pgxpool.Pool, shortCode string) (bool, error) {
	var exists bool
	err := pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM urls WHERE short_code = $1)`,
		shortCode,
	).Scan(&exists)
	return exists, err
}
func GetLongURL(ctx context.Context, pool *pgxpool.Pool, shortCode string) (string, *time.Time, error) {
	var longURL string
	var expiresAt *time.Time
 
	err := pool.QueryRow(ctx,
		`SELECT long_url, expires_at FROM urls WHERE short_code = $1`,
		shortCode,
	).Scan(&longURL, &expiresAt)
 
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil, ErrURLNotFound
	}
	if err != nil {
		return "", nil, fmt.Errorf("get long url: %w", err)
	}
	if expiresAt != nil && time.Now().After(*expiresAt) {
		return "", nil, ErrURLNotFound
	}
 
	return longURL, expiresAt, nil
}
 
// CreateURL inserts a new short_code -> long_url mapping.
func CreateURL(ctx context.Context, pool *pgxpool.Pool, shortCode, longURL string, expiresAt *time.Time) error {
	_, err := pool.Exec(ctx,
		`INSERT INTO urls (short_code, long_url, expires_at) VALUES ($1, $2, $3)`,
		shortCode, longURL, expiresAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrShortCodeTaken
		}
		return err
	}
	return nil
}