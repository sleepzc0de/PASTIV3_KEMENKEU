import type { NextConfig } from "next";
import { jalurMasukDari, nilaiUntukBuild } from "./lib/loginPath";

// Alamat halaman login yang tidak bisa ditebak (lihat lib/loginPath.ts). Dibaca saat build dan disisipkan ke middleware
// sebagai konstanta; tidak dipakai komponen klien, jadi tidak ikut ke bundel JavaScript yang diunduh pengunjung.
// Build produksi wajib mengisinya (deploy.sh membuatnya otomatis); `next dev` tanpa nilai memakai /login seperti biasa.
const loginPath = jalurMasukDari(process.env.LOGIN_PATH);
if (process.env.NODE_ENV === "production" && loginPath === "/login") {
  throw new Error(
    "LOGIN_PATH belum diisi. Build produksi menyembunyikan halaman login di alamat acak; isi LOGIN_PATH (deploy.sh membuatnya di deploy.env) " +
      "atau, untuk membuatnya manual: node -e \"console.log('/'+require('crypto').randomBytes(16).toString('hex'))\""
  );
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
