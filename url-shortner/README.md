# URL Shortener

A small URL-shortening service written in Go. It provides an HTTP API for creating short URLs, stores URL mappings in PostgreSQL, and uses Redis to cache redirect lookups.

## Features

- Create a short URL from a long HTTP or HTTPS URL.
- Choose a custom alias, or let the service generate a shortcode.
- Redirect shortcodes to their original URLs.
- Cache redirect lookups in Redis.
- Support optional link expiration.
- Run the application, PostgreSQL, Redis, and Nginx with Docker Compose.

## Architecture

The service has four runtime components:

- **Go application**: validates requests, creates mappings, redirects visitors, and coordinates PostgreSQL and Redis.
- **PostgreSQL**: the source of truth for shortcode-to-URL mappings.
- **Redis**: a performance cache for redirect lookups. Redis is not the source of truth.
- **Nginx**: accepts requests on port 80 and proxies them to the Go application.

The Go application is also published directly on port 8080 for debugging.

## Shortcode and Alias

A **shortcode** is the final identifier used in the short URL:

```text
http://localhost/{short_code}
```

An **alias** is an optional custom shortcode supplied by the user.

### With an alias

Request:

```json
{
  "url": "https://github.com",
  "alias": "github"
}
```

The alias becomes the shortcode:

```json
{
  "short_code": "github",
  "short_url": "http://localhost/github"
}
```

The alias must be unique. Reusing it returns `409 Conflict`.

### Without an alias

Request:

```json
{
  "url": "https://github.com"
}
```

The key generator creates a random seven-character base62 shortcode, for example:

```json
{
  "short_code": "MZbShwt",
  "short_url": "http://localhost/MZbShwt"
}
```

The generated shortcode is then saved in PostgreSQL. The key generator only creates a candidate code; it does not save a URL by itself.

The `/internal/keygen` endpoint is a debug endpoint that generates and returns a code without persisting it. A code returned only by that endpoint cannot redirect to a URL.

## Request Flow

### Creating a short URL

1. A client sends `POST /api/shorten` with a long URL.
2. The application validates that the URL uses HTTP or HTTPS.
3. If an alias is provided, the application uses it as the shortcode.
4. If no alias is provided, the key generator creates a random shortcode.
5. PostgreSQL stores the shortcode, original URL, creation time, and optional expiration time.
6. The application returns the shortcode and short URL.

PostgreSQL enforces uniqueness with the primary key on `short_code`. If a generated code collides with an existing code, the application generates another code and retries.

### Redirecting a visitor

When a visitor requests `GET /{code}`:

1. The application checks Redis for the code.
2. On a cache hit, it redirects immediately.
3. On a cache miss, it reads the mapping from PostgreSQL.
4. It stores the result in Redis with a default 24-hour TTL.
5. It redirects the visitor to the original URL.

If a link has an expiration time, the Redis TTL is capped so the cache entry does not outlive the link. An expired or unknown shortcode returns `404 Not Found`.

## Project Files

```text
.
├── base/
│   ├── db.go
│   ├── redis.go
│   └── schema.sql
├── utilities/
│   ├── cache.go
│   ├── config.go
│   ├── handlers.go
│   ├── key-gen.go
│   └── urls.go
├── Dockerfile
├── docker-compose.yml
├── main.go
├── nginx.CONF
├── go.mod
└── go.sum
```

### Application files

- `main.go`: loads configuration, connects to PostgreSQL and Redis, registers routes, and starts the Echo HTTP server.
- `utilities/handlers.go`: implements the shorten and redirect handlers, validates long URLs, handles aliases, and controls the create and redirect flows.
- `utilities/urls.go`: contains PostgreSQL operations for checking, inserting, and retrieving URL mappings.
- `utilities/key-gen.go`: generates cryptographically random seven-character base62 shortcodes.
- `utilities/cache.go`: reads and writes Redis cache entries using keys such as `url:github` and applies cache expiration.
- `utilities/config.go`: loads `DATABASE_URL`, `REDIS_ADDR`, and `BASE_URL` from the environment with local defaults.

### Infrastructure files

- `base/db.go`: creates and verifies the PostgreSQL connection pool.
- `base/redis.go`: creates and verifies the Redis client.
- `base/schema.sql`: creates the `urls` table and its creation-time index. Docker runs this file only when the PostgreSQL data volume is initialized for the first time.
- `Dockerfile`: builds a static Go binary in a Go builder image and runs it in a smaller Alpine runtime image.
- `docker-compose.yml`: defines the Go application, PostgreSQL, Redis, and Nginx services, including health checks, volumes, environment variables, and dependencies.
- `nginx.CONF`: proxies incoming port 80 requests to the Go application on port 8080.
- `go.mod` and `go.sum`: define and lock the Go dependencies.

## Running with Docker Compose

From the project directory:

```powershell
docker compose up -d --build
```

Check service status:

```powershell
docker compose ps
```

The database, Redis, and application should be healthy. View logs with:

```powershell
docker compose logs -f app
```

The service is available at:

- Nginx: `http://localhost`
- Go application directly: `http://localhost:8080`

To stop the services without deleting data:

```powershell
docker compose down
```

To stop the services and delete PostgreSQL and Redis volumes:

```powershell
docker compose down -v
```

Use `docker compose down -v` when the database schema or initialization process must run again from an empty database. This also deletes existing stored mappings and cached data.

## API Endpoints

### Health check

```text
GET /healthz
```

Returns `ok` with status `200` when Redis is reachable.

### Create a short URL

```text
POST /api/shorten
Content-Type: application/json
```

Request with a custom alias:

```json
{
  "url": "https://github.com",
  "alias": "github"
}
```

Request without an alias:

```json
{
  "url": "https://github.com"
}
```

Optional expiration, in seconds:

```json
{
  "url": "https://github.com",
  "expires_in_seconds": 3600
}
```

Successful requests return `201 Created`:

```json
{
  "short_code": "github",
  "short_url": "http://localhost/github"
}
```

Possible errors include:

- `400 Bad Request`: invalid JSON or a URL that is not an absolute HTTP or HTTPS URL.
- `409 Conflict`: the requested alias already exists.
- `500 Internal Server Error`: an unexpected database, Redis, or shortcode-generation failure.

### Redirect

```text
GET /{short_code}
```

Returns `302 Found` and redirects to the original URL. Unknown or expired codes return `404 Not Found`.

### Generate a debug shortcode

```text
GET /internal/keygen
```

Returns a generated code, but does not store a URL mapping. This endpoint is for inspecting the key generator only.

## Testing the API

PowerShell's native HTTP client avoids JSON quoting issues that can occur when calling `curl.exe` from PowerShell:

```powershell
$body = '{"url":"https://github.com","alias":"github-test"}'

Invoke-RestMethod `
  -Uri http://localhost/api/shorten `
  -Method Post `
  -ContentType "application/json" `
  -Body $body
```

Open the returned `short_url` in a browser, or request it directly:

```powershell
Invoke-WebRequest -Uri http://localhost/github-test -MaximumRedirection 0
```

A successful test should show:

- `GET /healthz` returns `200` and `ok`.
- Creating a mapping returns `201`.
- Opening its short URL returns a redirect to the original URL.
- Reusing the same alias returns `409`.
- An unknown shortcode returns `404`.

## Configuration

The default Docker Compose configuration uses:

```text
DATABASE_URL=postgres://postgres:postgres@db:5432/urlshortener?sslmode=disable
REDIS_ADDR=redis:6379
BASE_URL=http://localhost
```

These values can be overridden through a `.env` file or environment variables. See `.env.example` for the available settings.

## Local Go Development

The application expects PostgreSQL and Redis to be reachable at the configured addresses. The simplest development setup is Docker Compose. To run only the Go application locally after its dependencies are available:

```powershell
go run .
```

The application listens on port `8080`.
