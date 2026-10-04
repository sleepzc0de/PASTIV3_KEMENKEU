// Alamat halaman login yang tidak bisa ditebak, dan aturan siapa boleh mengetahuinya. Berkas ini murni (tanpa Next/React) supaya
// aturannya bisa diuji. Nilai LOGIN_PATH sendiri hanya boleh dibaca (process.env.LOGIN_PATH) di middleware.ts dan next.config.ts:
// komponen klien boleh mengimpor konstanta di berkas ini (mis. JALUR_KEMBALI), tetapi tidak boleh membaca alamat loginnya, karena
// bundel JavaScript diunduh semua orang.
//
// Cara kerja:
//  - Halaman login (berkas app/(auth)/login) tetap ada di /login secara internal, tetapi dilayani di alamat acak LOGIN_PATH
//    (mis. /3f9a...): middleware menulis ulang alamat itu ke /login, dan permintaan langsung ke /login dijawab 404.
//  - Alamat itu hanya diberitahukan kepada peramban yang sudah pernah membukanya: server memasang cookie petunjuk (httpOnly,
//    namanya hanya ada di kode server) saat halaman login dilayani. Pengunjung lain (pemindai otomatis, penyerang yang menebak
//    /login, /admin, dan sejenisnya) mendapat 404 di /, /login, /dashboard, dan /kembali-masuk, tanpa pengalihan yang
//    membocorkan alamatnya. Cookie sesi sengaja TIDAK dianggap petunjuk: namanya terbaca di JavaScript klien, jadi bisa dipalsukan.
//    Akibatnya pengguna yang belum pernah membuka alamat baru harus diberi tahu alamatnya sekali.
//  - Tanpa LOGIN_PATH (pengembangan lokal), semuanya berjalan seperti semula di /login.
//
// Ini lapisan tambahan, bukan pengganti captcha, pembatasan percobaan login, dan kata sandi yang kuat.

export const LOGIN_BAWAAN = "/login";
export const JALUR_KEMBALI = "/kembali-masuk";
export const COOKIE_TOKEN = "pasti_access_token";
export const COOKIE_PETUNJUK = "pasti_jalur";
export const UMUR_PETUNJUK_DETIK = 365 * 24 * 60 * 60;

// Satu segmen alamat: huruf, angka, "_", "-", dan "=" (mis. hasil base64), 16-128 karakter. Titik dilarang: alamat bertitik dianggap
// berkas statis dan tidak melewati middleware. Nama rute aplikasi dilarang supaya tidak menimpanya. Deploy.sh membuat 32 heksadesimal
// acak; nilai buatan sendiri juga diterima, tetapi makin mudah ditebak makin kecil gunanya.
const POLA_JALUR = /^\/[A-Za-z0-9_=-]{16,128}$/;
const NAMA_RUTE_DIPAKAI = new Set(["login", "dashboard", "kembali-masuk", "halaman-tidak-ada", "sso", "api"]);

export function jalurMasukSah(s: string | undefined | null): s is string {
  return typeof s === "string" && POLA_JALUR.test(s) && !NAMA_RUTE_DIPAKAI.has(s.slice(1).toLowerCase());
}

// Nilai LOGIN_PATH menjadi alamat yang dipakai. Kosong berarti tidak dipakai (/login bawaan); nilai yang bentuknya salah
// ditolak dengan galat supaya salah ketik tidak diam-diam membuka /login.
export function jalurMasukDari(env: string | undefined | null): string {
  const nilai = (env ?? "").trim();
  if (nilai === "") return LOGIN_BAWAAN;
  if (!jalurMasukSah(nilai)) {
    throw new Error(
      "LOGIN_PATH harus berbentuk /<16-128 karakter huruf, angka, _, - atau => tanpa titik dan bukan nama rute aplikasi, mis. /3f9a1c07d2b84e65a0c1f7e93b2d4a68"
    );
  }
  return nilai;
}

// Peramban atau aplikasi pesan kadang menyandikan "=" menjadi %3D saat alamat disalin. Alamat yang setelah dibaca sandinya sama dengan
// jalur masuk diperlakukan sebagai jalur masuk; selain itu pathname dibiarkan apa adanya (tidak ada pembacaan sandi umum).
export function samakanJalur(pathname: string, jalurMasuk: string): string {
  if (pathname === jalurMasuk || !pathname.includes("%")) return pathname;
  try {
    return decodeURIComponent(pathname) === jalurMasuk ? jalurMasuk : pathname;
  } catch {
    return pathname;
  }
}

// Nilai yang disisipkan ke build (next.config.ts -> env): kosong untuk /login bawaan (tidak disembunyikan), selain itu alamatnya.
// Middleware membacanya kembali dengan jalurMasukDari; "/login" sendiri tidak lolos validasinya.
export function nilaiUntukBuild(jalur: string): string {
  return jalur === LOGIN_BAWAAN ? "" : jalur;
}

export interface Masukan {
  pathname: string;
  search: string; // termasuk "?" bila ada
  adaToken: boolean; // cookie sesi ada
  adaPetunjuk: boolean; // peramban ini pernah membuka halaman login
  jalurMasuk: string; // hasil jalurMasukDari
}

export type Aksi =
  | { jenis: "lanjut" }
  | { jenis: "tulis-ulang"; ke: string; setPetunjuk: boolean } // dilayani dari rute internal; alamat di peramban tetap
  | { jenis: "alihkan"; ke: string }
  | { jenis: "tidak-ada" };

// Menentukan perlakuan tiap permintaan halaman. Alamat login hanya muncul pada pengalihan untuk peramban yang sudah pernah
// membuka halaman login (cookie petunjuk).
export function tentukanAksi(m: Masukan): Aksi {
  const { pathname, search, adaToken, adaPetunjuk, jalurMasuk } = m;
  const tersembunyi = jalurMasuk !== LOGIN_BAWAAN;
  // Pengunjung yang tidak mengenal alamat login: 404 bila disembunyikan, selain itu arahkan ke /login seperti biasa.
  const takDikenal: Aksi = tersembunyi ? { jenis: "tidak-ada" } : { jenis: "alihkan", ke: LOGIN_BAWAAN };

  if (pathname === jalurMasuk) {
    if (adaToken) return { jenis: "alihkan", ke: "/dashboard" };
    return tersembunyi ? { jenis: "tulis-ulang", ke: LOGIN_BAWAAN + search, setPetunjuk: !adaPetunjuk } : { jenis: "lanjut" };
  }
  // Rute internal tidak boleh dibuka langsung bila alamat aslinya disembunyikan.
  if (tersembunyi && (pathname === LOGIN_BAWAAN || pathname.startsWith(LOGIN_BAWAAN + "/"))) return { jenis: "tidak-ada" };

  if (pathname === "/") {
    if (adaToken) return { jenis: "alihkan", ke: "/dashboard" };
    return adaPetunjuk ? { jenis: "alihkan", ke: jalurMasuk } : takDikenal;
  }
  // Tujuan pengalihan dari kode klien (logout, sesi habis) dan dari backend (kegagalan SSO): meneruskan pesan galatnya.
  if (pathname === JALUR_KEMBALI) {
    return adaPetunjuk || !tersembunyi ? { jenis: "alihkan", ke: jalurMasuk + search } : takDikenal;
  }
  if (pathname === "/dashboard" || pathname.startsWith("/dashboard/")) {
    if (adaToken) return { jenis: "lanjut" };
    return adaPetunjuk ? { jenis: "alihkan", ke: jalurMasuk } : takDikenal;
  }
  return { jenis: "lanjut" };
}
