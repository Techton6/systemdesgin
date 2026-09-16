# --- Stage 1: build ---
FROM golang:1.27-alpine AS build
WORKDIR /src

# Cache deps separately from source.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# CGO off => fully static binary that runs on bare alpine.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/urlshortener .

# --- Stage 2: runtime ---
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -u 10001 app

WORKDIR /app
COPY --from=build /out/urlshortener .
USER app

EXPOSE 8080
# main.go ignores os.Args, so pass no subcommand.
CMD ["./urlshortener"]