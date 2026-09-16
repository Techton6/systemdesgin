package utilities
 
import (
	"context"
	"time"
 
	"github.com/redis/go-redis/v9"
)
const defaultCacheTTL = 24 * time.Hour

func cacheKey(shortCode string) string {
	return "url:" + shortCode
}
// getCachedURL returns the cached long URL for a short code.

func getCachedURL(ctx context.Context, rdb *redis.Client, shortCode string) (string, error) {
	return rdb.Get(ctx, cacheKey(shortCode)).Result()
}
// cacheURL stores a short_code -> long_url mapping with a TTL.

func cacheURL(ctx context.Context, rdb *redis.Client, shortCode, longURL string, expiresAt *time.Time) {
	ttl := defaultCacheTTL
	if expiresAt != nil {
		if remaining := time.Until(*expiresAt); remaining < ttl {
			ttl = remaining
		}
	}
	if ttl <= 0 {
		return
	}
	_ = rdb.Set(ctx, cacheKey(shortCode), longURL, ttl).Err()
}
 