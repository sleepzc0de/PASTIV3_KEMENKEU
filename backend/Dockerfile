# ===== Stage 1: Build =====
FROM golang:1.27-alpine AS builder

WORKDIR /app

# Cache dependency layer terpisah dari source code
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build binary statis (CGO_ENABLED=0 penting untuk image alpine minimal)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /app/pasti-backend ./cmd/main.go

# ===== Stage 2: Runtime (image minimal) =====
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/pasti-backend .

# Jalankan sebagai non-root user demi keamanan
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
USER appuser

EXPOSE 8686

ENTRYPOINT ["./pasti-backend"]