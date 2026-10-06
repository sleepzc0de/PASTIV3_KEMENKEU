# Peran data dan cakupan data per satker

Pengguna memegang satu atau lebih **peran data** dan bertindak sebagai **satu peran dalam satu waktu** (peran aktif). Peran aktif menentukan data yang boleh dilihat.
Migrasi: `053_create_user_roles.sql`. Kode: `backend/peran/` (logika dan penyimpanan), `backend/handlers/peran_handler.go`, `backend/middleware/auth_middleware.go`.

## Lima peran

| Peran | Sumber | Data yang dilihat | Hak administrasi |
|---|---|---|---|
| **Super Admin** | `users.role = superadmin` | Seluruh data | Penuh |
| **Pengguna Barang** | tabel `user_roles` | Seluruh data (hanya baca) | Tidak |
| **UE1** | `user_roles`, kode 5 digit | Satker yang kode satkernya diawali kode UE1 | Tidak |
| **Kanwil** | `user_roles`, kode 9 digit | Satker yang kode satkernya diawali kode Kanwil | Tidak |
| **Satker** | `user_roles`, kode 6 digit | Satu satker | Tidak |

Role akun lama `admin` tetap ada (akun yang mengelola pengguna, sinkronisasi, dan penarikan) dan melihat seluruh data seperti Super Admin; perannya bukan salah satu dari lima di atas.
Fitur **SAPA** tidak punya peran sendiri lagi: peran Satker, Kanwil, UE1, dan Pengguna Barang di SAPA adalah peran data yang sedang aktif di sini (tabel lama `sapa_peran` tidak dibaca lagi; migrasi 054 menyalinnya ke `user_roles`). Lihat [sapa.md](sapa.md#peran-sapa-dan-hak-akses).

## Kode: satu kode satker lengkap memuat semua tingkat

```
015040199119091000KP
└─┬─┘                    karakter 1-5   = UE1     (01504)
└───┬───┘                karakter 1-9   = Kanwil  (015040199 = UE1 + 4 digit)
         └──┬───┘        karakter 10-15 = satker  (119091)
```

## Cara kerja

- **Peran aktif disimpan di database** (`user_roles.aktif`, paling banyak satu per pengguna) dan dibaca middleware pada **setiap permintaan**, bersama pemeriksaan akun aktif.
  Berpindah peran (`POST /auth/peran/aktif`) atau dicabutnya peran oleh admin berlaku di permintaan berikutnya tanpa token baru.
- Tanpa pilihan: admin/superadmin memakai peran akunnya; pengguna biasa memakai peran data pertamanya.
- **Admin/superadmin yang bertindak sebagai peran data kehilangan hak administrasi selama itu** (`role` di konteks turun menjadi `user`), jadi pratinjau sebagai Satker benar-benar
  memperlihatkan apa yang dilihat Satker. Kembali ke peran bawaan lewat pemilih peran.
- **Pengguna tanpa peran** melihat seluruh data seperti sebelum peran ada. `PERAN_DATA_WAJIB=true` mengubahnya menjadi tanpa data; nyalakan setelah semua pengguna diberi peran.
  Penerapan bertahap: pembatasan berlaku per pengguna begitu ia diberi peran.
- Tabel `user_roles` yang belum ada (migrasi belum dijalankan) diperlakukan seperti belum ada peran, bukan galat.

## Yang dibatasi

Cakupan dibaca dari `context` (`peran.CakupanDari`) oleh semua pembaca data berikut, yang memakai sumber baris terbatas (`dgSumber`/`dgSumberTabel`: subquery dengan kondisi kode satker):

| Area | Pembatasan |
|---|---|
| Digitalisasi Aset: ringkasan, peta, daftar, rincian, ekspor, pilihan filter, status sinkronisasi (jumlah baris) | Hanya baris satker dalam cakupan; rincian baris di luar cakupan dijawab 404 |
| Dashboard > Aset | Mengikuti ringkasan di atas |
| Dashboard > Satker (`/satker/keterhubungan`) | Aset dalam cakupan; pengadaan hanya untuk satker yang dikenal di data aset dalam cakupan (Satker: satkernya sendiri) |
| Referensi UE1: daftar kode yang belum terdaftar | Menurut cakupan |
| **Pengadaan Terpadu** (`/inaproc/*`: analitik, data, ekspor, penarikan) | **Ditolak (403) bagi peran UE1/Kanwil/Satker.** Data Inaproc hanya punya kode satker 6 digit dan sebagian tabelnya tidak punya `kd_satker_str`, jadi belum bisa dibatasi per UE1/Kanwil. Menu Pengadaan dan tab Pengadaan di Dashboard disembunyikan bagi peran ini. |
| HRIS2, pengguna, pengaturan | Hanya admin/superadmin (tidak berubah) |

Kode cakupan dibentuk dari angka murni yang diperiksa ulang sebelum ditulis ke SQL (`Cakupan.KondisiSQL`); kode yang tidak sah menutup data (`1 = 0`), bukan membukanya.

## Mengelola peran

Admin: **Manajemen Pengguna > ikon perisai (Peran data)** pada baris pengguna. Daftar peran yang dimiliki, tombol cabut, dan **saran dari kode satker di data SSO** pengguna itu
(UE1, Kanwil, Satker diturunkan dari kode satker; satu klik untuk menambahkan). Format klaim `kode_satker` dari SSO **belum diverifikasi**: lihat [sso-data.md](sso-data.md).

API (admin): `GET/POST /users/:id/peran`, `DELETE /users/:id/peran/:peranId`. Pengguna: `GET /auth/peran`, `POST /auth/peran/aktif` (`{"id": 12}` atau `{"id": null}` untuk peran bawaan).
`GET /auth/me` memuat `role` (hak administrasi saat ini), `akun_role`, dan `peran` (peran berlaku, cakupan, daftar peran).

Pemilih peran ada di menu pengguna (kanan atas) bila pengguna punya lebih dari satu pilihan; setelah berpindah halaman dimuat ulang supaya menu dan data mengikuti peran baru.

## Pengujian

- `go test ./peran` (logika murni: validasi kode, kondisi SQL, pemilihan peran berlaku, saran dari kode satker).
- `go test ./routes -run PeranData` (butuh `PASTI_UJI_MSSQL_DSN`): lewat router asli dengan JWT dan SQL Server sungguhan; memeriksa pembatasan semua pembaca data untuk UE1, Kanwil, dan Satker,
  berpindah peran, admin yang bertindak sebagai peran data, 403 pada Pengadaan, `PERAN_DATA_WAJIB`, validasi, dan hak mengelola peran. Data uji memakai kode UE1 09971/09972.
- `node --test lib/peran.test.mjs`.

## Belum dikerjakan

- Pembatasan **data Pengadaan lengkap** per satker (butuh pemetaan `kd_satker_str` ke UE1/Kanwil di semua tabel Inaproc dan penanganan tabel tanpa kolom itu).
- Pemberian peran otomatis saat login SSO dari `kode_satker` (sengaja belum: format klaimnya belum terverifikasi; sementara lewat saran satu klik).
