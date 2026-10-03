# ===== Stage 1: Build =====
FROM golang:1.27-alpine AS builder

WORKDIR /app

# Cache dependency layer terpisah dari source code
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build binary statis (CGO_ENABLED=0 penting untuk image alpine minimal)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /app/pasti-backend ./cmd/main.go

# Pelari migrasi (dipanggil deploy.sh): memakai driver & konfigurasi yang sama dengan backend
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /app/pasti-migrate ./cmd/migrate

# Penyiap data Ringkasan/Pemantauan Data Aset (dijalankan manual atau terjadwal, lihat docs/sldk-sync.md)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /app/pasti-sldk-sync ./cmd/sldk-sync

# ===== Stage 2: Runtime (image minimal) =====
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/pasti-backend .
COPY --from=builder /app/pasti-migrate .
COPY --from=builder /app/pasti-sldk-sync .
COPY --from=builder /app/migrations ./migrations

# Jalankan sebagai non-root user demi keamanan
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
USER appuser

EXPOSE 8686

ENTRYPOINT ["./pasti-backend"]