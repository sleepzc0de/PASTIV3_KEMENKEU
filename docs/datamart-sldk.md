# Datamart SLDK untuk Digitalisasi Aset

Sinkronisasi [Digitalisasi Aset](digitalisasi-aset.md) membaca SLDK (database `Interchange`, tabel aset sekitar 206 GB). Tanpa cadangan, bila SLDK tidak dapat dihubungi atau akses ditolak, sinkronisasi gagal (data PASTI yang sudah ada tetap aman,
tetapi tidak bisa diperbarui). **Datamart** adalah salinan kecil dari data SLDK yang dibutuhkan ketujuh query Digitalisasi Aset, disimpan di database terpisah dengan **nama skema, tabel, dan kolom yang sama persis dengan SLDK**
(skema `DJKN`, tabel `SIMAN2_*`). Karena namanya sama, query Digitalisasi Aset berjalan **tanpa diubah** terhadap datamart: aplikasi membaca datamart otomatis bila SLDK gagal dibaca (lihat *Fallback otomatis di aplikasi*), atau Anda dapat mengarahkan `SLDK_DB_*` ke datamart.

Script: [`backend/datamart/buat_datamart_sldk.sql`](../backend/datamart/buat_datamart_sldk.sql). Script ini dijalankan administrator database (bukan oleh aplikasi) dan **hanya membaca SLDK**.

## Memasang dan menyegarkan

Jalankan di server datamart, pada database datamart (disarankan database tersendiri, mis. `pasti_datamart`; jangan di database yang sama dengan SLDK asli). `SUMBER` adalah awalan nama database SLDK yang dapat dibaca dari server itu:

```bash
# SLDK ada di server yang sama dengan datamart
sqlcmd -S <server> -d pasti_datamart -b -i backend/datamart/buat_datamart_sldk.sql -v SUMBER="[Interchange]"

# SLDK di server lain, lewat linked server
sqlcmd -S <server> -d pasti_datamart -b -i backend/datamart/buat_datamart_sldk.sql -v SUMBER="[LNK_SLDK].[Interchange]"
```

- Menjalankan berkas ini **memasang** dua prosedur (`dbo.datamart_sldk_muat`, `dbo.datamart_sldk_segarkan`; `SUMBER` tertanam di dalamnya) lalu langsung menjalankan penyegaran pertama, yang **membuat** tabel-tabelnya.
- **Penyegaran berikutnya**: `EXEC dbo.datamart_sldk_segarkan;`. Jadwalkan sebagai langkah *Transact-SQL* biasa di SQL Agent (tanpa sqlcmd dan tanpa variabel), di luar jam kerja dan kira-kira sama seringnya dengan
  sinkronisasi PASTI (bawaan mingguan). Mengganti `SUMBER`: jalankan ulang berkas dengan nilai baru.
- Tabel dibangun dulu di skema `DJKN_STG`, divalidasi, lalu **ditukar** ke `DJKN` dalam satu transaksi singkat. Pembaca tidak pernah melihat tabel setengah jadi, dan bila apa pun gagal, datamart yang lama **tidak berubah**.
- Hasil kosong tidak pernah menggantikan datamart (0 satker atau 0 aset ditolak), proses ganda ditolak (kunci aplikasi), dan sisa proses yang terhenti dibersihkan pada proses berikutnya.
- Syarat: SQL Server 2016 atau lebih baru (query memakai `STRING_AGG`, jadi datamart juga butuh tingkat kompatibilitas 140+); hak baca ke tabel SLDK di bawah; di datamart hak membuat tabel/skema/prosedur dan `ALTER` pada skema `DJKN`.
- Beri hak baca kepada pengguna aplikasi di **tingkat skema**: `GRANT SELECT ON SCHEMA::DJKN TO <pengguna>`. Izin tingkat tabel hilang saat tabel ditukar; izin tingkat skema bertahan (diuji).

## Jejak dan kesegaran

```sql
SELECT TOP 5 * FROM dbo.datamart_sldk_run ORDER BY id DESC;                 -- mulai, selesai (UTC), status berjalan/sukses/gagal, pesan galat
SELECT * FROM dbo.datamart_sldk_tabel WHERE run_id = <id>;                  -- jumlah baris per tabel pada proses itu
SELECT MAX(mulai) FROM dbo.datamart_sldk_run WHERE status = N'sukses';     -- data datamart per kapan (awal penyegaran terakhir yang berhasil); dibaca aplikasi (UTC)
```

Galat dari SLDK (tabel atau kolom hilang karena skema berubah, `SUMBER` salah, akses ditolak) tercatat pada `pesan` dengan nama prosedur dan barisnya. Setelah SLDK mengubah skema, penyegaran terjadwal gagal dengan pesan itu dan datamart lama tetap dipakai.

## Fallback otomatis di aplikasi

Aplikasi membaca datamart **sendiri** bila SLDK gagal dibaca; tidak perlu mengubah `SLDK_DB_*`. Isi di `backend/.env` (lalu mulai ulang backend):

| Variabel | Arti |
|---|---|
| `DATAMART_DB_HOST`, `DATAMART_DB_PORT`, `DATAMART_DB_USER`, `DATAMART_DB_PASSWORD`, `DATAMART_DB_NAME` | koneksi ke database datamart. Kosong = tanpa cadangan. Pengguna ini cukup berhak `SELECT` pada skema `DJKN` dan tabel `dbo.datamart_sldk_run` |
| `DIGITALISASI_SUMBER` | `otomatis` (bawaan): SLDK dulu, datamart bila SLDK gagal dibaca. `sldk`: hanya SLDK (datamart diabaikan). `datamart`: hanya datamart, mis. saat SLDK sengaja tidak dipakai. Nilai lain dianggap `otomatis` |

**Kapan beralih ke datamart** (mode `otomatis`):

- Hanya bila *membaca* dari SLDK gagal: tidak dapat terhubung, query ditolak (akses, izin), atau sambungan terputus di tengah. Pembatalan oleh pengguna, batas waktu, galat data (mis. kolom hasil query hilang atau melebihi batas baris), dan galat menulis ke PASTI **tidak** memicu peralihan.
- **Hanya bila datamart lebih baru daripada sinkronisasi sukses terakhir dataset itu di PASTI** (awal penyegaran datamart dibandingkan dengan awal sinkronisasi sukses terakhir, pada presisi detik). Datamart yang lebih tua tidak boleh memundurkan data PASTI yang sudah lebih baru: sinkronisasi dicatat *gagal*
  dengan alasannya ("datamart tidak dipakai: ... lebih lama daripada data PASTI saat ini"). Dataset yang belum pernah disinkronkan memakai datamart yang mana pun. Kesegaran yang tidak dapat diperiksa (tabel jejak tidak terbaca, atau datamart belum pernah berhasil disegarkan) juga menolak pemakaian.
- Bila *sambungan* ke SLDK gagal, dataset berikutnya pada antrean yang sama langsung ke datamart tanpa menunggu SLDK lagi. Bila hanya satu query yang gagal, dataset berikutnya tetap mencoba SLDK lebih dulu.
- SLDK yang mati saat server dimulai tidak lagi mematikan sinkronisasi sampai restart berikutnya: koneksi SLDK dicoba lagi saat sinkronisasi berjalan.
- Mode `sldk` dan `datamart` adalah pilihan sengaja operator: `sldk` tidak pernah menyentuh datamart, `datamart` tidak pernah menyentuh SLDK dan tanpa pemeriksaan "lebih baru" (kesegarannya tetap dicatat di pesan riwayat).

**Yang terlihat**

- Riwayat (`digitalisasi_sync_log.sumber`, migrasi 063) mencatat `sldk` atau `datamart` untuk tiap sinkronisasi; halaman Sinkronisasi menandai yang dari datamart dengan lencana *Datamart* dan menulis di pesan "dari datamart, data per <waktu>; SLDK tidak dapat dihubungi" (tanpa teks galat mentah yang bisa memuat nama server;
  teks lengkapnya ada di log server dengan awalan `[DIGITALISASI WARN]`).
- Halaman Sinkronisasi menampilkan keadaan sumber: cadangan aktif beserta kesegaran datamart, peringatan bila datamart tidak terjangkau atau belum pernah disegarkan, dan peringatan bila datamart lebih dari 14 hari tidak disegarkan.
- Sinkronisasi yang berhasil dari datamart bersifat **sementara**: penjadwal mencobanya lagi setelah 20 jam (bukan 7 hari), supaya data kembali dari SLDK begitu SLDK pulih. Bila SLDK masih mati dan datamart belum disegarkan, percobaan itu dicatat gagal tanpa mengubah data (pengaman di atas).
- Sinkronisasi otomatis berjalan bila ada sumber data apa pun (SLDK atau datamart).

**Penyegaran datamart tetap tugas terjadwal** (SQL Agent, lihat atas): fallback hanya sebaik penyegaran terakhir datamart. Fitur lain yang membaca SLDK (penarikan referensi Kanwil, pemantauan) tidak memakai datamart.

## Data yang disalin

Hanya kolom yang dipakai query, dan hanya baris yang dibutuhkan. Semua snapshot `TGL_TARIK` aset inti dipertahankan (beberapa query memilih snapshot terbaru sendiri). Tipe dan *collation* kolom mengikuti SLDK (tabel dibuat dengan `SELECT ... INTO`).

| Tabel | Baris yang disalin |
|---|---|
| `SIMAN2_R_SATKER` | `kd_satker` berawalan 015, semua status |
| `SIMAN2_R_KONDISI` | semua |
| `SIMAN2_R_STATUS_HUKUM_NEW` | hanya snapshot `TGL_TARIK` terbaru |
| `SIMAN2_M_ASET` | **aset inti**: `status_bmn_yn = Y`, satker 015, dan kode barang tanah (`2%`), gedung/bangunan (`40101%`), rumah negara (`4010201%`, `4010202%`, `4010209%`), rusunara (`4010208%`), atau 10 kode kendaraan dinas. Ditambah **aset tautan**: aset berstatus Y (satker atau jenis apa pun) yang terhubung lewat `SIMAN2_M_ASET_TANAH_BANGUNAN` dengan aset inti, karena query membaca sisi lain (tanah gedung kantor dapat milik satker di luar 015; bangunan di atas tanah dihitung sebagai `Jumlah_Bangunan`) |
| `SIMAN2_M_ASET_TANAH_BANGUNAN` | tautan yang salah satu sisinya aset inti |
| `SIMAN2_M_ASET_PHOTO` | `status_data = 1`, aset inti |
| `SIMAN2_T_INV_BANGUNAN` | aset inti berkode `4%` |
| `SIMAN2_PENGELOLAAN` | `kode_register` aset inti berkode `4%` (status asuransi) |
| `SIMAN2_M_ASET_HUKUM` | `status_data = 1`, aset inti berkode `4%` |
| `SIMAN2_M_ASET_PEMAKAI` | **hanya rumah negara** (selain mess `4010202016`), persis kelompok yang dibaca `rumah_negara.sql` |
| `VW_SIMAN_RUANG_GEDUNG_BANGUNAN_015` | baris yang cocok (satker, kode barang, NUP) dengan aset inti berkode `4%`. Di SLDK ini *view*; di datamart menjadi tabel bernama sama |

**Keamanan**: `SIMAN2_M_ASET_PEMAKAI.nm_pmk` adalah nama penghuni rumah negara (data pribadi, sama dengan `DIGITALISASI_RUMAH_NEGARA.Nama_Penghuni`; lihat bagian *Data pribadi* di [digitalisasi-aset.md](digitalisasi-aset.md)), dan tabel foto memuat jalur berkas internal.
Perlakukan database datamart seperti database PASTI: batasi siapa yang boleh membacanya. Script tidak mengubah SLDK, izin, pengguna, atau pengaturan server (dijaga tes).

## Perawatan: bila query Digitalisasi Aset diubah

Daftar kolom dan kriteria baris ada di script. Tes `backend/datamart/datamart_test.go` (tanpa database, ikut `go test ./...`) membaca ketujuh query dan **gagal** bila query:

- memakai tabel atau kolom SLDK yang tidak disalin script (atau script menyalin yang tidak dipakai);
- menyaring kelompok kode barang, awalan satker, atau status yang tidak tercakup kriteria aset inti;
- memakai kendaraan dengan kode yang tidak ada di daftar script, atau mengubah kelompok rumah negara tanpa menyesuaikan penyaring pemakai.

Perbarui script lalu jalankan ulang tesnya. Ubahan pada script dipasang ke datamart dengan menjalankan ulang berkas itu.

Tes integrasi (`datamart_integrasi_test.go`; dilewati bila `PASTI_UJI_MSSQL_DSN` kosong) membuat dua database sementara `uji_dm_*`, memuat SLDK palsu (`testdata/sldk_palsu.sql`), memasang script, dan membuktikan bahwa **hasil ketujuh query dari datamart identik dengan dari SLDK** (termasuk aset tautan, snapshot ganda, view, dan pemakai),
serta jalur gagal (kolom SLDK hilang setelah terpasang, SLDK kosong, proses ganda) yang tidak merusak datamart. Perlu hak `CREATE DATABASE`:

```bash
cd backend
PASTI_UJI_MSSQL_DSN='sqlserver://localhost:1433?database=master&authenticator=winsspi&encrypt=disable' go test ./datamart/ -v
```

## Yang belum diverifikasi terhadap SLDK asli

Semua pengujian memakai SLDK palsu di SQL Server lokal (tipe kolomnya perkiraan); script belum pernah dijalankan ke `Interchange`, dan fallback otomatis belum pernah dicoba dengan SLDK yang benar-benar mati di jaringan Kemenkeu
(diuji dengan port yang ditolak dan galat tersuntik: koneksi gagal, query ditolak, kolom hilang, SLDK kosong, pembatalan). Tes integrasi fallback (`backend/datamart/fallback_integrasi_test.go`) menjalankan Manager dan Sync sungguhan terhadap SQL Server. Yang masih perlu diperiksa di lingkungan asli:

- **Linked server**: bila SLDK di server lain (`10.216.121.1`), filter berbasis subquery lokal pada tabel anak (foto, inventaris, hukum, dan sebagainya) dapat membuat SQL Server menarik seluruh tabel anak dari jauh sebelum menyaring. Periksa rencana eksekusi dan lama proses pada percobaan pertama; bila terlalu berat,
  jalankan script di server yang sama dengan SLDK (database terpisah) atau pakai pemuat berbasis aplikasi (pola sinkronisasi yang sudah ada, tanpa linked server).
- **Lama proses dan ukuran**: tabel aset SLDK dibaca dua kali (aset inti, lalu aset tautan); hasil antara ditahan di `tempdb`.
- **Tipe, *collation*, dan nilai** kolom SLDK (mis. `status_data`, `photo_utama_yn`, `kd_jns_bmn`, `no_aset` vs `NUP`) diasumsikan sama dengan yang dipakai query asli; perbandingan di atas menjamin kesetaraan hanya untuk data uji.
- Indeks pada tabel datamart dibuat dengan perkiraan kolom kunci; bila sebuah indeks gagal dibuat, script hanya memperingatkan (datamart tetap benar, hanya lebih lambat).
