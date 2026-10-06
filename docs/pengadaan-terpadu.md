# Pengadaan Terpadu: Penarikan Data, Ekspor, dan Dashboard

Menu **Pengadaan > Pengadaan Terpadu** menyatukan semua data Inaproc yang sebelumnya tersebar di empat menu (Pengadaan, Tender, E-Katalog V5, dan E-Katalog V6; keempatnya sudah dihapus). Analitiknya tampil di Dashboard:

| Halaman | Alamat | Isi |
|---|---|---|
| Penarikan Data | `/dashboard/pengadaan-terpadu/penarikan` | Tab **Tarik Data** (manual), **Data & Ekspor** (lihat, saring, unduh Excel/CSV/PDF), **Riwayat**, **Jadwal Otomatis** |
| Dashboard Pengadaan | `/dashboard?tab=pengadaan` | Tab **Pengadaan** di halaman Dashboard: sub-tab Ikhtisar, Perencanaan, Pemilihan, Kontrak, E-Katalog; grafik dan wawasan analitik |

Semua pengguna login boleh melihat, mengekspor, dan membuka dashboard, **dibatasi ke satkernya menurut peran data** (lihat [Pembatasan per satker](#pembatasan-per-satker)).
Menjalankan/membatalkan penarikan dan mengubah jadwal khusus admin/superadmin. Halaman per-dataset yang lama (`/dashboard/pengadaan/...`, `/tender/...`, `/ekatalog/...`, `/ekatalog-v6/...`) dan alamat
`/dashboard/pengadaan-terpadu/dasbor` sudah dihapus; alamat lamanya dialihkan (`redirects()` di `frontend/next.config.ts`) ke Dashboard tab Pengadaan
atau ke Penarikan Data. Rute API per-dataset yang lama (`GET/POST /api/v1/inaproc/<kelompok>/<dataset>[/local|/sync]` dan `/inaproc/sync-log`) juga
dicabut; semuanya kini lewat `/inaproc/penarikan`, `/inaproc/data`, `/inaproc/ekspor`, dan `/inaproc/analitik`.

## Katalog dataset (34)

Satu daftar di `backend/handlers/inaproc_penarikan_katalog.go` (`DaftarDataset`): Pengadaan/RUP 8, Tender dan Non-Tender 16,
E-Katalog V5 5, E-Katalog V6 5. Tiap entri punya ID = jalur API (mis. `tender/pengumuman`), tabel, mode penarikan, dan kolom penyaring.

| Mode | Yang dibutuhkan | Ikut otomatis |
|---|---|---|
| `klpd_tahun` | kode KLPD + tahun | ya |
| `klpd` | kode KLPD | ya |
| `transaksi` | tahun + status (bawaan `COMPLETED`) + KLPD | ya |
| `kategori` | kosong = kategori tingkat 1 (tingkat 2/3 butuh kode induk) | ya (tingkat 1) |
| `kode` | satu atau beberapa kode (penyedia, komoditas, distributor, produk penyedia) | **tidak**: Inaproc tidak punya daftar "semua" |

Menambah dataset: daftarkan di katalog (`generikDataset` untuk endpoint datar bercursor, `tulisanDataset` untuk handler lama). Tes katalog
memeriksa tabel, kolom penyaring, kolom ringkasan, dan kolom kunci terhadap migrasi.

## Cara penarikan bekerja

- **Satu antrean pada satu waktu** (`PenarikInaproc`, `inaproc_penarikan_manager.go`), berjalan di goroutine server, kemajuannya dibaca halaman
  tiap 2 detik. Tugas = satu dataset dengan satu isian (mis. "Pengumuman Tender, K10, 2025"). Jeda antar tugas bawaan 2 detik.
- **Dataset generik (21)**: halaman diambil dulu ke berkas sementara tanpa menyentuh database, baru data lama dihapus dan yang baru disisipkan
  **dalam satu transaksi**. Pengambilan gagal atau dibatalkan = data lama utuh. Baris yang gagal disimpan dilewati dan dihitung; hanya 3 galat
  pertama per penarikan yang ditulis ke log (selebihnya diringkas satu baris). **Lebar kolom**: sebelum menyimpan, lebar kolom `NVARCHAR(n)` tabelnya
  dibaca dari skema; nilai teks yang lebih panjang dari kolomnya dipotong (bukan menggugurkan satu baris penuh) dan dicatat sebagai "N nilai
  dipotong karena melebihi lebar kolom" di riwayat dan log. Pemotongan hanya jaring pengaman: bila muncul, lebarkan kolomnya lewat migrasi
  (contoh: 047 untuk `nip_pokja`/`nama_pokja`, karena satu pengumuman memuat seluruh anggota pokja).
- **Dataset lama (13, RUP dan non-tender)**: handler sinkron lama dipanggil di dalam proses. Handler itu menghapus data lama lebih dulu, jadi
  sebelum dijalankan dilakukan **uji sambungan** (satu permintaan `limit=1`); bila Inaproc menolak (token, 429), penarikan dibatalkan sebelum
  data disentuh. Kegagalan di tengah penarikan dataset ini tetap meninggalkan data parsial sampai penarikan berikutnya. Pembatalan baru
  berlaku setelah dataset itu selesai.
- **401** (token salah) dan token kosong: sisa antrean dilewati, tidak dicoba satu per satu. **403** berlaku per dataset: izin token di Inaproc bisa
  berbeda tiap endpoint dan dataset sebelumnya mungkin sudah berhasil dengan token yang sama, jadi hanya sisa tugas dataset itu yang dilewati;
  dataset lain tetap ditarik (bila semuanya ditolak, pemutus beruntun yang menghentikan antrean).
- Setiap tugas dicatat di `inaproc_penarikan` (status, baris, halaman, percobaan, pesan, dan isian tugas sebagai JSON) dan di `inaproc_sync_log`
  (dipakai kartu aktivitas lama). Antrean yang menggantung karena server dimulai ulang ditandai **dibatalkan** (bukan gagal) saat server naik,
  jadi tidak menghabiskan kesempatan percobaan.

## Penarikan yang stabil (batas Inaproc, percobaan ulang, tanpa tabrakan)

**Batas permintaan Inaproc**: 1.000 permintaan per 60 detik dan kuota 5.000 permintaan yang direset tiap jam. Semua panggilan ke Inaproc (antrean,
penjadwal, tampilan langsung) lewat satu pembatas (`inaproc_batas.go`, `inaproc_klien.go`):

- Jendela geser 60 detik dan 60 menit, dengan batas sedikit di bawah batas Inaproc (`INAPROC_BATAS_PER_MENIT` 800, `INAPROC_BATAS_PER_JAM` 4.500).
  Jendela geser lebih ketat daripada jendela tetap, jadi aman bagaimanapun Inaproc mereset kuotanya. Bila jatah habis, penarikan **menunggu** (tidak gagal).
- Hitungan per menit disimpan di `inaproc_kuota_menit` (ditulis tiap 10 detik, ditambahkan sebagai selisih), jadi kuota yang sudah terpakai tidak
  "terlupa" saat server dimulai ulang atau dua salinan backend hidup bersamaan saat deploy.
- Sebelum tugas dimulai, kuota **dipesan** sebesar perkiraan (halaman penarikan suksesnya yang terakhir + 30%, bawaan 30) supaya penarikan tidak
  berhenti di tengah, khususnya dataset lama yang menghapus data lebih dulu.
- **429**: semua pemanggil ikut ditahan; `Retry-After` dipakai bila ada, bila tidak jedanya naik bertahap (65 detik, 5, 15, 60 menit) untuk 429
  berulang dalam 30 menit. Per halaman: 429 dicoba 4 kali, 5xx/galat jaringan 3 kali (jeda 3 dan 10 detik), lalu dilanjutkan dari **halaman yang
  gagal** (bukan dari awal). Koneksi HTTP dipakai ulang.
- Tampilan langsung (halaman per-dataset lama) satu kali coba dan tidak menggantung; bila kuota habis dijawab 429 seketika.

**Kebijakan gagal tarik** (manual maupun otomatis; `inaproc_penarikan_jadwal.go`, dibaca dari riwayat sehingga bertahan saat server dimulai ulang):

- Tugas yang gagal dicoba ulang otomatis, paling banyak **3 kali** per siklus (`INAPROC_MAKS_PERCOBAAN`), dengan jarak minimal 10 menit. Percobaan
  ulang tidak menunggu jendela jam.
- Setelah 3 kali gagal, tugas **istirahat 8 jam** (`INAPROC_ISTIRAHAT_JAM`), lalu boleh ditarik lagi dan siklus baru dimulai. Tugas di rencana
  otomatis ditarik lagi otomatis; tugas manual di luar rencana (mis. per kode) tidak dicoba otomatis lagi setelah istirahat.
- Hanya status gagal yang dihitung; dibatalkan, dilewati, dan terhenti karena server mulai ulang tidak. Satu penarikan sukses (manual atau otomatis)
  menutup siklus. Halaman Penarikan Data menampilkan tugas yang sedang menunggu percobaan ulang atau istirahat.
- **Pemutus beruntun**: 5 tugas gagal berturut-turut menghentikan antrean (sisanya dilewati) dan menahan penarikan otomatis 30 menit, supaya
  kesempatan tidak habis percuma saat Inaproc atau jaringan sedang bermasalah.

**Tidak saling tabrakan**: hanya satu antrean berjalan di satu waktu; kunci aplikasi SQL Server (`sp_getapplock`) menutup celah bila ada dua
salinan backend (mis. saat deploy).
- Non-admin tidak melihat isi galat mentah maupun nama pemicu (bisa memuat alamat/potongan respons Inaproc).

## Pembatasan per satker

Peran data UE1, Kanwil, dan Satker (lihat [peran-data.md](peran-data.md)) hanya melihat pengadaan satker dalam cakupannya. Kuncinya `kd_satker_str` Inaproc, kode satker 6 digit yang sama
dengan karakter ke-10 sampai ke-15 kode satker data aset (`SUBSTRING(Kode_Satker, 10, 6)`; kode yang kurang dari 6 digit dilengkapi nol di depan).

- **Satker**: `kd_satker_str` sama dengan kodenya (juga bila satkernya belum ada di data aset).
- **UE1 / Kanwil**: `kd_satker_str` ada di antara satker data aset (`DIGITALISASI_SATKER`) yang kode lengkapnya berawalan kode UE1 (5 digit) atau Kanwil (9 digit). Satker yang punya
  pengadaan tetapi tidak ada di data aset tidak terlihat oleh UE1/Kanwil-nya (Inaproc tidak memuat UE1/Kanwil).
- Yang melihat seluruh data (Super Admin, Admin, Pengguna Barang, pengguna tanpa peran bila pembatasan belum diwajibkan) tidak berubah: query-nya persis seperti sebelumnya.

Tiap tabel masuk salah satu golongan di `backend/handlers/inaproc_cakupan.go` (dijaga tes `inaproc_cakupan_test.go`: dataset baru yang belum digolongkan membuat tes gagal, bukan terbuka):

| Golongan | Tabel | Cara membatasi |
|---|---|---|
| Langsung | 20 tabel yang punya kolom `kd_satker_str`: paket penyedia (+terumumkan, +anggaran), swakelola terumumkan dan anggarannya, riwayat kaji ulang, jadwal tahapan, tender (pengumuman, peserta, e-kontrak, kontrak, selesai), non-tender (pengumuman, kontrak, selesai), pencatatan non-tender (+realisasi) dan swakelola, instansi dan satker E-Katalog | `kd_satker_str` di cakupan |
| Tautan | `inaproc_paket_swakelola`, `inaproc_pencatatan_swakelola_realisasi`, `inaproc_tender_selesai_nilai`, paket e-purchasing V5 dan V6 | Lewat kode paket yang juga ada di tabel golongan langsung: kode RUP (swakelola, e-purchasing; satu paket e-purchasing bisa memuat beberapa kode dipisah `;`), `kd_swakelola_pct`, atau `kd_tender`. Baris yang paketnya tidak ditemukan di tabel pembanding (mis. paket e-purchasing swasta tanpa kode RUP) **tidak ikut** bagi peran terbatas |
| Tertutup | program master (hanya `kd_satker` internal), e-kontrak non-tender (hanya alamat satker), transaksi per produk E-Katalog V6, dan rujukan katalog (komoditas, penyedia, distributor, kategori, produk penyedia) | 403 pada data dan ekspor; tidak muncul di daftar dataset; bagian dasbor yang memakainya (Pagu program, Transaksi per produk V6) tidak dihitung dan disebut di catatan dasbor (`tidak_tersedia`) |

Yang berubah di API:

- `GET /inaproc/dataset` (baru): dataset yang boleh dibuka peran, beserta jumlah baris **dalam cakupan** (dipakai halaman Pengadaan Terpadu bagi peran terbatas; tidak memuat keadaan penarikan).
- `GET /inaproc/data/...`, `/inaproc/ekspor/...`, `/inaproc/analitik`: terbuka bagi peran terbatas tetapi dibatasi; tahun yang tersedia juga hanya dari baris dalam cakupan. Hasil dasbor di cache **per cakupan**
  (kunci KLPD, tahun, tingkat, kode) supaya dasbor satu satker tidak pernah dilayani ke peran lain. Respons dasbor memuat `batas` ({tingkat, kode}) dan `tidak_tersedia` bagi peran terbatas.
- `GET /inaproc/penarikan`, `/penarikan/aktif`, `/penarikan/riwayat`: tetap hanya untuk peran yang melihat seluruh data (403 bagi UE1/Kanwil/Satker).
- Pengguna yang belum diberi peran sementara `PERAN_DATA_WAJIB=true` mendapat 403 di semua `/inaproc/*`.

Tampilan: tab **Pengadaan** di Dashboard dan menu **Pengadaan > Pengadaan Terpadu** tampil bagi semua peran yang punya data. Peran terbatas melihat dasbor dengan catatan cakupan, dan halaman Pengadaan Terpadu
hanya berisi **Data & Ekspor** (tanpa Tarik Data, Riwayat, dan Jadwal Otomatis, yang urusan admin).

Belum diverifikasi terhadap data asli: bahwa `kd_satker_str` memang kode satker 6 digit yang sama dengan data aset (dasarnya sama dengan tab Satker di Dashboard) dan bahwa kode RUP/tender pada tabel tautan cocok dengan
tabel pembandingnya. Bila angka satker terasa terlalu kecil, periksa lebih dulu apakah satkernya ada di data aset (UE1/Kanwil) dan apakah `kd_satker_str`-nya terisi.

## Penarikan otomatis

Penjadwal berjalan di dalam server (cek tiap 5 menit, pertama 2 menit setelah server mulai) dan membaca pengaturan dari database tiap putaran,
jadi perubahan dari halaman berlaku tanpa memulai ulang server.

- Bawaan: aktif, **tiap 2 hari**, mulai antara **01.00 dan 05.00 WIB**, tahun berjalan dan satu tahun sebelumnya, jeda 2 detik, semua dataset otomatis
  (29 dataset, 56 tugas per putaran).
- Sebuah tugas reguler jatuh tempo bila **penarikan otomatis** suksesnya yang terakhir lebih tua dari interval (penarikan manual tidak menggeser
  jadwal) dan hanya dimulai di jendela jam. Tugas yang gagal mengikuti kebijakan percobaan ulang di atas.
- Hanya berjalan bila `INAPROC_TOKEN` terisi dan tidak ada antrean lain yang berjalan.
- Pengaturan tersimpan di `inaproc_penarikan_pengaturan` (satu baris). Selama belum pernah disimpan, nilai bawaan dari env:

| Env | Arti | Bawaan |
|---|---|---|
| `INAPROC_AUTO_SYNC` | `false` mematikan penarikan otomatis | `true` |
| `INAPROC_AUTO_INTERVAL_HARI` | 1-30 | `2` |
| `INAPROC_AUTO_JAM_MULAI` / `INAPROC_AUTO_JAM_AKHIR` | jendela jam WIB (0-23); sama = sepanjang hari | `1` / `5` |
| `INAPROC_AUTO_JUMLAH_TAHUN` | 1-5 tahun (berjalan + sebelumnya) | `2` |
| `INAPROC_BATAS_PER_MENIT` / `INAPROC_BATAS_PER_JAM` | batas permintaan ke Inaproc (di bawah 1.000 per 60 detik dan 5.000 per jam) | `800` / `4500` |
| `INAPROC_MAKS_PERCOBAAN` / `INAPROC_ISTIRAHAT_JAM` | percobaan per siklus (1-10) dan lama istirahat (1-72 jam) | `3` / `8` |

## Ekspor

`GET /api/v1/inaproc/ekspor/<dataset>?format=xlsx|csv|pdf&kode_klpd=&tahun=&cari=&pemisah=titik-koma|koma|tab`

- **Excel**: semua kolom, ditulis mengalir (tabel besar tidak dimuat ke memori), bilangan dan tanggal sungguhan, judul dibekukan, filter, maksimal 1.048.575 baris
  (lebih dari itu ditolak 422 sebelum data dibaca).
- **CSV**: UTF-8 dengan BOM, pemisah bawaan titik koma (Excel Indonesia), angka berdesimal titik. Sel teks yang diawali `= + - @` diberi apostrof
  (mencegah rumus berbahaya dari data luar).
- **PDF**: A4 lanskap, kolom ringkasan, maksimal 5.000 baris (keterangan bila terpotong).
- Kolom teknis (`row_key`, `extra_json`) tidak diekspor. Penyaring memakai parameter SQL; nama tabel dan kolom hanya dari katalog.

## Dashboard Pengadaan

`GET /api/v1/inaproc/analitik?tahun=&kode_klpd=&segarkan=1`. Tiap bagian dihitung terpisah (kegagalan satu bagian tidak menjatuhkan yang lain),
hasilnya di-cache 2 menit dan dikosongkan setiap antrean penarikan selesai. Bagian dihubungkan lewat **kode RUP**: tiap paket RUP aktif
(tidak dihapus, tidak nonaktif) dicocokkan dengan pengumuman tender, non-tender, atau paket e-purchasing V5/V6; yang cocok ke beberapa saluran
dihitung di saluran pertama (tender, non-tender, e-purchasing).

Wawasan dihasilkan `backend/analitik/wawasan.go` dari angka tersebut; semua ambangnya (konsentrasi satker 35%, triwulan IV 40%, HHI 1.500/2.500,
peserta tunggal 30%/50%, addendum 15%/30%, median proses 60 hari, efisiensi 2%/8%, dst.) ada di blok konstanta di awal berkas.

## Tabel

Migrasi `045_create_inaproc_penarikan.sql`: `inaproc_penarikan` (riwayat tugas) dan `inaproc_penarikan_pengaturan` (pengaturan otomatis).
Migrasi `046_inaproc_penarikan_kuota_percobaan.sql`: kolom `permintaan` (isian tugas) dan indeks pada `inaproc_penarikan`, serta `inaproc_kuota_menit`
(hitungan permintaan per menit).
Migrasi `047_inaproc_tender_pengumuman_pokja_lebar.sql`: `nip_pokja` dan `nama_pokja` pada `inaproc_tender_pengumuman` menjadi `NVARCHAR(MAX)` (sebelumnya
50 dan 255 karakter sehingga pengumuman dengan banyak anggota pokja gagal disimpan). Idempotent; data yang ada tetap.
Migrasi `048_inaproc_lebarkan_kolom_dari_data_asli.sql`: kolom yang terbukti terlalu sempit pada penarikan pertama ke Inaproc asli. `kd_rup`/`kd_rup_paket`
(delapan tabel tender dan non-tender) menjadi `NVARCHAR(450)` (batas terbesar yang masih boleh diindeks; indeksnya tetap dipakai) karena satu paket dapat
memuat beberapa kode RUP dipisah `;`; `nama_paket`, `nama_penyedia`, dan `no_realisasi` pada pencatatan non tender (+ realisasi) menjadi `NVARCHAR(MAX)`.
Hanya mengubah kolom yang masih lebih sempit dari sasarannya, sehingga aman dijalankan ulang.
Migrasi `049_inaproc_kd_rup_tanpa_batas.sql`: 450 karakter ternyata masih kurang (satu baris pencatatan non tender memuat puluhan kode RUP), jadi `kd_rup`/`kd_rup_paket`
di kedelapan tabel menjadi `NVARCHAR(MAX)` dan tujuh indeksnya dibuang (kolom MAX tidak bisa menjadi kunci indeks). Tidak ada query yang mencari berdasarkan
kolom itu di tabel-tabel ini, jadi indeksnya tidak terpakai. Idempotent.
Migrasi `050_inaproc_lebarkan_nama_paket_rup_dan_mak.sql`: `nama_paket` pada empat tabel RUP (penyedia dan swakelola, termasuk terumumkan) menjadi `NVARCHAR(MAX)`
karena satu paket bernama gabungan panjang melebihi 500 karakter (handler lama membuang barisnya); `mak` pada `inaproc_ekatalog6_paket_epurchasing`
menjadi `NVARCHAR(MAX)`. Idempotent.

## Pengujian

- `go test ./...` (tanpa database sungguhan; memakai `internal/fakesql`).
- Tes integrasi SQL Server (dilewati bila env kosong): `PASTI_UJI_MSSQL_DSN=sqlserver://...` ke database **khusus tes** yang migrasinya sudah diterapkan,
  lalu `go test ./handlers -run SQLServer`. Data uji memakai kode KLPD `UJI` dan dibersihkan sendiri. Memeriksa SQL dasbor, pembaca data/ekspor
  untuk ke-34 dataset, serta riwayat dan pengaturan penarikan terhadap skema asli.
- Frontend: `node --test lib/pengadaan.test.mjs` (Node 22.18+), `npx tsc --noEmit`.

## Asumsi dan batasan

- Belum diuji ke API Inaproc sungguhan (rate limit, bentuk respons, dan isi `status_*` bisa berbeda dari asumsi). Pola status pada wawasan
  "tender gagal" dan "belum terumumkan" memakai kecocokan teks (`gagal|batal|ulang|ditutup`, `umumkan`).
- `total` pada paket e-purchasing V6 dianggap nilai per order. `kd_rup` pada pengumuman dan paket e-purchasing bisa memuat beberapa kode dipisah `;`
  (terlihat pada data asli); corong dasbor memecahnya dengan `STRING_SPLIT` (SQL Server 2016+, tingkat kompatibilitas 130+) sebelum dicocokkan ke
  paket RUP. Daftar lokal dan ekspor menampilkan kolomnya apa adanya.
- Alasan penolakan Inaproc (mis. 403) kini dibaca dari badan galat dalam beberapa bentuk dan, bila tidak dikenali, potongan isinya disertakan di
  pesan tugas dan log (`Inaproc membalas 403, isi: ...`). Balasan nyata untuk `tender/tender-selesai` adalah
  `{"error":"Access to this API has been disallowed"}`: token Inaproc belum diberi izin ke endpoint itu, jadi izinnya harus diminta ke Inaproc.
  Tugasnya tetap dicatat gagal dan ikut kebijakan percobaan ulang (3 kali per siklus, lalu istirahat 8 jam) sampai izin diberikan.
- 13 handler sinkron lama (RUP dan non-tender) belum punya jaring pengaman lebar kolom: baris yang gagal disimpan tetap dibuang, tetapi kini
  dihitung (`total_failed`) dan muncul di riwayat sebagai "N baris gagal disimpan", selain di log. Kolom yang terbukti sempit pada data asli dilebarkan
  lewat migrasi (048-050).
- Daftar kosong dari backend harus berupa `[]`, bukan `null` (irisan `nil` di Go menjadi `null` di JSON). `bermasalah` pernah dikirim `null` pada server tanpa
  riwayat kegagalan sehingga halaman Penarikan Data crash (`Cannot read properties of null (reading 'length')`) padahal di mesin yang punya kegagalan
  tampil normal. Backend kini menginisialisasi daftarnya (`out := []TugasBermasalah{}`) dan frontend membacanya lewat `daftarAman`; kartu kuota juga
  disembunyikan bila backend (lebih lama) tidak mengirim `kuota`. Tes integrasi memeriksa bahwa `bermasalah`, `riwayat`, `datasets`, `kelompok`, dan
  `tahun_bawaan` selalu larik.
- Sesi login aplikasi (cookie `pasti_access_token`) berakhir sesuai umur JWT tanpa pembaruan otomatis; penarikan yang lebih lama dari itu tetap berjalan
  di server, tetapi halaman mengarahkan ke login (401 pada polling `/inaproc/penarikan/aktif`). Masuk lagi untuk melihat kemajuannya.
- Dataset rujukan (penyedia, komoditas, distributor, produk penyedia) hanya terisi lewat penarikan per kode; nama di grafik E-Katalog muncul
  bila sudah ditarik, selebihnya kode.
- Dataset RUP/non-tender lama belum seatomik dataset generik (lihat di atas); kuotanya dipesan di muka dan penarikan gagalnya dicoba ulang oleh kebijakan percobaan.
- Hitungan kuota hanya mencakup permintaan dari aplikasi ini. Bila token yang sama dipakai pihak lain, Inaproc bisa menjawab 429 lebih awal; itu
  ditangani jeda bersama dan percobaan ulang.
