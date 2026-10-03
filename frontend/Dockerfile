# ===== Stage 1: Dependencies =====
FROM node:20-alpine AS deps
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci

# ===== Stage 2: Build =====
FROM node:20-alpine AS builder
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .

# Env public HARUS ada saat build karena di-inject ke bundle client-side
ARG NEXT_PUBLIC_API_URL
ARG NEXT_PUBLIC_API_ROOT_URL
ENV NEXT_PUBLIC_API_URL=$NEXT_PUBLIC_API_URL
ENV NEXT_PUBLIC_API_ROOT_URL=$NEXT_PUBLIC_API_ROOT_URL
# Opsional: server ubin peta untuk halaman Digitalisasi Aset (kosong = OpenStreetMap).
ARG NEXT_PUBLIC_MAP_TILE_URL
ARG NEXT_PUBLIC_MAP_TILE_ATTRIBUTION
ENV NEXT_PUBLIC_MAP_TILE_URL=$NEXT_PUBLIC_MAP_TILE_URL
ENV NEXT_PUBLIC_MAP_TILE_ATTRIBUTION=$NEXT_PUBLIC_MAP_TILE_ATTRIBUTION
# Alamat halaman login yang tidak bisa ditebak. Hanya dibaca saat build (next.config.ts) dan disisipkan ke middleware; tidak masuk
# bundel klien dan tidak perlu ada di image runtime.
ARG LOGIN_PATH
ENV LOGIN_PATH=$LOGIN_PATH
# Build image (server) menolak LOGIN_PATH kosong supaya /login tidak terbuka tanpa disengaja; build lokal tidak diwajibkan.
ENV LOGIN_PATH_WAJIB=1

RUN npm run build

# ===== Stage 3: Runtime (image minimal) =====
FROM node:20-alpine AS runner
WORKDIR /app

ENV NODE_ENV=production

RUN addgroup -S nodejs && adduser -S nextjs -G nodejs

COPY --from=builder /app/public ./public
COPY --from=builder --chown=nextjs:nodejs /app/.next/standalone ./
COPY --from=builder --chown=nextjs:nodejs /app/.next/static ./.next/static

USER nextjs

EXPOSE 3000
ENV PORT=3000
ENV HOSTNAME="0.0.0.0"

CMD ["node", "server.js"]