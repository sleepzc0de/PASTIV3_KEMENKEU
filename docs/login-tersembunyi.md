# Alamat halaman login yang tidak bisa ditebak

Halaman login tidak lagi di `/login`, tetapi di alamat acak, mis. `https://pasti.kemenkeu.go.id/3f9a1c07d2b84e65a0c1f7e93b2d4a68`.
Tujuannya mengurangi serangan otomatis (pemindai dan *credential stuffing* yang menembak `/login`, `/admin`, dan sejenisnya).

> Ini lapisan tambahan berbasis kerahasiaan alamat, **bukan pengganti** kontrol yang sudah ada: captcha, pembatasan percobaan
> login, kata sandi + pepper, dan SSO. Siapa pun yang tahu alamatnya tetap melihat halaman login yang sama.

## Cara kerja

| Permintaan | Hasil |
|---|---|
| `/<LOGIN_PATH>` | halaman login (dilayani dari rute internal `/login`); server memasang cookie petunjuk `pasti_jalur` (httpOnly, 1 tahun) |
| `/login` | **404** (rute internal tidak bisa dibuka langsung) |
| `/`, `/dashboard/...`, `/kembali-masuk` tanpa cookie petunjuk | **404**, tanpa pengalihan yang membocorkan alamat login |
| `/`, `/dashboard/...`, `/kembali-masuk` dengan cookie petunjuk | dialihkan ke `/<LOGIN_PATH>` (pesan galat dibawa serta) |
| `/`, `/dashboard/...` dengan sesi (`pasti_access_token`) | berjalan seperti biasa |

- Logout, sesi habis, dan kegagalan SSO tidak lagi menuju `/login`, tetapi ke **`/kembali-masuk`**; server yang meneruskan ke alamat login
  bila peramban pernah membuka halaman login. Backend mengarahkan galat SSO ke `FRONTEND_URL/kembali-masuk?error=sso_failed&reason=...`.
- Cookie sesi (`pasti_access_token`) **sengaja tidak dianggap petunjuk**: namanya terbaca di JavaScript klien sehingga bisa dipalsukan
  untuk memancing pengalihan. Cookie petunjuk dipasang server dan namanya hanya ada di kode server.
- Nilai `LOGIN_PATH` hanya ada di middleware (disisipkan saat build). Diperiksa setelah build: tidak ada di bundel JavaScript klien.
- Aturan lengkap dan pengujiannya: `frontend/lib/loginPath.ts` dan `frontend/lib/loginPath.test.mjs`.

## Konfigurasi

- **Server (deploy.sh):** `LOGIN_PATH` dibuat otomatis sekali di `deploy.env` (`/` + 32 karakter heksadesimal acak), ditanam ke frontend saat
  build lewat `docker-compose.yml` → `Dockerfile`, dan dicetak di akhir deploy (**Halaman login**). Deploy pertama sesudah fitur ini
  memperingatkan bahwa alamat lama tidak berlaku lagi. Cek kesehatan frontend memakai alamat ini.
- **Mengganti alamat:** hapus baris `LOGIN_PATH` di `deploy.env` lalu `./deploy.sh` (dibuat ulang), atau isi sendiri (`/` + 32-64 heksadesimal
  huruf kecil). Alamat lama langsung 404 dan semua pengguna perlu diberi tahu alamat barunya.
- **Pengembangan lokal:** tanpa `LOGIN_PATH` di `frontend/.env.local`, halaman login tetap di `/login`. Build produksi (`next build`) menolak
  `LOGIN_PATH` kosong agar `/login` tidak terbuka tanpa disengaja.
- Membuat nilai manual: `node -e "console.log('/'+require('crypto').randomBytes(16).toString('hex'))"`

## Yang perlu diketahui pengguna dan admin

- Pengguna **harus diberi tahu alamat login sekali** (simpan sebagai bookmark). Peramban yang belum pernah membuka alamat itu mendapat 404 di
  `/` dan `/dashboard`; ini disengaja.
- Pengguna yang sedang login saat fitur ini dipasang tidak punya cookie petunjuk: bila sesinya habis, `/kembali-masuk` menjawab 404
  sampai mereka membuka alamat login baru sekali.
- Hapus cookie peramban atau pindah peramban/perangkat berarti perlu membuka alamat login lagi.
- Alamat login bersifat rahasia semi-publik: jangan ditaruh di halaman terbuka. Bila bocor, ganti (lihat di atas).
