import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import type { NextConfig } from "next";
import { jalurMasukDari } from "./lib/loginPath";

// Alamat halaman login yang tidak bisa ditebak (lihat lib/loginPath.ts). Bawaannya ada di login-path.txt, satu sumber untuk
// `npm run dev`/`build` lokal, build Docker, dan deploy.sh, jadi tidak perlu mengisi apa pun. LOGIN_PATH di lingkungan (deploy.env
// atau .env.local) menimpanya; LOGIN_PATH=/login menampilkan halaman login di /login seperti semula.
// Hasilnya dibaca saat build dan disisipkan ke middleware sebagai konstanta; tidak dipakai komponen klien, jadi tidak ikut ke bundel
// JavaScript yang diunduh pengunjung.
function alamatBawaan(): string {
  try {
    return readFileSync(resolve(process.cwd(), "login-path.txt"), "utf8").trim();
  } catch {
    return "";
  }
}

const loginPath = jalurMasukDari(process.env.LOGIN_PATH?.trim() || alamatBawaan());
if (loginPath === "/login" && process.env.NODE_ENV === "production") {
  console.warn("[PERINGATAN] Halaman login tidak disembunyikan (LOGIN_PATH=/login atau login-path.txt tidak ada): alamatnya tetap /login.");
}

const nextConfig: NextConfig = {
  output: "standalone",

  env: {
    LOGIN_PATH: loginPath,
  },

  // Menu lama (Data Aset SLDK, Pengadaan, Tender, E-Katalog V5/V6 dari Inaproc) sudah dihapus: isinya kini ada di Dashboard dan Pengadaan Terpadu.
  // Alamat lamanya (markah, tautan yang pernah dibagikan) dialihkan, bukan 404. Sementara (bukan permanen) supaya peramban tidak menyimpannya.
  async redirects() {
    return [
      { source: "/dashboard/assets", destination: "/dashboard", permanent: false },
      { source: "/dashboard/pengadaan", destination: "/dashboard?tab=pengadaan", permanent: false },
      { source: "/dashboard/pengadaan/:path*", destination: "/dashboard?tab=pengadaan", permanent: false },
      { source: "/dashboard/pengadaan-terpadu/dasbor", destination: "/dashboard?tab=pengadaan", permanent: false },
      { source: "/dashboard/tender/:path*", destination: "/dashboard/pengadaan-terpadu/penarikan", permanent: false },
      { source: "/dashboard/ekatalog/:path*", destination: "/dashboard/pengadaan-terpadu/penarikan", permanent: false },
      { source: "/dashboard/ekatalog-v6/:path*", destination: "/dashboard/pengadaan-terpadu/penarikan", permanent: false },
    ];
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
