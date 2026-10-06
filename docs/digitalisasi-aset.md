# Digitalisasi Aset

Halaman **Digitalisasi Aset** (`/dashboard/digitalisasi`) menampilkan analitik, peta, dan daftar aset KL 015 yang **disalin dari
SLDK ke database PASTI**. Halaman tidak membaca SLDK langsung; SLDK hanya dibaca saat admin menjalankan sinkronisasi.

| Tab | Isi |
|---|---|
| Ringkasan | Angka utama (satker, tanah, gedung, hunian, nilai, kendaraan dinas), grafik per jenis, kondisi, UE1, provinsi, status hukum, asuransi, kamar tidur, status penghuni, dan tabel **kelengkapan data** (aset tanpa koordinat/foto/kondisi). Tiap grafik punya tombol *Tabel*. |
| Peta | Peta Leaflet sebaran aset. Titik berklaster (donat berwarna menunjukkan komposisi jenis), filter jenis aset dan UE1, klik titik untuk kartu ringkas lalu *Detail lengkap*. |
| Data | Daftar tiap dataset dengan pencarian, filter UE1/provinsi/kondisi, filter "belum punya koordinat", halaman, dan detail per baris (tombol *Lihat di peta*). |
| Sinkronisasi | Status tiap dataset, tombol sinkronisasi (admin), kemajuan, pembatalan, dan riwayat. |

Semua pengguna login boleh melihat keempat tab; menjalankan dan membatalkan sinkronisasi khusus admin/superadmin.

## Tabel dan query

Tujuh dataset, masing-masing satu query SLDK dan satu tabel di database PASTI:

| Dataset (key) | Tabel | Query |
|---|---|---|
| `satker` | `DIGITALISASI_SATKER` | `backend/digitalisasi/queries/satker.sql` |
| `tanah` | `DIGITALISASI_TANAH` | `tanah.sql` |
| `gedung_kantor_utama` | `DIGITALISASI_GEDUNG_KANTOR_UTAMA` | `gedung_kantor_utama.sql` |
| `gedung_lainnya` | `DIGITALISASI_GEDUNG_LAINNYA` | `gedung_lainnya.sql` |
| `rusunara` | `DIGITALISASI_RUSUNARA` | `rusunara.sql` |
| `rumah_negara` | `DIGITALISASI_RUMAH_NEGARA` | `rumah_negara.sql` |
| `mess_rumah_negara` | `DIGITALISASI_MESS_RUMAH_NEGARA` | `mess_rumah_negara.sql` |

- Query adalah query asli dari pengelola data, dibaca apa adanya (tertanam di biner backend). **Satu-satunya tambahan** adalah dua kolom
  koordinat (`A.gps_latitude`, `A.gps_longitude` dari `SIMAN2_M_ASET`) di enam dataset aset supaya peta bisa dibuat; barisnya diberi
  komentar `-- [PASTI]`. Dataset `satker` tidak diubah.
- Kolom tabel sama dengan alias kolom query (mis. `Kode_Satker`, `Luas_Tanah`), ditambah `id` (kunci), `Latitude`/`Longitude`
  (hasil `GPS_Latitude`/`GPS_Longitude`), `id_sinkron` (baris riwayat yang mengisinya), dan `synced_at`.
- Definisi kolom, tipe, dan peran kolom ada di satu tempat: `backend/digitalisasi/datasets.go`. Migrasi
  `backend/migrations/020_create_digitalisasi.sql` **dihasilkan** dari file itu (jangan disunting manual).

### Mengubah query atau kolom

1. Ubah file `.sql` dan/atau `datasets.go` (kolom tujuan harus punya alias `AS <nama>` di query).
2. Jalankan `go test ./digitalisasi -run TestMigrationFile -update` di folder `backend` untuk menghasilkan ulang migrasi.
   Untuk tabel yang **sudah ada** di database, buat migrasi baru (`ALTER TABLE`) alih-alih menyunting 020: file yang sudah
   diterapkan tidak dijalankan lagi.
3. Jalankan `go test ./...`. Tes menjaga agar query, definisi kolom, migrasi, dan kolom yang dipakai frontend tetap sejalan.

## Cara kerja sinkronisasi

1. Admin memilih dataset (atau "Sinkronkan semua") dan mengonfirmasi. Permintaan langsung dijawab; pekerjaan berjalan di server,
   jadi halaman boleh ditinggalkan. Hanya **satu antrean** berjalan pada satu waktu; dataset dijalankan berurutan.
2. Per dataset: query dijalankan di SLDK pada satu koneksi (query memakai tabel `#sementara`), seluruh hasil dibaca ke memori
   (maks. 500.000 baris, **tanpa batas waktu**: berjalan sampai selesai atau sampai admin menekan Batalkan), lalu **dalam satu
   transaksi singkat** isi tabel tujuan dikosongkan dan diisi ulang. Pembaca tidak terkunci selama query SLDK berjalan; bila ada
   yang gagal, tabel tidak berubah.
3. Perlindungan: hasil **0 baris tidak menimpa** tabel yang sudah berisi; kolom hasil yang hilang dari query menghentikan
   sinkronisasi dengan pesan yang jelas; nilai teks yang melebihi panjang kolom dipotong dan dicatat; koordinat di luar rentang bumi,
   tidak berpasangan, atau (0, 0) dibuang (NULL).
4. `DIGITALISASI_SATKER`: kolom `Status_Gedung_Kantor` dan `Foto` selalu NULL dari query, jadi dianggap **diisi manual**. Nilai yang ada
   dipertahankan per `Kode_Satker` bila sumber mengirim NULL.
5. Kemajuan dan hasil dicatat di `digitalisasi_sync_log` (status `antri` → `berjalan` → `sukses`/`gagal`/`dibatalkan`). Saat server
   dimulai ulang, baris yang masih `antri`/`berjalan` ditandai `gagal` karena prosesnya sudah hilang.

> Tiap query membaca tabel aset SLDK (±206 GB). Jalankan di luar jam kerja dan jangan bersamaan dengan
> `pasti-sldk-sync ringkasan` (lihat [sldk-sync.md](sldk-sync.md)).

### Sinkronisasi otomatis mingguan

Server menyinkronkan dataset sendiri tanpa perlu ditekan manual. Penjadwalnya berjalan di dalam proses backend (tanpa cron di luar) dan
membaca riwayat `digitalisasi_sync_log`, jadi keadaannya bertahan saat server dimulai ulang dan sinkronisasi **manual ikut dihitung**.

- Sebuah dataset **jatuh tempo** bila sinkronisasi suksesnya yang terakhir sudah lebih tua dari 7 hari (atau belum pernah ada). Yang
  jatuh tempo diantrekan bersama, berurutan, sebagai satu antrean (dicatat `dijalankan_oleh = otomatis (mingguan)`).
- Hanya **dimulai** di jendela jam malam, bawaan **01.00-05.00 WIB**, supaya query berat ke SLDK tidak berjalan di jam kerja. Yang sudah
  berjalan boleh melewati jendela itu sampai selesai. Penjadwal memeriksa tiap 15 menit.
- Percobaan yang **gagal** (atau terhenti karena server dimulai ulang) tidak diulang langsung: dataset yang sama baru dicoba lagi
  paling cepat 20 jam kemudian, yaitu pada malam berikutnya, supaya SLDK tidak dibebani berulang saat ada masalah.
- Tidak pernah bersamaan dengan sinkronisasi lain: bila admin sedang menyinkronkan manual, penjadwal menunggu putaran berikutnya.
- **Tanpa batas waktu**, baik manual maupun otomatis; sinkronisasi yang macet dihentikan lewat tombol *Batalkan*.
- Halaman Sinkronisasi menampilkan jadwal dan perkiraan jalan berikutnya. Hanya berjalan bila `SLDK_DB_*` terisi (koneksi SLDK ada).
- Dataset yang belum pernah disinkronkan ikut diantrekan pada malam pertama setelah deploy; sebelum itu, coba satu dataset kecil dulu
  secara manual (lihat *Menjalankan pertama kali*), atau matikan otomatis dulu.

Pengaturan opsional di `backend/.env` (tanpa mengisinya: aktif, 7 hari, 01-05 WIB):

| Variabel | Arti | Bawaan |
|---|---|---|
| `DIGITALISASI_AUTO_SYNC` | `false` untuk mematikan sinkronisasi otomatis | `true` |
| `DIGITALISASI_AUTO_INTERVAL_HARI` | jarak minimal antar sinkronisasi sukses sebuah dataset (1-365) | `7` |
| `DIGITALISASI_AUTO_JAM_MULAI` / `DIGITALISASI_AUTO_JAM_AKHIR` | jendela jam (WIB, 0-23) sinkronisasi boleh dimulai; sama = sepanjang hari | `1` / `5` |

Nilai di luar rentang diganti bawaannya. Restart backend setelah mengubahnya.

## Peta

- Titik digambar hanya bila koordinat berada di kotak Indonesia (lintang −11,5 s.d. 6,5; bujur 94,5 s.d. 141,5). Koordinat di luar
  kotak itu (biasanya tanda minus hilang atau lintang/bujur tertukar) dihitung di *Kelengkapan data* dan tidak digambar.
- Maksimum 100.000 titik per dataset; bila lebih, peta memberi peringatan dan UE1 bisa dipakai untuk mempersempit.
- Peta dasar memakai OpenStreetMap lewat browser pengguna. Bila jaringan memblokirnya, titik tetap tampil dan halaman memberi
  catatan. Server lain dapat dipasang lewat variabel build `NEXT_PUBLIC_MAP_TILE_URL` (pola `https://host/{z}/{x}/{y}.png`) dan
  `NEXT_PUBLIC_MAP_TILE_ATTRIBUTION` di `deploy.env`, lalu bangun ulang frontend.
- Warna jenis aset tetap per jenis (tanah biru, kantor utama jingga, gedung lainnya aqua, rusunara merah muda, rumah negara
  ungu, mess kuning).

## Unduh data (tab Data)

Tiap dataset di tab **Data** (Tanah, Kantor Utama, Gedung Lain, Rusunara, Rumah Negara, Mess, dan Satker) punya tombol **Excel**, **CSV**, dan
**PDF** di atas tabel. Unduhan mengikuti pencarian dan filter yang sedang berlaku di layar (kata kunci, unit eselon I, provinsi, kondisi, jenis
satker, "belum punya koordinat"), bukan hanya halaman yang tampil. Berkas yang disaring bernama `digitalisasi-<dataset>-disaring_<tanggal>.<ext>`.

- **Endpoint**: `GET /api/v1/digitalisasi/ekspor/:dataset?format=xlsx|csv|pdf` dengan penyaring yang sama dengan daftar (`q`, `ue1`, `provinsi`,
  `kondisi`, `jenis_satker`, `tanpa_koordinat`); CSV menerima `pemisah=titik-koma|koma|tab` (bawaan titik koma, untuk Excel Indonesia). Dapat
  dipakai semua pengguna login, sama seperti daftar. Kode: `backend/handlers/digitalisasi_ekspor.go`; penyaringnya dipakai bersama daftar
  (`dgFilter`) sehingga keduanya tidak bisa berbeda.
- **Excel dan CSV** memuat **semua kolom** tabel (bukan hanya kolom tabel di layar) dan waktu sinkronisasi; kolom teknis (`id`, `id_sinkron`) tidak
  ikut. Judul kolom sama dengan yang terlihat di layar, dengan satuan (`Luas ... (m²)`, `Nilai ... (Rp)`); lintang dan bujur ditulis sebagai bilangan
  dengan tujuh desimal. Waktu sinkronisasi (disimpan UTC) ditulis dalam **WIB**. Excel dibatasi satu sheet (1.048.575 baris); bila lebih, server
  menyarankan CSV.
- **PDF** memuat kolom ringkasan seperti tabel di layar (satker, uraian, kab/kota, provinsi, luas, kondisi, nilai; satker: kode, nama, jenis,
  kab/kota, provinsi, KDJ, KDO) dan dibatasi 5.000 baris.
- Baris dibaca dan ditulis mengalir, jadi tabel besar tidak dimuat seluruhnya ke memori (batas waktu 10 menit).
- **Kolom pribadi** (lihat bagian berikut) tidak ikut berkas milik pengguna biasa.

## Data pribadi

`DIGITALISASI_RUMAH_NEGARA.Nama_Penghuni` (nama pemakai dari `SIMAN2_M_ASET_PEMAKAI`) hanya dikirim ke admin/superadmin: tidak ikut
daftar, detail, pencarian, maupun unduhan Excel/CSV/PDF untuk pengguna lain. Pesan galat sinkronisasi (bisa memuat nama server/tabel) juga hanya
terlihat admin.

## Menjalankan pertama kali

1. `./deploy.sh` (migrasi 020 berjalan otomatis; image backend tidak berubah selain kodenya).
2. Pastikan `SLDK_DB_*` terisi dan IP server diizinkan di SLDK (halaman Sinkronisasi menampilkan peringatan bila koneksi SLDK tidak ada).
3. Buka Digitalisasi Aset → Sinkronisasi → mulai dari satu dataset kecil (mis. *Mess Rumah Negara*) untuk memastikan query dan
   koordinat benar, baru "Sinkronkan semua".

## Hal yang belum diverifikasi terhadap SLDK asli

Seluruh pengujian memakai database palsu; belum ada yang dijalankan ke SLDK:

- kolom `gps_latitude`/`gps_longitude` pada `SIMAN2_M_ASET` dan tipenya (dibaca longgar: angka, atau teks dengan koma desimal);
- arti nilai `SIMAN2_PENGELOLAAN.is_asuransi` (grafik asuransi memetakan `1/Y/YA/TRUE` dan `0/T/N/TIDAK/FALSE`, selain itu ditampilkan apa adanya);
- jumlah baris tiap dataset terhadap batas 500.000 dan berapa lama query berjalan (kini tanpa batas waktu; sinkronisasi otomatis
  mingguan belum pernah berjalan terhadap SLDK asli, hanya diuji dengan database palsu dan jam yang disimulasikan);
- nama UE1: hanya empat yang diketahui dari komentar query (01504 DJP, 01505 DJBC, 01508 DJPb, 01515 BATII); sisanya tampil "UE1 <kode>".
