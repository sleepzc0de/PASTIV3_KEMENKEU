// Jalan pintas untuk callback login SSO. Berkas ini murni (tanpa Next/React) supaya aturannya bisa diuji; dibaca oleh next.config.ts.
//
// Setelah pengguna masuk di SSO Kemenkeu, peramban dikembalikan ke https://<domain>/sso/callback/login?code=...&state=..., yang HARUS dijawab backend
// (menukar kode dengan token lalu mengalihkan ke /sso/callback di frontend). Bila proxy di depan domain salah mengarahkan path itu ke frontend, pengguna
// melihat halaman 404 dan login SSO tidak pernah selesai. Cara yang benar adalah memperbaiki aturan proxy (lihat docs/deploy-vps.md); bila itu belum
// bisa, SSO_CALLBACK_VIA_FRONTEND=1 membuat frontend sendiri meneruskan PERSIS path itu ke backend lewat jaringan Docker. Mati secara bawaan.
//
// Jalan pintas yang lebih luas, BACKEND_VIA_FRONTEND=1, meneruskan SEMUA rute milik backend (/api/, /health, /sso/login, /sso/callback/login) untuk
// proxy atau pemetaan alamat (IP masking) yang mengirim seluruh domain ke port frontend. Rutenya sama dengan blok location backend di
// deploy/nginx/pasti.kemenkeu.go.id.conf; pendaftaran akun publik tetap ditutup (middleware.ts, JALUR_DAFTAR_DITUTUP).
//
// Aturan rewrite dibaca Next saat build (bukan saat jalan), jadi mengubah nilainya memerlukan build ulang (./deploy.sh melakukannya).

export const JALUR_CALLBACK_SSO = "/sso/callback/login";
// Rute backend yang diteruskan BACKEND_VIA_FRONTEND (source Next; :path+ = satu segmen atau lebih setelah /api/, seperti "location ~ ^/api/" di Nginx).
export const RUTE_BACKEND = ["/api/:path+", "/health", "/sso/login", JALUR_CALLBACK_SSO];
// Pendaftaran akun publik: ditutup seperti di Nginx ("location = /api/v1/auth/register { return 404; }"), juga saat /api diteruskan frontend.
export const JALUR_DAFTAR_DITUTUP = "/api/v1/auth/register";
// Batas proxy Next untuk rewrite ke backend. Bawaan Next memutus permintaan setelah 30 detik dan hanya meneruskan 10 MB pertama badan permintaan
// (sisanya terpotong, bukan ditolak). Disamakan dengan Nginx (proxy_read_timeout 900s) dan dibuat di atas batas backend (BATAS_BADAN_MB, bawaan 12) supaya
// backend sendiri yang menolak unggahan terlalu besar dengan pesan yang jelas.
export const BATAS_WAKTU_PROXY_MS = 900_000;
export const BATAS_BADAN_PROXY = "16mb";
// Nama layanan backend di jaringan Docker Compose (docker-compose.yml) dan port di dalam container.
export const BACKEND_INTERNAL_BAWAAN = "http://backend:8686";

const POLA_BACKEND = /^https?:\/\/[A-Za-z0-9._-]+(:[0-9]{1,5})?$/;

export interface OpsiCallbackSSO {
  nyala?: string | null; // SSO_CALLBACK_VIA_FRONTEND: "1" menyalakan; kosong atau "0" mematikan
  backend?: string | null; // BACKEND_INTERNAL_URL: alamat backend dari dalam container frontend (tanpa path)
}

export interface OpsiJalanPintas extends OpsiCallbackSSO {
  semua?: string | null; // BACKEND_VIA_FRONTEND: "1" meneruskan semua rute backend (mencakup callback SSO)
}

// Hanya "1" menyalakan dan hanya kosong atau "0" mematikan: nilai lain (mis. "true", "ya", salah ketik) ditolak supaya jalan pintas tidak menyala atau
// mati diam-diam karena salah tulis.
function saklar(nama: string, nilai: string | null | undefined): boolean {
  const v = (nilai ?? "").trim();
  if (v === "" || v === "0") return false;
  if (v === "1") return true;
  throw new Error(nama + ' harus 1 (menyalakan) atau 0/kosong (mematikan), bukan "' + v + '"');
}

export function callbackViaFrontendNyala(nilai: string | null | undefined): boolean {
  return saklar("SSO_CALLBACK_VIA_FRONTEND", nilai);
}

export function backendViaFrontendNyala(nilai: string | null | undefined): boolean {
  return saklar("BACKEND_VIA_FRONTEND", nilai);
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

// Aturan yang dipakai next.config.ts: semua rute backend bila BACKEND_VIA_FRONTEND=1 (callback SSO sudah termasuk), selain itu jalan pintas callback saja
// (atau kosong). Kedua nilai selalu diperiksa supaya salah tulis di salah satunya tetap menggagalkan build.
export function aturanJalanPintas(o: OpsiJalanPintas): AturanRewrite[] {
  const callback = aturanCallbackSSO(o);
  if (!backendViaFrontendNyala(o.semua)) return callback;
  const backend = backendInternalDari(o.backend);
  return RUTE_BACKEND.map((source) => ({ source, destination: backend + source }));
}
