import type { NextConfig } from "next";
import { jalurMasukDari, nilaiUntukBuild } from "./lib/loginPath";

// Alamat halaman login yang tidak bisa ditebak (lihat lib/loginPath.ts). Dibaca saat build dan disisipkan ke middleware
// sebagai konstanta; tidak dipakai komponen klien, jadi tidak ikut ke bundel JavaScript yang diunduh pengunjung.
// Build Docker (frontend/Dockerfile, LOGIN_PATH_WAJIB=1) menolak nilai kosong supaya /login tidak terbuka di server tanpa disengaja;
// deploy.sh selalu mengisinya. Build/jalankan lokal tanpa nilai tetap memakai /login (dengan peringatan saat build produksi).
const loginPath = jalurMasukDari(process.env.LOGIN_PATH);
if (loginPath === "/login") {
  const cara = "isi LOGIN_PATH (deploy.sh membuatnya di deploy.env), atau buat manual: node -e \"console.log('/'+require('crypto').randomBytes(16).toString('hex'))\"";
  if (process.env.LOGIN_PATH_WAJIB === "1") {
    throw new Error("LOGIN_PATH belum diisi. Build ini menyembunyikan halaman login di alamat acak; " + cara);
  }
  if (process.env.NODE_ENV === "production") {
    console.warn("[PERINGATAN] LOGIN_PATH kosong: halaman login tetap di /login (tidak disembunyikan). Untuk menyembunyikannya, " + cara);
  }
}

const nextConfig: NextConfig = {
  output: "standalone",

  env: {
    // Kosong = tidak disembunyikan (pengembangan lokal); middleware membaca kosong sebagai /login.
    LOGIN_PATH: nilaiUntukBuild(loginPath),
  },

  async headers() {
    return [
      {
        source: "/:path*",
        headers: [
          {
            key: "X-Frame-Options",
            value: "SAMEORIGIN",
          },
          {
            key: "Content-Security-Policy",
            value: "frame-ancestors 'self';",
          },
          {
            key: "X-Content-Type-Options",
            value: "nosniff",
          },
          {
            key: "Referrer-Policy",
            value: "strict-origin-when-cross-origin",
          },
          {
            key: "X-XSS-Protection",
            value: "1; mode=block",
          },
        ],
      },
    ];
  },
};

export default nextConfig;
