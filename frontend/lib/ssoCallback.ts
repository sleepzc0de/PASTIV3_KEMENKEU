// Jalan pintas untuk callback login SSO. Berkas ini murni (tanpa Next/React) supaya aturannya bisa diuji; dibaca oleh next.config.ts.
//
// Setelah pengguna masuk di SSO Kemenkeu, peramban dikembalikan ke https://<domain>/sso/callback/login?code=...&state=..., yang HARUS dijawab backend
// (menukar kode dengan token lalu mengalihkan ke /sso/callback di frontend). Bila proxy di depan domain salah mengarahkan path itu ke frontend, pengguna
// melihat halaman 404 dan login SSO tidak pernah selesai. Cara yang benar adalah memperbaiki aturan proxy (lihat docs/deploy-vps.md); bila itu belum
// bisa, SSO_CALLBACK_VIA_FRONTEND=1 membuat frontend sendiri meneruskan PERSIS path itu ke backend lewat jaringan Docker. Mati secara bawaan.
//
// Aturan rewrite dibaca Next saat build (bukan saat jalan), jadi mengubah nilainya memerlukan build ulang (./deploy.sh melakukannya).

export const JALUR_CALLBACK_SSO = "/sso/callback/login";
// Nama layanan backend di jaringan Docker Compose (docker-compose.yml) dan port di dalam container.
export const BACKEND_INTERNAL_BAWAAN = "http://backend:8686";

const POLA_BACKEND = /^https?:\/\/[A-Za-z0-9._-]+(:[0-9]{1,5})?$/;

export interface OpsiCallbackSSO {
  nyala?: string | null; // SSO_CALLBACK_VIA_FRONTEND: "1" menyalakan; kosong atau "0" mematikan
  backend?: string | null; // BACKEND_INTERNAL_URL: alamat backend dari dalam container frontend (tanpa path)
}

// Hanya "1" menyalakan dan hanya kosong atau "0" mematikan: nilai lain (mis. "true", "ya", salah ketik) ditolak supaya jalan pintas tidak menyala atau
// mati diam-diam karena salah tulis.
export function callbackViaFrontendNyala(nilai: string | null | undefined): boolean {
  const v = (nilai ?? "").trim();
  if (v === "" || v === "0") return false;
  if (v === "1") return true;
  throw new Error('SSO_CALLBACK_VIA_FRONTEND harus 1 (menyalakan) atau 0/kosong (mematikan), bukan "' + v + '"');
}

export function backendInternalDari(nilai: string | null | undefined): string {
  const v = (nilai ?? "").trim().replace(/\/+$/, "");
  if (v === "") return BACKEND_INTERNAL_BAWAAN;
  if (!POLA_BACKEND.test(v)) {
    throw new Error("BACKEND_INTERNAL_URL harus berbentuk http://host[:port] tanpa path, mis. " + BACKEND_INTERNAL_BAWAAN);
  }
  return v;
}

export interface AturanRewrite {
  source: string;
  destination: string;
}

// Satu aturan: PERSIS /sso/callback/login (bukan awalan, bukan /sso/callback) ke path yang sama di backend. Query string (code, state) ikut diteruskan oleh
// Next. Kosong bila jalan pintas mati.
export function aturanCallbackSSO(o: OpsiCallbackSSO): AturanRewrite[] {
  if (!callbackViaFrontendNyala(o.nyala)) return [];
  return [{ source: JALUR_CALLBACK_SSO, destination: backendInternalDari(o.backend) + JALUR_CALLBACK_SSO }];
}
