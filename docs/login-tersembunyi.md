# Alamat halaman login bukan /login

Halaman login tidak di `/login`, tetapi di alamat khusus yang ditetapkan di **`frontend/login-path.txt`** (bawaan: `/TDA5MW5wNHN0MXYzMjAyRw==`).
Tujuannya mengurangi serangan otomatis yang menembak alamat umum (`/login`, `/admin`, dan sejenisnya): `/login` menjawab 404.

> Ini lapisan tambahan, **bukan pengganti** kontrol yang sudah ada: captcha, pembatasan percobaan login, kata sandi + pepper, dan SSO.
> Karena alamat utama (`/`) langsung mengarahkan ke halaman login, alamat itu tidak dirahasiakan dari siapa pun yang membuka situs;
> yang dicegah hanyalah penebakan alamat umum.

## Cara kerja

| Permintaan | Hasil |
|---|---|
| `/` (kunjungan pertama, tanpa sesi) | **dialihkan** ke `/<alamat login>`; pengguna tidak perlu mengetik alamatnya |
| `/` dengan sesi | dialihkan ke `/dashboard` |
| `/<alamat login>` | halaman login (dilayani dari rute internal `/login`, alamat di peramban tetap) |
| `/login` | **404** (rute internal tidak bisa dibuka langsung) |
| `/dashboard/...` tanpa sesi | dialihkan ke `/<alamat login>` |
| `/kembali-masuk` | dialihkan ke `/<alamat login>` dengan pesan galat (logout, sesi habis, galat SSO) |

- Logout, sesi habis, dan kegagalan SSO tidak menuju `/login`, tetapi ke **`/kembali-masuk`**; server meneruskannya ke alamat login. Backend
  mengarahkan galat SSO ke `FRONTEND_URL/kembali-masuk?error=sso_failed&reason=...`.
- Alamat login hanya ada di middleware (disisipkan saat build), tidak di bundel JavaScript klien (diperiksa setelah build). Alamat
  yang "="-nya tersalin sebagai `%3D` tetap dikenali.
- Aturan lengkap dan pengujiannya: `frontend/lib/loginPath.ts` dan `frontend/lib/loginPath.test.mjs`.

## Konfigurasi: satu sumber, tanpa langkah tambahan

**Ubah alamatnya di `frontend/login-path.txt`** (satu baris), lalu commit. Berkas itu dibaca oleh:

- `next.config.ts`: `npm run dev`, `npm run build`, dan build Docker di server memakai alamat yang sama, tanpa mengisi env apa pun;
- `deploy.sh`: untuk cek kesehatan dan ringkasan akhir deploy (**Halaman login**), setelah kode ditarik.

Bentuk yang diterima: satu segmen 16-128 karakter berisi huruf, angka, `_`, `-`, dan `=` (mis. hasil base64), tanpa titik dan bukan nama
rute aplikasi (`login`, `dashboard`, `kembali-masuk`, `halaman-tidak-ada`, `sso`, `api`). Tes memeriksa isi berkas ini.

Penimpaan (opsional):

- **`LOGIN_PATH` di `deploy.env`** (server) atau di `frontend/.env.local` (lokal) menimpa bawaan repo. `deploy.sh` memperingatkan bila
  nilainya berbeda dari bawaan repo.
- `LOGIN_PATH=/login` mengembalikan halaman login ke `/login` seperti semula.
- Versi `deploy.sh` yang lebih lama membuat alamat acak sendiri di `deploy.env` (dengan komentar "Dibuat otomatis oleh deploy.sh"). Nilai
  itu dibuang otomatis pada deploy berikutnya supaya bawaan repo berlaku; alamat yang Anda tulis sendiri tidak disentuh.

Catatan: alamat di repo berarti siapa pun yang bisa membaca repo (dan riwayatnya) tahu alamatnya; jaga repo tetap privat. Alamat yang
berupa kata/frasa terkait aplikasi (walau di-base64 atau leetspeak) mudah ditebak; nilai acak lebih kuat:
`node -e "console.log('/'+require('crypto').randomBytes(16).toString('hex'))"`

## Yang perlu diketahui

- Mengganti alamat memutus alamat lama (langsung 404): bookmark pengguna ke alamat lama perlu diperbarui, atau cukup buka alamat utama situs.
- **Deploy pertama sesudah fitur ini** bisa berjalan dengan `deploy.sh` lama: skrip menarik kode baru di tengah jalan, tetapi yang
  berjalan sudah terbaca ke memori. Gejalanya alamat login yang tercetak bukan yang di `frontend/login-path.txt` (ringkasan akhir masih
  berformat lama). Jalankan `./deploy.sh` sekali lagi; skrip yang sekarang ada di disk sudah versi baru. Untuk perubahan berikutnya,
  `deploy.sh` mendeteksi dirinya berubah oleh `git pull` dan berhenti sebelum membangun apa pun (container lama tidak tersentuh)
  dengan pesan untuk menjalankannya ulang.
