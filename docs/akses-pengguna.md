# Akses pengguna: tamu, pernyataan penggunaan aplikasi, superadmin, dan manajemen pengguna

Ringkasan aturan (kode: `backend/middleware/auth_middleware.go`, `backend/handlers/persetujuan_handler.go`, `backend/handlers/akses_pengguna.go`, `backend/handlers/superadmin_sinkron.go`,
`backend/persetujuan/`, frontend `components/akses/`). Peran dan cakupan data dibahas di [peran-data.md](peran-data.md).

## Login pertama: tamu dan pernyataan

Pengguna yang masuk pertama kali, lewat **SSO Kemenkeu** maupun **akun non-SSO**, berstatus **tamu** (belum punya peran). Tamu tidak dapat membuka fitur apa pun; tampilannya hanya:

1. **Modal pernyataan penggunaan aplikasi** (tidak dapat ditutup; tanpa sidebar atau menu) bila belum menyetujui. Pengguna membaca pernyataan, **mengetik `SAYA SETUJU`** (huruf besar/kecil
   dan spasi berlebih tidak dibedakan), dan **mencentang** kotak persetujuan; tombol *Setuju dan lanjutkan* baru aktif bila keduanya terpenuhi. Pilihan lain hanya *Keluar*.
   - **Akun non-SSO** sekaligus mengisi **nama lengkap, NIP (18 digit, atau 9 digit untuk NIP lama), dan email** (kedinasan atau pribadi). Akun SSO tidak mengisi apa pun: identitasnya dari SSO.
2. **Layar "Menunggu penetapan peran"** setelah menyetujui, sampai Superadmin atau Pengguna Barang memberi peran (tombol *Periksa lagi* dan *Keluar*).

Aturan di server (bukan hanya tampilan):

- Persetujuan disimpan di `users.persetujuan_at` dan `users.persetujuan_versi`. Versi pernyataan ada di `backend/persetujuan/persetujuan.go` (`Versi`); **menaikkan versi** (mis. setelah mengubah teksnya)
  membuat semua pengguna menyetujui ulang. Teks pernyataan juga di berkas itu (`Paragraf`) **dan perlu ditinjau/diganti oleh pemilik aplikasi**: yang ada sekarang draf.
- Semua endpoint selain `GET /auth/me`, `GET /auth/peran`, `GET|POST /auth/persetujuan` menjawab **403** untuk pengguna yang belum menyetujui (`code: persetujuan_diperlukan`) dan untuk tamu
  (`code: tamu`). Persetujuan didahulukan bila keduanya berlaku. Superadmin dikecualikan dari persetujuan (supaya tidak pernah terkunci) dan bukan tamu.
- Pengguna **yang sudah punya peran** tetapi belum menyetujui juga melihat modal lebih dulu, lalu langsung memakai perannya (tanpa layar menunggu).
- `POST /auth/persetujuan` (`{"frasa","setuju":true,"nama","nip","email"}`): frasa dan centang diperiksa ulang; untuk akun non-SSO isian divalidasi (galat per isian di `galat`), **email tidak boleh dipakai
  akun lain** (email maupun nama login, 409), dan **email atau NIP superadmin yang dicadangkan di `.env` tidak boleh dipakai akun non-SSO** (400). Mengulang persetujuan versi yang sama tidak mengubah apa pun.
- Akun yang dibuat lewat Manajemen Pengguna (`POST /users`) juga berstatus tamu dan harus menyetujui saat login pertamanya; role akun yang bisa dibuat hanya `user`.

## Superadmin: hanya dari `.env`

Satu-satunya cara menjadi superadmin adalah identitas **SSO Kemenkeu** yang emailnya (`SUPERADMIN_PROTECTED_EMAIL`) atau NIP-nya (`SUPERADMIN_PROTECTED_NIP`) tercantum di `.env` (boleh beberapa entri dipisah koma).
Tidak ada API yang mengubah role akun menjadi superadmin atau admin (`PUT /users/:id/role` dihapus; `POST/PUT /users` menolak role selain `user`).

- Saat SSO login, akun yang cocok dijadikan superadmin permanen (tidak dapat dihapus, dinonaktifkan, atau diubah).
- **Saat server dimulai** (`handlers.SinkronkanSuperadmin`): akun SSO yang cocok dinaikkan/diaktifkan, dan akun lain yang masih berrole superadmin atau bertanda protected (sisa data lama, `.env` yang diubah,
  atau akun non-SSO) **diturunkan** menjadi pengguna biasa. Bila `.env` tidak berisi identitas superadmin sama sekali, tidak ada yang diturunkan (konfigurasi yang terlewat tidak boleh mengunci semua superadmin) dan
  log memuat peringatan.
- Akun **non-SSO tidak pernah** menjadi superadmin, walau emailnya sama dengan yang di `.env` (emailnya isian pengguna, bukan data terverifikasi).
- Role dibaca dari database pada setiap permintaan, jadi penurunan langsung berlaku walau token lama masih bertuliskan superadmin.

## Manajemen pengguna

| Peran | Daftar dan detail pengguna | Mengubah (tambah, ubah, nonaktifkan, hapus, beri/cabut peran) |
|---|---|---|
| Super Admin | Semua pengguna | Ya |
| Pengguna Barang | Semua pengguna **kecuali superadmin** | Ya, kecuali terhadap superadmin |
| UE1 | Pengguna yang kode satker SSO-nya berawalan **5 digit** yang sama | Tidak (hanya lihat) |
| Kanwil | Pengguna yang kode satker SSO-nya berawalan **9 digit** yang sama | Tidak (hanya lihat) |
| Satker | Pengguna yang **karakter ke-10 sampai ke-15** kode satker SSO-nya sama (6 digit) | Tidak (hanya lihat) |
| Tamu | Tidak ada | Tidak |

- Superadmin tidak muncul di daftar maupun detail bagi siapa pun selain superadmin, dan setiap aksi terhadapnya oleh Pengguna Barang dijawab **404** (seolah tidak ada; keberadaannya tidak bocor). Pengguna di luar
  cakupan bagi UE1/Kanwil/Satker juga dijawab 404.
- Kode satker pengguna adalah `employees.kode_satker` dari SSO. **Akun non-SSO tidak punya kode satker**, jadi tidak tampak bagi UE1/Kanwil/Satker (hanya bagi superadmin dan Pengguna Barang).
- Daftar memuat: **kode satker SSO**, **nama satker pada data Digitalisasi Aset** (dicari lewat `SUBSTRING(Kode_Satker, 10, 6)` pada `DIGITALISASI_SATKER`, sama dengan kunci ke `kd_satker_str` data Pengadaan),
  peran data yang dipegang, penanda **Tamu**, dan penanda **Belum setuju** (belum menyetujui pernyataan).
- Superadmin yang bertindak sebagai peran data (pemilih peran) diperlakukan sebagai peran itu: hanya melihat, bukan mengelola.
- API: `GET /users`, `GET /users/:id`, `GET /users/:id/peran` (boleh melihat); `POST /users`, `PUT /users/:id`, `PUT /users/:id/deactivate`, `DELETE /users/:id`, `POST /users/:id/peran`,
  `DELETE /users/:id/peran/:peranId` (boleh mengubah).

## Migrasi 055 dan penerapan

`055_persetujuan_dan_akses_tamu.sql` menambah `users.persetujuan_at`, `users.persetujuan_versi`, `users.nip`, lalu mengubah akun `admin` lama (bukan superadmin permanen) menjadi pengguna biasa dengan peran
**Pengguna Barang**. Aman dijalankan ulang. Dampak saat diterapkan:

- **Semua pengguna yang belum punya peran data menjadi tamu** (sebelumnya mereka melihat semua data) dan harus menyetujui pernyataan; Superadmin atau Pengguna Barang perlu memberi mereka peran.
  `PERAN_DATA_WAJIB` sudah tidak ada: pembatasan selalu berlaku.
- Pengguna yang sudah punya peran melihat modal pernyataan sekali, lalu bekerja seperti biasa.
- `SUPERADMIN_PROTECTED_EMAIL`/`SUPERADMIN_PROTECTED_NIP` di `.env` produksi harus berisi superadmin yang benar **sebelum** deploy: akun lain yang masih berrole superadmin akan diturunkan saat server dimulai.
- Data seed pengembangan (`cmd/seed`) memberi akun dummy persetujuan dan peran Pengguna Barang agar bisa dipakai langsung.

## Belum diverifikasi

- Format klaim `kode_satker` SSO yang asli (kolom `employees.kode_satker`): pembatasan daftar pengguna UE1/Kanwil/Satker bergantung padanya (kode satker lengkap, minimal 15 karakter).
- Teks pernyataan adalah draf. Ubah di `backend/persetujuan/persetujuan.go` lalu naikkan `Versi`.
- Alur login SSO dan non-SSO asli (nama, NIP, email non-SSO) diuji lewat router dengan SQL Server dan tampilan dengan data tiruan, bukan dengan SSO Kemenkeu sungguhan.

## Pengujian

`go test ./middleware ./persetujuan ./peran ./utils ./handlers` (tanpa database) dan, dengan `PASTI_UJI_MSSQL_DSN`, `go test ./routes -run 'AksesPengguna|PersetujuanDanTamu|Migrasi055|SinkronkanSuperadmin|PeranData'`.
Frontend: `node --test lib/persetujuan.test.mjs lib/peran.test.mjs`.
