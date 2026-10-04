# Pengadaan Terpadu: Penarikan Data, Ekspor, dan Dasbor

Dua halaman di menu **Pengadaan Terpadu** menyatukan semua data Inaproc yang sebelumnya tersebar di empat menu:

| Halaman | Alamat | Isi |
|---|---|---|
| Penarikan Data | `/dashboard/pengadaan-terpadu/penarikan` | Tab **Tarik Data** (manual), **Data & Ekspor** (lihat, saring, unduh Excel/CSV/PDF), **Riwayat**, **Jadwal Otomatis** |
| Dasbor Pengadaan | `/dashboard/pengadaan-terpadu/dasbor` | Tab Ikhtisar, Perencanaan, Pemilihan, Kontrak, E-Katalog; grafik dan wawasan analitik |

Semua pengguna login boleh melihat, mengekspor, dan membuka dasbor. Menjalankan/membatalkan penarikan dan mengubah jadwal khusus
admin/superadmin. Halaman per-dataset yang lama (`/dashboard/pengadaan/...`, `/tender/...`, `/ekatalog/...`) tetap ada dan memakai tabel yang sama.

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
  **dalam satu transaksi**. Pengambilan gagal atau dibatalkan = data lama utuh. Baris yang gagal disimpan dilewati dan dihitung.
- **Dataset lama (13, RUP dan non-tender)**: handler sinkron lama dipanggil di dalam proses. Handler itu menghapus data lama lebih dulu, jadi
  sebelum dijalankan dilakukan **uji sambungan** (satu permintaan `limit=1`); bila Inaproc menolak (token, 429), penarikan dibatalkan sebelum
  data disentuh. Kegagalan di tengah penarikan dataset ini tetap meninggalkan data parsial sampai penarikan berikutnya. Pembatalan baru
  berlaku setelah dataset itu selesai.
- **429 (batas laju)**: tugas dicoba ulang tiga kali dengan jeda 45 detik, 2 menit, 5 menit. **401/403/503** (token ditolak/kosong): sisa antrean
  dilewati, tidak dicoba satu per satu.
- Setiap tugas dicatat di `inaproc_penarikan` (status, baris, halaman, percobaan, pesan) dan di `inaproc_sync_log` (dipakai kartu aktivitas lama).
  Antrean yang menggantung karena server dimulai ulang ditandai gagal saat server naik.
- Non-admin tidak melihat isi galat mentah maupun nama pemicu (bisa memuat alamat/potongan respons Inaproc).

## Penarikan otomatis

Penjadwal berjalan di dalam server (cek tiap 15 menit, pertama 2 menit setelah server mulai) dan membaca pengaturan dari database tiap putaran,
jadi perubahan dari halaman berlaku tanpa memulai ulang server.

- Bawaan: aktif, **tiap 2 hari**, mulai antara **01.00 dan 05.00 WIB**, tahun berjalan dan satu tahun sebelumnya, jeda 2 detik, semua dataset otomatis
  (29 dataset, 56 tugas per putaran).
- Sebuah tugas jatuh tempo bila **penarikan otomatis** suksesnya yang terakhir lebih tua dari interval (penarikan manual tidak menggeser jadwal)
  dan percobaan otomatis terakhirnya (apa pun hasilnya) lebih dari 6 jam lalu.
- Hanya berjalan bila `INAPROC_TOKEN` terisi dan tidak ada antrean lain yang berjalan.
- Pengaturan tersimpan di `inaproc_penarikan_pengaturan` (satu baris). Selama belum pernah disimpan, nilai bawaan dari env:

| Env | Arti | Bawaan |
|---|---|---|
| `INAPROC_AUTO_SYNC` | `false` mematikan penarikan otomatis | `true` |
| `INAPROC_AUTO_INTERVAL_HARI` | 1-30 | `2` |
| `INAPROC_AUTO_JAM_MULAI` / `INAPROC_AUTO_JAM_AKHIR` | jendela jam WIB (0-23); sama = sepanjang hari | `1` / `5` |
| `INAPROC_AUTO_JUMLAH_TAHUN` | 1-5 tahun (berjalan + sebelumnya) | `2` |

## Ekspor

`GET /api/v1/inaproc/ekspor/<dataset>?format=xlsx|csv|pdf&kode_klpd=&tahun=&cari=&pemisah=titik-koma|koma|tab`

- **Excel**: semua kolom, ditulis mengalir (tabel besar tidak dimuat ke memori), bilangan dan tanggal sungguhan, judul dibekukan, filter, maksimal 1.048.575 baris
  (lebih dari itu ditolak 422 sebelum data dibaca).
- **CSV**: UTF-8 dengan BOM, pemisah bawaan titik koma (Excel Indonesia), angka berdesimal titik. Sel teks yang diawali `= + - @` diberi apostrof
  (mencegah rumus berbahaya dari data luar).
- **PDF**: A4 lanskap, kolom ringkasan, maksimal 5.000 baris (keterangan bila terpotong).
- Kolom teknis (`row_key`, `extra_json`) tidak diekspor. Penyaring memakai parameter SQL; nama tabel dan kolom hanya dari katalog.

## Dasbor

`GET /api/v1/inaproc/analitik?tahun=&kode_klpd=&segarkan=1`. Tiap bagian dihitung terpisah (kegagalan satu bagian tidak menjatuhkan yang lain),
hasilnya di-cache 2 menit dan dikosongkan setiap antrean penarikan selesai. Bagian dihubungkan lewat **kode RUP**: tiap paket RUP aktif
(tidak dihapus, tidak nonaktif) dicocokkan dengan pengumuman tender, non-tender, atau paket e-purchasing V5/V6; yang cocok ke beberapa saluran
dihitung di saluran pertama (tender, non-tender, e-purchasing).

Wawasan dihasilkan `backend/analitik/wawasan.go` dari angka tersebut; semua ambangnya (konsentrasi satker 35%, triwulan IV 40%, HHI 1.500/2.500,
peserta tunggal 30%/50%, addendum 15%/30%, median proses 60 hari, efisiensi 2%/8%, dst.) ada di blok konstanta di awal berkas.

## Tabel

Migrasi `045_create_inaproc_penarikan.sql`: `inaproc_penarikan` (riwayat tugas) dan `inaproc_penarikan_pengaturan` (pengaturan otomatis).

## Pengujian

- `go test ./...` (tanpa database sungguhan; memakai `internal/fakesql`).
- Tes integrasi SQL Server (dilewati bila env kosong): `PASTI_UJI_MSSQL_DSN=sqlserver://...` ke database **khusus tes** yang migrasinya sudah diterapkan,
  lalu `go test ./handlers -run SQLServer`. Data uji memakai kode KLPD `UJI` dan dibersihkan sendiri. Memeriksa SQL dasbor, pembaca data/ekspor
  untuk ke-34 dataset, serta riwayat dan pengaturan penarikan terhadap skema asli.
- Frontend: `node --test lib/pengadaan.test.mjs` (Node 22.18+), `npx tsc --noEmit`.

## Asumsi dan batasan

- Belum diuji ke API Inaproc sungguhan (rate limit, bentuk respons, dan isi `status_*` bisa berbeda dari asumsi). Pola status pada wawasan
  "tender gagal" dan "belum terumumkan" memakai kecocokan teks (`gagal|batal|ulang|ditutup`, `umumkan`).
- `total` pada paket e-purchasing V6 dianggap nilai per order; `kd_rup` dianggap satu kode per baris.
- Dataset rujukan (penyedia, komoditas, distributor, produk penyedia) hanya terisi lewat penarikan per kode; nama di grafik E-Katalog muncul
  bila sudah ditarik, selebihnya kode.
- Dataset RUP/non-tender lama belum seatomik dataset generik (lihat di atas).
