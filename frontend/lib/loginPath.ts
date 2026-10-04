// Alamat halaman login yang bukan /login. Berkas ini murni (tanpa Next/React) supaya aturannya bisa diuji. Nilai LOGIN_PATH sendiri
// hanya boleh dibaca (process.env.LOGIN_PATH) di middleware.ts dan next.config.ts: komponen klien boleh mengimpor konstanta di berkas
// ini (mis. JALUR_KEMBALI), tetapi tidak boleh membaca alamat loginnya, supaya alamat tidak tertanam di bundel JavaScript.
//
// Cara kerja:
//  - Halaman login (berkas app/(auth)/login) tetap ada di /login secara internal, tetapi dilayani di alamat LOGIN_PATH
//    (mis. /TDA5...): middleware menulis ulang alamat itu ke /login, dan permintaan langsung ke /login dijawab 404. Pemindai dan
//    penyerang yang menembak /login tidak menemukan apa-apa.
//  - Membuka alamat utama (/), /dashboard tanpa sesi, atau /kembali-masuk (tujuan logout, sesi habis, dan galat SSO) langsung
//    mengarahkan ke alamat login; pengguna tidak perlu mengetik alamatnya.
//  - Alamat bawaan ada di frontend/login-path.txt (satu sumber untuk lokal, Docker, dan deploy.sh); LOGIN_PATH di lingkungan
//    (deploy.env atau .env.local) menimpanya. LOGIN_PATH=/login menampilkan halaman login di /login seperti semula.
//
// Karena / mengarahkan ke alamat login, alamat itu tidak dirahasiakan dari siapa pun yang membuka situs; yang berubah hanya /login
// tidak lagi ada. Ini lapisan tambahan, bukan pengganti captcha, pembatasan percobaan login, dan kata sandi yang kuat.

export const LOGIN_BAWAAN = "/login";
export const JALUR_KEMBALI = "/kembali-masuk";
export const COOKIE_TOKEN = "pasti_access_token";

// Satu segmen alamat: huruf, angka, "_", "-", dan "=" (mis. hasil base64), 16-128 karakter. Titik dilarang: alamat bertitik dianggap
// berkas statis dan tidak melewati middleware. Nama rute aplikasi dilarang supaya tidak menimpanya.
const POLA_JALUR = /^\/[A-Za-z0-9_=-]{16,128}$/;
const NAMA_RUTE_DIPAKAI = new Set(["login", "dashboard", "kembali-masuk", "halaman-tidak-ada", "sso", "api"]);

export function jalurMasukSah(s: string | undefined | null): s is string {
  return typeof s === "string" && POLA_JALUR.test(s) && !NAMA_RUTE_DIPAKAI.has(s.slice(1).toLowerCase());
}

// Nilai LOGIN_PATH menjadi alamat yang dipakai. "/login" (atau kosong) berarti halaman login tetap di /login; nilai lain yang
// bentuknya salah ditolak dengan galat supaya salah ketik tidak diam-diam membuka /login.
export function jalurMasukDari(env: string | undefined | null): string {
  const nilai = (env ?? "").trim();
  if (nilai === "" || nilai === LOGIN_BAWAAN) return LOGIN_BAWAAN;
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

export interface Masukan {
  pathname: string;
  search: string; // termasuk "?" bila ada
  adaToken: boolean; // cookie sesi ada
  jalurMasuk: string; // hasil jalurMasukDari
}

export type Aksi =
  | { jenis: "lanjut" }
  | { jenis: "tulis-ulang"; ke: string } // dilayani dari rute internal; alamat di peramban tetap
  | { jenis: "alihkan"; ke: string }
  | { jenis: "tidak-ada" };

// Menentukan perlakuan tiap permintaan halaman.
export function tentukanAksi(m: Masukan): Aksi {
  const { pathname, search, adaToken, jalurMasuk } = m;
  const tersembunyi = jalurMasuk !== LOGIN_BAWAAN;

  if (pathname === jalurMasuk) {
    if (adaToken) return { jenis: "alihkan", ke: "/dashboard" };
    return tersembunyi ? { jenis: "tulis-ulang", ke: LOGIN_BAWAAN + search } : { jenis: "lanjut" };
  }
  // Rute internal tidak boleh dibuka langsung bila alamat aslinya diganti.
  if (tersembunyi && (pathname === LOGIN_BAWAAN || pathname.startsWith(LOGIN_BAWAAN + "/"))) return { jenis: "tidak-ada" };

  // Alamat utama: ke dashboard bila ada sesi, selain itu ke halaman login.
  if (pathname === "/") return { jenis: "alihkan", ke: adaToken ? "/dashboard" : jalurMasuk };
  // Tujuan pengalihan dari kode klien (logout, sesi habis) dan dari backend (kegagalan SSO): meneruskan pesan galatnya.
  if (pathname === JALUR_KEMBALI) return { jenis: "alihkan", ke: jalurMasuk + search };
  if (pathname === "/dashboard" || pathname.startsWith("/dashboard/")) {
    return adaToken ? { jenis: "lanjut" } : { jenis: "alihkan", ke: jalurMasuk };
  }
  return { jenis: "lanjut" };
}
