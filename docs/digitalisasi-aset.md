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

## Riwayat bulanan (`DIGITALISASI_ASET_ALL`)

Tabel `DIGITALISASI_*` selalu berisi data **terbaru** (diganti penuh tiap sinkronisasi). Untuk melihat perubahan data dari bulan ke bulan, tabel `DIGITALISASI_ASET_ALL` menampung **potret akhir tiap bulan**
dari ketujuh tabel itu dalam satu tabel: satu baris = satu baris tabel sumber pada satu periode.

**Status: tabel sudah disiapkan (migrasi `065_create_digitalisasi_aset_all.sql`), tetapi penyimpanan otomatis tiap akhir bulan belum dibuat** dan aplikasi belum menampilkannya. Yang sudah ada dan diuji:
tabelnya, dan SQL penyimpanan potret (`digitalisasi.HistorySQL`, `backend/digitalisasi/riwayat.go`) yang kelak dipicu otomatis. Kode keduanya satu sumber: migrasi 065 **dihasilkan** dari `riwayat.go`
(`go test ./digitalisasi -run TestHistoryMigrationFile -update`).

### Struktur

Ketujuh tabel sumber kolomnya berbeda, jadi tabel riwayat memakai **kolom inti seragam bertipe asli** (dipetakan lewat `Roles` tiap dataset: `Alamat_tanah`, `Alamat_bangunan`, dan seterusnya sama-sama menjadi
`alamat`) dan satu **kolom JSON** untuk kolom khusus jenis aset, tanpa puluhan kolom kosong.

| Kolom | Isi |
|---|---|
| `periode` | **tanggal terakhir** bulan yang ditutup: posisi akhir Oktober 2026 = `2026-10-31`. Tabel menolak tanggal lain (CHECK `periode = EOMONTH(periode)`) |
| `jenis_aset` | kunci dataset: `satker`, `tanah`, `gedung_kantor_utama`, `gedung_lainnya`, `rusunara`, `rumah_negara`, `mess_rumah_negara` |
| `kode_ue1`, `kode_satker`, `nama_satker` | satker pemilik |
| `kode_register`, `kode_barang`, `nup`, `uraian` | identitas aset (`kode_register` hanya pada tanah, gedung, dan rumah negara; `kode_barang` dan `nup` kosong pada jenis `satker`) |
| `alamat`, `kelurahan_desa`, `kecamatan`, `kab_kota`, `provinsi` | lokasi |
| `luas`, `nilai` | luas tanah (jenis `tanah`) atau luas bangunan; nilai perolehan (kosong pada jenis yang tidak punya nilai, mis. `rumah_negara`) |
| `kondisi`, `status_hukum`, `status_asuransi`, `status_penghuni` | status (kosong bila jenis aset tidak punya) |
| `latitude`, `longitude` | koordinat bila ada |
| `data_tambahan` | JSON kolom khusus jenis aset, kunci = nama kolom huruf kecil: mis. `kamar_tipe_e`, `kamar_tipe_d`, `kamar_tipe_c` (rusunara, mess), `jumlah_bangunan` dan `luas_bangunan` (tanah), `jumlah_kdj`, `jumlah_kdo` (satker), `fungsi` (gedung lainnya). Nilai kosong tidak ditulis |
| `data_per` | "data per" sumber: awal penyegaran datamart yang menghasilkan data itu (UTC) |
| `diambil_pada` | waktu potret disimpan (UTC) |

Tidak ada kunci unik karena kunci aset berbeda per jenis; gunakan `kode_register`, atau `kode_satker` + `kode_barang` + `nup`, untuk mengikuti satu aset antarbulan (kolom `id` pada tabel `DIGITALISASI_*`
berubah setiap sinkronisasi dan tidak boleh dipakai). Indeks berkelompok pada `(periode, jenis_aset, id)` membuat penambahan tiap bulan berurutan di ujung tabel dan pembacaan satu bulan berurutan; ada
indeks tambahan per satker, per UE1, dan per kode register.

**Yang tidak disalin:** data pribadi (`Nama_Penghuni`), tautan foto (`Foto*`), dan nama pengguna bangunan (`Nama_Pengguna`): riwayat tidak boleh menyimpan lebih banyak daripada yang boleh keluar lewat API Akses
Data. Kolom baru di `datasets.go` otomatis masuk JSON kecuali dikecualikan di `Column.HistoryExcluded`; tes menjaga setiap kolom punya tepat satu tujuan (inti, JSON, atau dikecualikan) dan bahwa kolom yang ditutup API
juga dikecualikan di sini.

### Menyimpan potret

`digitalisasi.HistorySQL(ds)` menghasilkan satu batch per dataset dengan parameter bernama `@periode` (`DATE`, tanggal terakhir bulan) dan `@data_per` (`DATETIME2(3)`, boleh `NULL`); hasilnya satu angka
(jumlah baris yang disimpan). Batch ini **aman diulang**: potret lama untuk periode dan jenis aset yang sama dibuang dalam satu transaksi dengan penyisipan (gagal = tidak ada yang berubah), sedangkan periode dan jenis
aset lain tidak disentuh. Potret yang sudah ada bisa diperbarui dengan mengulangnya, dan dipakai ulang oleh tahap otomatis nanti.

### Contoh pembacaan deret waktu

Total tiap bulan per jenis aset:

```sql
SELECT periode, jenis_aset, COUNT(*) AS jumlah, SUM(luas) AS total_luas, SUM(nilai) AS total_nilai
FROM DIGITALISASI_ASET_ALL
GROUP BY periode, jenis_aset
ORDER BY jenis_aset, periode;
```

Satker yang jumlah, luas, atau nilai tanahnya berubah antara dua bulan:

```sql
WITH per_satker AS (
    SELECT periode, kode_satker, COUNT(*) AS jumlah, SUM(luas) AS luas, SUM(nilai) AS nilai
    FROM DIGITALISASI_ASET_ALL
    WHERE jenis_aset = N'tanah' AND periode IN ('2026-10-31', '2026-11-30')
    GROUP BY periode, kode_satker
)
SELECT COALESCE(b.kode_satker, a.kode_satker) AS kode_satker,
       ISNULL(b.jumlah, 0) - ISNULL(a.jumlah, 0) AS selisih_jumlah,
       ISNULL(b.luas, 0) - ISNULL(a.luas, 0) AS selisih_luas,
       ISNULL(b.nilai, 0) - ISNULL(a.nilai, 0) AS selisih_nilai
FROM (SELECT * FROM per_satker WHERE periode = '2026-10-31') AS a
FULL JOIN (SELECT * FROM per_satker WHERE periode = '2026-11-30') AS b ON b.kode_satker = a.kode_satker
WHERE ISNULL(b.jumlah, 0) <> ISNULL(a.jumlah, 0) OR ISNULL(b.luas, 0) <> ISNULL(a.luas, 0) OR ISNULL(b.nilai, 0) <> ISNULL(a.nilai, 0)
ORDER BY kode_satker;
```

Tanah yang kondisinya berubah antara dua bulan (aset yang sama dikenali dari `kode_register`):

```sql
SELECT a.kode_satker, a.kode_register, a.kondisi AS kondisi_awal, b.kondisi AS kondisi_akhir
FROM DIGITALISASI_ASET_ALL AS a
JOIN DIGITALISASI_ASET_ALL AS b ON b.jenis_aset = a.jenis_aset AND b.kode_register = a.kode_register
WHERE a.jenis_aset = N'tanah' AND a.periode = '2026-10-31' AND b.periode = '2026-11-30'
  AND ISNULL(a.kondisi, N'') <> ISNULL(b.kondisi, N'');
```

Kolom khusus jenis aset dari JSON, mis. jumlah kamar rusunara per bulan:

```sql
SELECT periode,
       SUM(CAST(JSON_VALUE(data_tambahan, '$.kamar_tipe_e') AS INT)) AS kamar_tipe_e,
       SUM(CAST(JSON_VALUE(data_tambahan, '$.kamar_tipe_d') AS INT)) AS kamar_tipe_d,
       SUM(CAST(JSON_VALUE(data_tambahan, '$.kamar_tipe_c') AS INT)) AS kamar_tipe_c
FROM DIGITALISASI_ASET_ALL
WHERE jenis_aset = N'rusunara'
GROUP BY periode
ORDER BY periode;
```

Keempat contoh di atas dijalankan oleh tes integrasi terhadap data uji, jadi sintaksnya dijamin benar. `JSON_VALUE` mengembalikan teks maksimal 4.000 karakter; untuk kolom teks panjang pakai `OPENJSON ... WITH`.

### Penyimpanan otomatis (tahap berikutnya)

Yang masih harus diputuskan dan dibuat: pemicu (mis. penjadwal aplikasi pada awal bulan menyimpan periode `EOMONTH` bulan sebelumnya), sumbernya (tabel `DIGITALISASI_*` PASTI sesudah sinkronisasi sukses terakhir, dengan
`@data_per` dari riwayat sinkronisasi `digitalisasi_sync_log.data_per`), penanganan bulan yang terlewat, dan penolakan menyimpan potret bila tabel sumber kosong atau sinkronisasi terakhir gagal (supaya riwayat tidak
berisi bulan kosong).
Perkiraan ukuran: tiap bulan menambah sebanyak jumlah seluruh baris ketujuh tabel sumber.

## Menjalankan pertama kali

1. `./deploy.sh` (migrasi 020, 063, 064, dan 065 berjalan otomatis; image backend tidak berubah selain kodenya).
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
