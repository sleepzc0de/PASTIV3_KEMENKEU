# Digitalisasi Aset

Halaman **Digitalisasi Aset** (`/dashboard/digitalisasi`) menampilkan analitik, peta, dan daftar aset KL 015 yang **disalin ke database PASTI dari datamart**,
yaitu tujuh tabel `DIGITALISASI_*` yang dibentuk dari SLDK oleh satu script di sisi database ([datamart-digitalisasi.md](datamart-digitalisasi.md)). Halaman tidak membaca
SLDK, dan **aplikasi pun tidak pernah mengakses SLDK untuk fitur ini**: sinkronisasi (dijalankan superadmin atau penjadwal) hanya membaca datamart.

| Tab | Isi |
|---|---|
| Ringkasan | Angka utama (satker, tanah, gedung, hunian, nilai, kendaraan dinas), grafik per jenis, kondisi, UE1, provinsi, status hukum, asuransi, kamar tidur, status penghuni, dan tabel **kelengkapan data** (aset tanpa koordinat/foto/kondisi). Tiap grafik punya tombol *Tabel*. |
| Peta | Peta Leaflet sebaran aset. Titik berklaster (donat berwarna menunjukkan komposisi jenis), filter jenis aset dan UE1, klik titik untuk kartu ringkas lalu *Detail lengkap*. |
| Data | Daftar tiap dataset dengan pencarian, filter UE1/provinsi/kondisi, filter "belum punya koordinat", halaman, dan detail per baris (tombol *Lihat di peta*). |
| Sinkronisasi | Status tiap dataset, tombol sinkronisasi (superadmin), kemajuan, pembatalan, dan riwayat. |

Tab **Sinkronisasi hanya tampil bagi superadmin dan Pengguna Barang**; peran UE1, Kanwil, dan Satker hanya melihat tab Ringkasan, Peta, dan Data. Ini ditegakkan di dua sisi:
frontend menyembunyikan tab (tautan `?tab=sinkronisasi` jatuh ke Ringkasan dan status sinkronisasi tidak diminta), dan backend menjawab **403** pada `GET /digitalisasi/sinkronisasi`
bagi peran yang tidak melihat seluruh data (`middleware.RequireCakupanSemuaPesan`). Menjalankan dan membatalkan sinkronisasi tetap khusus superadmin (Pengguna Barang hanya melihat).

## Dashboard Aset

Data yang sama dipakai oleh tab **Aset** di halaman **Dashboard** (`/dashboard`): angka pokok, grafik, tabel kelengkapan (komponen `RingkasanAset`
yang sama dengan tab Ringkasan di sini) ditambah blok **Wawasan analitik** yang dihitung di browser dari ringkasan itu
(`frontend/lib/wawasanAset.ts`, fungsi murni dengan tes `lib/wawasanAset.test.mjs`). Wawasannya: skala aset, konsentrasi nilai pada satu UE1,
sebaran provinsi, aset rusak berat, kelengkapan koordinat/foto/kondisi, cakupan asuransi gedung, satker tanpa satker induk, status hukum tanah
kosong, hunian, dan kesegaran data sinkronisasi. Semua ambangnya ada di blok konstanta di awal berkas. Dashboard hanya **membaca salinan** di
database PASTI; bila belum ada sinkronisasi sama sekali, tampil keadaan kosong dengan tombol ke tab Sinkronisasi (untuk superadmin; peran lain hanya diberi keterangan). Klik pada grafik
membuka tab Data lewat `/dashboard/digitalisasi?tab=data&dataset=<kunci>[&tanpa_koordinat=1][&q=<kata kunci>]`.
Label UE1 (mis. `01504 · DJP`) dan uraiannya dibaca dari referensi UE1 (lihat [referensi-ue1-dan-satker.md](referensi-ue1-dan-satker.md)); tab **Satker** di Dashboard
menghubungkan data aset dengan data pengadaan per satker.

> Fitur **Data Aset (SLDK)** yang lama (pencarian langsung ke tabel aset SLDK, Ringkasan, Pemantauan, dan perintah `pasti-sldk-sync`) sudah
> dihapus, termasuk variabel `.env` `SLDK_ASSET_TABLE`, `SLDK_ASSET_SEARCH_COLUMNS`, dan `SLDK_KL_KODE`. Koneksi `SLDK_DB_*` tidak
> dipakai Digitalisasi Aset lagi (sinkronisasinya memakai `DATAMART_DB_*`); ia tinggal dipakai penarikan referensi Kanwil. Tabel `sldk_agregat`, `sldk_sync_log`, dan `sldk_pengaturan` (migrasi 019) tidak dipakai lagi; migrasinya sengaja
> tidak dihapus dan tabelnya tidak di-DROP otomatis. Bila datanya tidak diperlukan, hapus manual.
## Tabel dan query

Tujuh dataset, masing-masing satu query SLDK (dijalankan oleh script datamart, bukan aplikasi) dan satu tabel di datamart yang disalin ke database PASTI:

| Dataset (key) | Tabel | Query |
|---|---|---|
| `satker` | `DIGITALISASI_SATKER` | `backend/digitalisasi/queries/satker.sql` |
| `tanah` | `DIGITALISASI_TANAH` | `tanah.sql` |
| `gedung_kantor_utama` | `DIGITALISASI_GEDUNG_KANTOR_UTAMA` | `gedung_kantor_utama.sql` |
| `gedung_lainnya` | `DIGITALISASI_GEDUNG_LAINNYA` | `gedung_lainnya.sql` |
| `rusunara` | `DIGITALISASI_RUSUNARA` | `rusunara.sql` |
| `rumah_negara` | `DIGITALISASI_RUMAH_NEGARA` | `rumah_negara.sql` |
| `mess_rumah_negara` | `DIGITALISASI_MESS_RUMAH_NEGARA` | `mess_rumah_negara.sql` |

- Query adalah query asli dari pengelola data, disisipkan apa adanya ke script datamart (`backend/datamart/buat_datamart_digitalisasi.sql`) yang dihasilkan dari berkas-berkas
  ini; aplikasi tidak menjalankannya. **Satu-satunya tambahan** adalah dua kolom
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
3. Jalankan `go test ./datamart -run TestSkripDigitalisasiSesuaiGenerator -update` untuk menghasilkan ulang script datamart, lalu pasang ulang script itu di datamart
   (tim Pusdatin) sebelum menyinkronkan.
4. Jalankan `go test ./...`. Tes menjaga agar query, definisi kolom, migrasi, script datamart, dan kolom yang dipakai frontend tetap sejalan.

## Cara kerja sinkronisasi

1. Superadmin memilih dataset (atau "Sinkronkan semua") dan mengonfirmasi. Permintaan langsung dijawab; pekerjaan berjalan di server,
   jadi halaman boleh ditinggalkan. Hanya **satu antrean** berjalan pada satu waktu; dataset dijalankan berurutan.
2. Per dataset: seluruh isi tabel `DIGITALISASI_*` di **datamart** dibaca ke memori (maks. 500.000 baris, **tanpa batas waktu**: berjalan sampai selesai atau sampai
   superadmin menekan Batalkan), lalu **dalam satu transaksi singkat** isi tabel tujuan di PASTI dikosongkan dan diisi ulang. Pembaca tidak terkunci selama penyalinan
   berjalan; bila ada yang gagal, tabel tidak berubah. Nilai disalin **apa adanya** (DECIMAL dan koordinat sebagai teks, tanpa melewati `float64`): datamart sudah menyaring dan
   mengonversinya persis seperti yang dahulu dilakukan aplikasi (spasi dipangkas, teks dipotong sesuai panjang kolom, koordinat divalidasi dan berpasangan, (0, 0) dibuang).
3. Perlindungan: hasil **0 baris tidak menimpa** tabel yang sudah berisi; kolom yang hilang atau bertipe tak terduga di datamart menghentikan sinkronisasi dengan pesan yang
   jelas; datamart yang **lebih lama** daripada data PASTI saat ini (mis. dipulihkan dari cadangan lama) ditolak, jadi data tidak mundur.
4. `DIGITALISASI_SATKER`: kolom `Status_Gedung_Kantor` dan `Foto` selalu NULL dari sumber, jadi dianggap **diisi manual**. Nilai yang ada di PASTI
   dipertahankan per `Kode_Satker` bila datamart mengirim NULL.
5. Kemajuan dan hasil dicatat di `digitalisasi_sync_log` (status `antri` → `berjalan` → `sukses`/`gagal`/`dibatalkan`), termasuk **data per** kapan: awal penyegaran
   datamart yang menjadi sumber (kolom `data_per`, migrasi 064). Saat server dimulai ulang, baris yang masih `antri`/`berjalan` ditandai `gagal` karena prosesnya sudah hilang.

> Sinkronisasi hanya menyalin data sebatas **penyegaran datamart terakhir**. Datamart disegarkan terjadwal oleh tim Pusdatin (menjalankan ulang script, atau `EXEC dbo.datamart_digitalisasi_segarkan` bila `@Bersihkan = 0`;
> beban beratnya ke SLDK ada di sana, di luar jam kerja); sinkronisasi aplikasi sendiri ringan. Bila datamart belum disegarkan sejak sinkronisasi sebelumnya, sinkronisasi tetap
> berjalan dan riwayatnya ditandai "sama dengan sinkronisasi sebelumnya". Halaman Sinkronisasi memperingatkan bila datamart tak terjangkau, belum pernah disegarkan,
> penyegaran terakhirnya gagal, atau sudah lebih dari 14 hari tidak disegarkan.

### Sinkronisasi otomatis mingguan

Server menyinkronkan dataset sendiri tanpa perlu ditekan manual. Penjadwalnya berjalan di dalam proses backend (tanpa cron di luar) dan
membaca riwayat `digitalisasi_sync_log`, jadi keadaannya bertahan saat server dimulai ulang dan sinkronisasi **manual ikut dihitung**.

- Sebuah dataset **jatuh tempo** bila sinkronisasi suksesnya yang terakhir sudah lebih tua dari 7 hari (atau belum pernah ada). Yang
  jatuh tempo diantrekan bersama, berurutan, sebagai satu antrean (dicatat `dijalankan_oleh = otomatis (mingguan)`).
- Hanya **dimulai** di jendela jam malam, bawaan **01.00-05.00 WIB**, supaya penyalinan tabel besar tidak berjalan di jam kerja. Yang sudah
  berjalan boleh melewati jendela itu sampai selesai. Penjadwal memeriksa tiap 15 menit.
- Percobaan yang **gagal** (atau terhenti karena server dimulai ulang) tidak diulang langsung: dataset yang sama baru dicoba lagi
  paling cepat 20 jam kemudian, yaitu pada malam berikutnya, supaya datamart tidak dibebani berulang saat ada masalah.
- Tidak pernah bersamaan dengan sinkronisasi lain: bila superadmin sedang menyinkronkan manual, penjadwal menunggu putaran berikutnya.
- **Tanpa batas waktu**, baik manual maupun otomatis; sinkronisasi yang macet dihentikan lewat tombol *Batalkan*.
- Halaman Sinkronisasi menampilkan jadwal dan perkiraan jalan berikutnya. Hanya berjalan bila datamart dikonfigurasi (`DATAMART_DB_*` terisi).
- Dataset yang belum pernah disinkronkan ikut diantrekan pada malam pertama setelah deploy; sebelum itu, coba satu dataset kecil dulu
  secara manual (lihat *Menjalankan pertama kali*), atau matikan otomatis dulu.

Pengaturan opsional di `backend/.env` (tanpa mengisinya: aktif, 7 hari, 01-05 WIB):

| Variabel | Arti | Bawaan |
|---|---|---|
| `DIGITALISASI_AUTO_SYNC` | `false` untuk mematikan sinkronisasi otomatis | `true` |
| `DIGITALISASI_AUTO_INTERVAL_HARI` | jarak minimal antar sinkronisasi sukses sebuah dataset (1-365) | `7` |
| `DIGITALISASI_AUTO_JAM_MULAI` / `DIGITALISASI_AUTO_JAM_AKHIR` | jendela jam (WIB, 0-23) sinkronisasi boleh dimulai; sama = sepanjang hari | `1` / `5` |

Nilai di luar rentang diganti bawaannya. Restart backend setelah mengubahnya.

### Sumber data: datamart

Satu-satunya sumber sinkronisasi adalah database **datamart** (`DATAMART_DB_HOST`, `DATAMART_DB_PORT`, `DATAMART_DB_USER`, `DATAMART_DB_PASSWORD`, `DATAMART_DB_NAME` di `backend/.env`; pengguna cukup berhak
`SELECT` pada tujuh tabel `dbo.DIGITALISASI_*` dan `dbo.datamart_digitalisasi_run`). Tanpa `DATAMART_DB_HOST`, sinkronisasi tidak berjalan dan halaman Sinkronisasi menjelaskannya.
Tidak ada lagi pilihan sumber atau peralihan otomatis: variabel `DIGITALISASI_SUMBER` dari versi sebelumnya diabaikan (server mencatat peringatan bila masih terisi).

Datamart dibentuk dan disegarkan oleh satu script di sisi database, [`backend/datamart/buat_datamart_digitalisasi.sql`](../backend/datamart/buat_datamart_digitalisasi.sql); cara memasangnya, jaminan kesamaan
dengan data yang dahulu disimpan aplikasi langsung dari SLDK, batas-batasnya, dan pembuktian tesnya ada di [datamart-digitalisasi.md](datamart-digitalisasi.md).

## Peta

- Titik digambar hanya bila koordinat berada di kotak Indonesia (lintang −11,5 s.d. 6,5; bujur 94,5 s.d. 141,5). Koordinat di luar
  kotak itu (biasanya tanda minus hilang atau lintang/bujur tertukar) dihitung di *Kelengkapan data* dan tidak digambar.
- Maksimum 100.000 titik per dataset; bila lebih, peta memberi peringatan dan UE1 bisa dipakai untuk mempersempit.
- Peta dasar memakai OpenStreetMap lewat browser pengguna. Bila jaringan memblokirnya, titik tetap tampil dan halaman memberi
  catatan. Server lain dapat dipasang lewat variabel build `NEXT_PUBLIC_MAP_TILE_URL` (pola `https://host/{z}/{x}/{y}.png`) dan
  `NEXT_PUBLIC_MAP_TILE_ATTRIBUTION` di `deploy.<env>.env`, lalu bangun ulang frontend.
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

`DIGITALISASI_RUMAH_NEGARA.Nama_Penghuni` (nama pemakai dari `SIMAN2_M_ASET_PEMAKAI`) hanya dikirim ke superadmin: tidak ikut
daftar, detail, pencarian, maupun unduhan Excel/CSV/PDF untuk pengguna lain. Pesan galat sinkronisasi (bisa memuat nama server/tabel) juga hanya
terlihat superadmin.

## Menjalankan pertama kali

1. `./deploy.sh` (migrasi 020, 063, dan 064 berjalan otomatis; image backend tidak berubah selain kodenya).
2. Tim Pusdatin menjalankan script datamart di data warehouse ([datamart-digitalisasi.md](datamart-digitalisasi.md)) dan menjadwalkan penyegarannya. Isi `DATAMART_DB_*` di `backend/.env` (atau lewat
   `./deploy.sh --kredensial`) lalu mulai ulang backend; halaman Sinkronisasi menampilkan peringatan bila datamart belum dikonfigurasi atau belum pernah disegarkan.
3. Buka Digitalisasi Aset → Sinkronisasi → mulai dari satu dataset kecil (mis. *Mess Rumah Negara*) untuk memastikan datanya benar, baru "Sinkronkan semua".

## Hal yang belum diverifikasi terhadap SLDK asli

Seluruh pengujian memakai SLDK palsu dan database palsu; belum ada yang dijalankan ke SLDK asli (script datamart juga belum). Penyalinan datamart → PASTI diuji ujung ke ujung terhadap SQL Server lokal:

- kolom `gps_latitude`/`gps_longitude` pada `SIMAN2_M_ASET` dan tipenya (dibaca longgar: angka, atau teks dengan koma desimal);
- arti nilai `SIMAN2_PENGELOLAAN.is_asuransi` (grafik asuransi memetakan `1/Y/YA/TRUE` dan `0/T/N/TIDAK/FALSE`, selain itu ditampilkan apa adanya);
- jumlah baris tiap dataset terhadap batas 500.000 dan berapa lama script datamart berjalan terhadap SLDK asli (penyalinan datamart → PASTI tanpa batas waktu; sinkronisasi otomatis
  mingguan belum pernah berjalan sungguhan, hanya diuji dengan database palsu dan jam yang disimulasikan);
- nama UE1: hanya empat yang diketahui dari komentar query (01504 DJP, 01505 DJBC, 01508 DJPb, 01515 BATII); sisanya tampil "UE1 <kode>".
