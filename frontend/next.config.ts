import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import type { NextConfig } from "next";
import { jalurMasukDari } from "./lib/loginPath";
import { aturanCallbackSSO } from "./lib/ssoCallback";

// Alamat halaman login yang tidak bisa ditebak (lihat lib/loginPath.ts). Bawaannya ada di login-path.txt, satu sumber untuk
// `npm run dev`/`build` lokal, build Docker, dan deploy.sh, jadi tidak perlu mengisi apa pun. LOGIN_PATH di lingkungan (deploy.<env>.env
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

// Versi aplikasi (tampil di footer dan halaman login): satu sumber, field "version" di package.json. Dinaikkan dengan `npm version patch|minor`
// (lihat docs/versi-aplikasi.md). Dibaca saat build dan disisipkan ke bundel sebagai konstanta.
function versiAplikasi(): string {
  try {
    const v = JSON.parse(readFileSync(resolve(process.cwd(), "package.json"), "utf8")).version;
    return typeof v === "string" ? v.trim() : "";
  } catch {
    return "";
  }
}

const loginPath = jalurMasukDari(process.env.LOGIN_PATH?.trim() || alamatBawaan());
if (loginPath === "/login" && process.env.NODE_ENV === "production") {
  console.warn("[PERINGATAN] Halaman login tidak disembunyikan (LOGIN_PATH=/login atau login-path.txt tidak ada): alamatnya tetap /login.");
}

// Jalan pintas callback SSO (mati bawaan; lihat lib/ssoCallback.ts): SSO_CALLBACK_VIA_FRONTEND=1 membuat frontend meneruskan /sso/callback/login ke backend
// bila proxy di depan domain salah mengarahkannya ke sini. Dibaca saat build; salah isi menggagalkan build, bukan diam-diam mati.
const aturanSSO = aturanCallbackSSO({ nyala: process.env.SSO_CALLBACK_VIA_FRONTEND, backend: process.env.BACKEND_INTERNAL_URL });
if (aturanSSO.length > 0) {
  console.warn("[PERINGATAN] SSO_CALLBACK_VIA_FRONTEND=1: frontend meneruskan " + aturanSSO[0].source + " ke " + aturanSSO[0].destination + ". Perbaiki aturan proxy bila memungkinkan.");
}

const nextConfig: NextConfig = {
  output: "standalone",

  env: {
    LOGIN_PATH: loginPath,
    NEXT_PUBLIC_APP_VERSION: versiAplikasi(),
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

  // Kosong kecuali jalan pintas callback SSO menyala. Larik biasa = dicek setelah halaman dan berkas statis, jadi tidak menimpa rute aplikasi mana pun.
  async rewrites() {
    return aturanSSO;
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
