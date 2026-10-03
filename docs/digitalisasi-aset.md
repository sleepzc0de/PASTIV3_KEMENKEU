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
   (maks. 500.000 baris, batas waktu 2 jam), lalu **dalam satu transaksi singkat** isi tabel tujuan dikosongkan dan diisi ulang.
   Pembaca tidak terkunci selama query SLDK berjalan; bila ada yang gagal, tabel tidak berubah.
3. Perlindungan: hasil **0 baris tidak menimpa** tabel yang sudah berisi; kolom hasil yang hilang dari query menghentikan
   sinkronisasi dengan pesan yang jelas; nilai teks yang melebihi panjang kolom dipotong dan dicatat; koordinat di luar rentang bumi,
   tidak berpasangan, atau (0, 0) dibuang (NULL).
4. `DIGITALISASI_SATKER`: kolom `Status_Gedung_Kantor` dan `Foto` selalu NULL dari query, jadi dianggap **diisi manual**. Nilai yang ada
   dipertahankan per `Kode_Satker` bila sumber mengirim NULL.
5. Kemajuan dan hasil dicatat di `digitalisasi_sync_log` (status `antri` → `berjalan` → `sukses`/`gagal`/`dibatalkan`). Saat server
   dimulai ulang, baris yang masih `antri`/`berjalan` ditandai `gagal` karena prosesnya sudah hilang.

> Tiap query membaca tabel aset SLDK (±206 GB). Jalankan di luar jam kerja dan jangan bersamaan dengan
> `pasti-sldk-sync ringkasan` (lihat [sldk-sync.md](sldk-sync.md)).

## Peta

- Titik digambar hanya bila koordinat berada di kotak Indonesia (lintang −11,5 s.d. 6,5; bujur 94,5 s.d. 141,5). Koordinat di luar
  kotak itu (biasanya tanda minus hilang atau lintang/bujur tertukar) dihitung di *Kelengkapan data* dan tidak digambar.
- Maksimum 100.000 titik per dataset; bila lebih, peta memberi peringatan dan UE1 bisa dipakai untuk mempersempit.
- Peta dasar memakai OpenStreetMap lewat browser pengguna. Bila jaringan memblokirnya, titik tetap tampil dan halaman memberi
  catatan. Server lain dapat dipasang lewat variabel build `NEXT_PUBLIC_MAP_TILE_URL` (pola `https://host/{z}/{x}/{y}.png`) dan
  `NEXT_PUBLIC_MAP_TILE_ATTRIBUTION` di `deploy.env`, lalu bangun ulang frontend.
- Warna jenis aset tetap per jenis (tanah biru, kantor utama jingga, gedung lainnya aqua, rusunara merah muda, rumah negara
  ungu, mess kuning).

## Data pribadi

`DIGITALISASI_RUMAH_NEGARA.Nama_Penghuni` (nama pemakai dari `SIMAN2_M_ASET_PEMAKAI`) hanya dikirim ke admin/superadmin: tidak ikut
daftar, detail, maupun pencarian untuk pengguna lain. Pesan galat sinkronisasi (bisa memuat nama server/tabel) juga hanya terlihat
admin.

## Menjalankan pertama kali

1. `./deploy.sh` (migrasi 020 berjalan otomatis; image backend tidak berubah selain kodenya).
2. Pastikan `SLDK_DB_*` terisi dan IP server diizinkan di SLDK (halaman Sinkronisasi menampilkan peringatan bila koneksi SLDK tidak ada).
3. Buka Digitalisasi Aset → Sinkronisasi → mulai dari satu dataset kecil (mis. *Mess Rumah Negara*) untuk memastikan query dan
   koordinat benar, baru "Sinkronkan semua".

## Hal yang belum diverifikasi terhadap SLDK asli

Seluruh pengujian memakai database palsu; belum ada yang dijalankan ke SLDK:

- kolom `gps_latitude`/`gps_longitude` pada `SIMAN2_M_ASET` dan tipenya (dibaca longgar: angka, atau teks dengan koma desimal);
- arti nilai `SIMAN2_PENGELOLAAN.is_asuransi` (grafik asuransi memetakan `1/Y/YA/TRUE` dan `0/T/N/TIDAK/FALSE`, selain itu ditampilkan apa adanya);
- jumlah baris tiap dataset terhadap batas 500.000 dan lama query terhadap batas 2 jam;
- nama UE1: hanya empat yang diketahui dari komentar query (01504 DJP, 01505 DJBC, 01508 DJPb, 01515 BATII); sisanya tampil "UE1 <kode>".
