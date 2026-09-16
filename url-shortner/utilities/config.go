package utilities
import "os"

type Config struct {
	DatabaseURL string
	RedisAddr   string
	BaseURL     string
}

func LoadConfig() Config {
	return Config{
		DatabaseURL: getEnv("DATABASE_URL", ""),
		RedisAddr:   getEnv("REDIS_ADDR", ""),
		BaseURL:     getEnv("BASE_URL", "http://localhost:8080"),
	}
}
 
func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}