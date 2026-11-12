# --- dev: air hot-reload (bind mount) ---
FROM golang:1.27-alpine AS dev
WORKDIR /app
RUN go install github.com/air-verse/air@latest
COPY go.mod go.sum ./
RUN go mod download
COPY . .
CMD ["air"]

# --- prod: binary only, no air, no source (build target: docker build --target prod .) ---
FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /bot .

FROM alpine:3.20 AS prod
RUN apk add --no-cache ca-certificates
USER nobody
WORKDIR /app
COPY --from=build /bot /bot
# ownership untuk named volume sqlite (docker init volume ikut permission mountpoint)
RUN mkdir -p /app/data && chown -R nobody:nobody /app/data
CMD ["/bot"]
# ponytail: rootless via nobody + ca-certs saja; tambah non-root user khusus kalau perlu install timezone/apk lain
