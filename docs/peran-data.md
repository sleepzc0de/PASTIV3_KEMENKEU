# Peran, tamu, dan cakupan data per satker

Pengguna memegang satu atau lebih **peran data** dan bertindak sebagai **satu peran dalam satu waktu** (peran aktif). Peran aktif menentukan fitur dan data yang boleh dipakai.
**Pengguna tanpa peran adalah tamu**: tidak dapat membuka fitur apa pun. Alur login pertama, pernyataan penggunaan aplikasi, dan manajemen pengguna ada di [akses-pengguna.md](akses-pengguna.md).
Migrasi: `053_create_user_roles.sql` (peran), `055_persetujuan_dan_akses_tamu.sql` (persetujuan dan penyederhanaan role akun). Kode: `backend/peran/` (logika dan penyimpanan),
`backend/handlers/peran_handler.go`, `backend/middleware/auth_middleware.go`, `backend/middleware/role_middleware.go`.

## Peran dan hak

| Peran | Sumber | Data yang dilihat | Fitur khusus |
|---|---|---|---|
| **Super Admin** | `users.role = superadmin`, **hanya ditetapkan di `.env`** (lihat [akses-pengguna.md](akses-pengguna.md#superadmin-hanya-dari-env)) | Seluruh data | Semua: penarikan Inaproc, sinkronisasi SLDK, template dan pengaturan SAPA, referensi UE1, HRIS2, dan mengelola semua pengguna |
| **Pengguna Barang** | tabel `user_roles` | Seluruh data | Mengelola pengguna **kecuali superadmin** (tidak melihat dan tidak dapat menyentuhnya); tidak memakai fitur khusus superadmin |
| **UE1** | `user_roles`, kode 5 digit | Satker yang kode satkernya diawali kode UE1 | Melihat pengguna dengan kode satker SSO berawalan 5 digit yang sama (hanya lihat) |
| **Kanwil** | `user_roles`, kode 9 digit | Satker yang kode satkernya diawali kode Kanwil | Melihat pengguna dengan 9 digit awal kode satker SSO yang sama (hanya lihat) |
| **Satker** | `user_roles`, kode 6 digit | Satu satker | Melihat pengguna dengan karakter ke-10 sampai ke-15 kode satker SSO yang sama (hanya lihat) |
| **Tamu** | tanpa peran | Tidak ada | Tidak ada: hanya membaca dan menyetujui pernyataan, lalu menunggu peran |

Role akun (`users.role`) kini hanya `user` atau `superadmin`. Role `admin` yang lama **ditiadakan**: migrasi 055 menjadikan akun admin lama pengguna biasa dengan peran data
**Pengguna Barang** (akun superadmin permanen tidak berubah). Role `admin` yang tersisa di database (mis. migrasi belum dijalankan) diperlakukan seperti pengguna biasa tanpa hak istimewa.

Fitur **SAPA** tidak punya peran sendiri: peran Satker, Kanwil, UE1, dan Pengguna Barang di SAPA adalah peran data yang sedang aktif di sini (tabel lama `sapa_peran` tidak dibaca lagi;
migrasi 054 menyalinnya ke `user_roles`). Lihat [sapa.md](sapa.md#peran-sapa-dan-hak-akses).

## Kode: satu kode satker lengkap memuat semua tingkat

```
015040199119091000KP
└─┬─┘                    karakter 1-5   = UE1     (01504)
└───┬───┘                karakter 1-9   = Kanwil  (015040199 = UE1 + 4 digit)
         └──┬───┘        karakter 10-15 = satker  (119091)
```

## Cara kerja

- **Keadaan akun dan peran dibaca dari database pada setiap permintaan** (`middleware.AuthRequired`): akun aktif, `users.role`, persetujuan pernyataan, dan peran aktif (`user_roles.aktif`, paling banyak
  satu per pengguna). Role **tidak** diambil dari token, jadi hak superadmin yang dicabut, peran yang dicabut, atau pemindahan peran berlaku di permintaan berikutnya tanpa token baru.
- Tanpa pilihan: superadmin memakai peran akunnya; pengguna biasa memakai peran data pertamanya.
- **Superadmin yang bertindak sebagai peran data kehilangan hak superadmin selama itu** (`role` di konteks turun menjadi `user`), jadi pratinjau sebagai Satker benar-benar
  memperlihatkan apa yang dilihat Satker. Kembali ke peran bawaan lewat pemilih peran.
- **Tamu dan pengguna yang belum menyetujui pernyataan ditolak (403) di semua endpoint** kecuali `/auth/me`, `/auth/peran`, dan `/auth/persetujuan` (`AuthTamuBoleh`). Kode galatnya
  `persetujuan_diperlukan` atau `tamu`.
- Tabel `user_roles` yang belum ada (migrasi belum dijalankan) diperlakukan seperti belum ada peran (tamu), bukan galat.

## Yang dibatasi

Cakupan dibaca dari `context` (`peran.CakupanDari`) oleh semua pembaca data berikut, yang memakai sumber baris terbatas (`dgSumber`/`dgSumberTabel`: subquery dengan kondisi kode satker):

| Area | Pembatasan |
|---|---|
| Digitalisasi Aset: ringkasan, peta, daftar, rincian, ekspor, pilihan filter | Hanya baris satker dalam cakupan; rincian baris di luar cakupan dijawab 404 |
| Digitalisasi Aset: tab Sinkronisasi (`GET /digitalisasi/sinkronisasi`: status, jumlah baris, riwayat) | **Ditolak (403) bagi peran UE1/Kanwil/Satker** dan tabnya disembunyikan; hanya superadmin dan Pengguna Barang. Menjalankan/membatalkan: superadmin. |
| Dashboard > Aset | Mengikuti ringkasan di atas |
| Dashboard > Satker (`/satker/keterhubungan`) | Aset dalam cakupan; pengadaan hanya untuk satker yang dikenal di data aset dalam cakupan (Satker: satkernya sendiri) |
| Referensi UE1: daftar kode yang belum terdaftar | Menurut cakupan |
| **Pengadaan Terpadu**: data, ekspor, dasbor (`/inaproc/data`, `/inaproc/ekspor`, `/inaproc/analitik`, `/inaproc/dataset`) | Dibatasi per satker lewat `kd_satker_str` (kode 6 digit); dataset yang tidak punya kode satker tertutup (403). Lihat [pengadaan-terpadu.md](pengadaan-terpadu.md#pembatasan-per-satker). |
| Pengadaan Terpadu: keadaan penarikan (`/inaproc/penarikan`, `/aktif`, `/riwayat`) dan operasi superadmin | **Ditolak (403) bagi peran UE1/Kanwil/Satker**: memuat keadaan seluruh data dan riwayat penarikan. |
| HRIS2, referensi UE1 (ubah), sinkronisasi SLDK, penarikan Inaproc (mulai/batal/atur), template dan pengaturan SAPA | Hanya superadmin |
| Daftar dan pengelolaan pengguna | Lihat [akses-pengguna.md](akses-pengguna.md#manajemen-pengguna) |

Kode cakupan dibentuk dari angka murni yang diperiksa ulang sebelum ditulis ke SQL (`Cakupan.KondisiSQL`); kode yang tidak sah menutup data (`1 = 0`), bukan membukanya.

## Mengelola peran

**Manajemen Pengguna > ikon perisai (Peran data)** pada baris pengguna (superadmin dan Pengguna Barang). Daftar peran yang dimiliki, tombol cabut, dan **saran dari kode satker di data SSO**
pengguna itu (UE1, Kanwil, Satker diturunkan dari kode satker; satu klik untuk menambahkan). UE1, Kanwil, dan Satker membuka modal yang sama dalam mode **hanya lihat**. Format klaim
`kode_satker` dari SSO **belum diverifikasi**: lihat [sso-data.md](sso-data.md).

API: `GET /users/:id/peran` (semua yang boleh melihat pengguna itu), `POST /users/:id/peran` dan `DELETE /users/:id/peran/:peranId` (superadmin dan Pengguna Barang). Pengguna:
`GET /auth/peran`, `POST /auth/peran/aktif` (`{"id": 12}` atau `{"id": null}` untuk peran bawaan). `GET /auth/me` memuat `role` (hak superadmin saat ini), `akun_role`, `tamu`, `persetujuan`,
dan `peran` (peran berlaku, cakupan, daftar peran).

Pemilih peran ada di menu pengguna (kanan atas) bila pengguna punya lebih dari satu pilihan; setelah berpindah halaman dimuat ulang supaya menu dan data mengikuti peran baru.

## Pengujian

- `go test ./peran` (logika murni: validasi kode, kondisi SQL, pemilihan peran berlaku termasuk tamu dan role admin lama, saran dari kode satker).
- `go test ./routes -run 'PeranData|AksesPengguna|PersetujuanDanTamu|Migrasi055|SinkronkanSuperadmin'` (butuh `PASTI_UJI_MSSQL_DSN`): lewat router asli dengan JWT dan SQL Server sungguhan; memeriksa
  pembatasan semua pembaca data untuk UE1, Kanwil, dan Satker, berpindah peran, superadmin yang bertindak sebagai peran data, tamu, persetujuan, dan hak mengelola pengguna.
  Data uji memakai kode UE1 09971/09972.
- `go test ./middleware` (gerbang tamu/persetujuan tanpa database), `node --test lib/peran.test.mjs lib/persetujuan.test.mjs`.

## Belum dikerjakan

- Pembatasan Pengadaan untuk dataset yang tidak punya kode satker (program master, E-Katalog V6 transaksi per produk, rujukan katalog, e-kontrak non-tender) dan untuk satker yang tidak ada di data aset (bagi UE1/Kanwil); lihat [pengadaan-terpadu.md](pengadaan-terpadu.md#pembatasan-per-satker).
- Pemberian peran otomatis saat login SSO dari `kode_satker` (sengaja belum: format klaimnya belum terverifikasi; sementara lewat saran satu klik).
