# Datamart Digitalisasi Aset: tabel `DIGITALISASI_*` dibentuk dari SLDK, aplikasi hanya membaca datamart

Alur data Digitalisasi Aset:

```
tabel SLDK di data warehouse  --(script datamart, dijalankan tim Pusdatin)-->  dbo.DIGITALISASI_*  --(sinkronisasi aplikasi)-->  database PASTI: DIGITALISASI_*
```

**Aplikasi tidak pernah mengakses SLDK untuk Digitalisasi Aset.** Yang membaca tabel SLDK adalah satu script SQL, [`backend/datamart/buat_datamart_digitalisasi.sql`](../backend/datamart/buat_datamart_digitalisasi.sql),
yang dijalankan **tim Pusdatin di data warehouse** (bukan aplikasi): ia menjalankan ketujuh query asli Digitalisasi Aset terhadap tabel SLDK (`DJKN.SIMAN2_*`) yang sudah ada di data warehouse itu dan membentuk ketujuh tabel `DIGITALISASI_*` di sana,
dengan kolom, tipe, dan isi yang **sama dengan yang disimpan aplikasi saat sinkronisasi dahulu membaca SLDK langsung**. Sinkronisasi aplikasi lalu hanya menyalin tabel-tabel itu ke database PASTI.

## Menjalankan script (tim Pusdatin)

Script **hanya membentuk tabel**: tidak perlu mengubah apa pun bila tabel SLDK berada di database tempat script dijalankan dengan nama `DJKN.SIMAN2_M_ASET` dan seterusnya.

1. Buka database data warehouse yang berisi tabel SLDK (terpisah dari database aplikasi PASTI), buka [`buat_datamart_digitalisasi.sql`](../backend/datamart/buat_datamart_digitalisasi.sql) di SSMS (atau pakai `sqlcmd`).
2. Jalankan seluruh script. Selesai: tabel `DIGITALISASI_*` terbentuk dan terisi.
3. Untuk menyegarkan, jalankan script yang sama lagi (tabel tidak di-drop; hak akses tabel tetap). Jadwalkan di luar jam kerja, **sebelum** sinkronisasi PASTI (penjadwal aplikasi bawaan mulai 01.00-05.00 WIB tiap 7 hari), mis. pukul 23.00.

```bash
sqlcmd -S <server> -d <database_dwh> -b -i backend/datamart/buat_datamart_digitalisasi.sql
```

Bawaan script (`@Bersihkan = 1`): setelah berhasil, fungsi, prosedur, dan sinonim sementara dibuang, **yang tersisa hanya tabel**: tujuh `DIGITALISASI_*` dan dua tabel jejak kecil (`datamart_digitalisasi_run`, `datamart_digitalisasi_tabel`).
Bila lebih nyaman menjadwalkan satu perintah saja, ubah `@Bersihkan` menjadi `0`: rutin dipertahankan dan penyegaran berikutnya cukup `EXEC dbo.datamart_digitalisasi_segarkan;` (langkah Transact-SQL biasa di SQL Agent).
Bila penyegaran gagal, rutin tidak dibuang supaya penyebabnya dapat diperiksa.

**Bila tabel SLDK tidak ada di database ini dengan nama `DJKN.SIMAN2_*`**, script menolak dengan daftar tabel yang tidak terbaca beserta alasannya (sebelum membuat apa pun), mis.:

```
Tabel/view sumber SLDK berikut tidak dapat dibaca dari database ini. ...
  - DJKN.SIMAN2_M_ASET  ->  Invalid object name 'DJKN.SIMAN2_M_ASET'.
```

Ada dua cara menunjuk lokasi yang benar, keduanya hanya di bagian atas script:
- **Database lain** di server yang sama, atau linked server, dengan nama dan skema yang sama: isi `@Sumber`, mis. `N'[NamaDatabaseDWH]'` atau `N'[LinkedServer].[NamaDatabase]'`.
- **Nama atau skema berbeda**: di bagian *PEMETAAN SUMBER* tulis nama sebenarnya per tabel di bawah penanda yang tersedia, mis. `UPDATE @peta SET sumber = N'SLDK.SIMAN2_M_ASET' WHERE nama = N'SIMAN2_M_ASET';`. Query asli tetap tidak diubah; script membuat sinonim `DJKN.<nama>` hanya untuk tabel yang dipetakan ke tempat lain.

**Hak akses**: pengguna aplikasi di datamart cukup `GRANT SELECT ON dbo.DIGITALISASI_SATKER TO <pengguna>` dan seterusnya untuk ketujuh tabel, serta `SELECT` pada `dbo.datamart_digitalisasi_run`. Hak tabel **bertahan** saat penyegaran (isi diganti dengan `TRUNCATE` + `INSERT`, tabelnya tidak di-drop; diuji).
Yang menjalankan script butuh `SELECT` pada tabel SLDK yang dibaca query serta hak membuat tabel, skema, prosedur, dan fungsi di database itu. Syarat: SQL Server 2017 atau lebih baru (`STRING_AGG`, `TRIM`; tingkat kompatibilitas database 140+).

## Konfigurasi aplikasi

Di `backend/.env` (atau lewat `./deploy.sh --kredensial`), lalu mulai ulang backend:

| Variabel | Arti |
|---|---|
| `DATAMART_DB_HOST`, `DATAMART_DB_PORT`, `DATAMART_DB_USER`, `DATAMART_DB_PASSWORD`, `DATAMART_DB_NAME` | koneksi ke database datamart. Kosong = sinkronisasi Digitalisasi Aset tidak berjalan (halaman Sinkronisasi menjelaskannya) |

Variabel `SLDK_DB_*` **tidak dipakai** Digitalisasi Aset; ia tinggal dipakai penarikan referensi Kanwil. Variabel `DIGITALISASI_SUMBER` dari versi sebelumnya sudah dihapus (diabaikan, server mencatat peringatan bila masih terisi).
Migrasi 064 menambah kolom `digitalisasi_sync_log.data_per`; `./deploy.sh` menjalankannya.

## Perilaku sinkronisasi di aplikasi

- Per dataset, seluruh isi tabel datamart dibaca lalu menggantikan tabel PASTI dalam satu transaksi. Nilai disalin **apa adanya** (DECIMAL dan koordinat sebagai teks), tanpa konversi lagi. Struktur tabel datamart yang menyimpang
  (kolom hilang, tipe tak terduga seperti `FLOAT`) menghentikan dataset itu dengan pesan yang menyebut kolomnya; PASTI tidak berubah.
- **Hasil kosong** tidak menimpa tabel PASTI yang berisi. **Datamart yang lebih lama** daripada data PASTI saat ini (mis. dipulihkan dari cadangan) ditolak, berdasarkan "data per" (awal penyegaran datamart yang menulis baris-barisnya, dicatat di `digitalisasi_sync_log.data_per`).
  Sama dengan sinkronisasi sebelumnya (datamart belum disegarkan lagi) tetap berjalan dan riwayatnya ditandai.
- Kolom `Status_Gedung_Kantor` dan `Foto` di `DIGITALISASI_SATKER` (diisi manual di PASTI) dipertahankan per `Kode_Satker` bila datamart mengirim NULL.
- Halaman Sinkronisasi menampilkan sumber (datamart), "data per", dan peringatan bila datamart tak terjangkau, belum pernah disegarkan, penyegaran terakhirnya gagal (data yang disalin berasal dari penyegaran sukses sebelumnya), atau lebih dari 14 hari tidak disegarkan.
- Sinkronisasi otomatis dan manual berjalan seperti biasa ([digitalisasi-aset.md](digitalisasi-aset.md)); penjadwal hanya aktif bila datamart dikonfigurasi.

## Cara kerja script

- **Query asli apa adanya.** Query membaca `DJKN.SIMAN2_M_ASET` dan seterusnya langsung dari data warehouse (atau lewat *sinonim* bila lokasinya dipetakan ke tempat lain, lihat atas). Ketujuh query (`backend/digitalisasi/queries/*.sql`) dijalankan di dalam prosedur `dbo.datamart_dg_muat_<dataset>`
  tanpa diubah satu karakter pun; satu-satunya tambahan adalah `INTO #raw_<dataset>` pada `SELECT` terakhir. Tes membuktikan teks query di script sama dengan berkas sumbernya.
- **Konversi seperti aplikasi dahulu.** Hasil mentah query diubah oleh fungsi `dbo.dm_*` yang meniru implementasi acuan (`backend/digitalisasi/acuan`): spasi (semua spasi Unicode yang dipangkas Go) dipangkas, teks kosong menjadi `NULL`, teks dipotong sesuai panjang kolom (unit UTF-16, pasangan pengganti tidak dipecah),
  angka dengan koma desimal dibaca, bilangan bulat berpecahan menjadi `NULL`, koordinat harus berpasangan, di dalam rentang bumi, dan `(0,0)` dibuang. Kolom `FLOAT`/`REAL` di SLDK dibaca sebagai bilangan.
- **Tabel sama dengan migrasi 020.** `DIGITALISASI_*` datamart berkolom, bertipe, dan berindeks sama dengan tabel aplikasi (id identity, kolom data, `id_sinkron` = nomor proses penyegaran datamart, `synced_at`).
- **Aman bila gagal.** Ketujuh dataset dimuat dulu ke tabel staging (`DIGITALISASI_STG`); tabel hidup baru diganti, dalam **satu transaksi**, setelah semuanya berhasil. Bila apa pun gagal (kolom atau tabel SLDK hilang, akses ditolak, SLDK tak terjangkau, `@Sumber` salah), isi datamart yang lama **tidak berubah**.
  Hasil kosong tidak pernah menggantikan isi yang ada, proses ganda ditolak (kunci aplikasi), proses yang mati di tengah ditandai gagal pada proses berikutnya.
- **Terlihat jelas.** `dbo.datamart_digitalisasi_run` mencatat tiap proses (mulai/selesai UTC, `sukses`/`gagal`/`berjalan`, SLDK yang dibaca, pesan galat lengkap dengan prosedur dan barisnya); `dbo.datamart_digitalisasi_tabel` mencatat jumlah baris dan baris berkoordinat per tabel.
- **Tidak menyentuh yang lain.** SLDK hanya dibaca. Script menolak berjalan di database yang berisi `digitalisasi_sync_log` (database aplikasi PASTI), baik saat dipasang maupun saat disegarkan, dan memeriksa **sebelum membuat apa pun** bahwa semua tabel sumber terbaca. Tidak ada `GRANT`, pembuatan login/pengguna, atau pengaturan server.
- **Data pribadi**: `DIGITALISASI_RUMAH_NEGARA.Nama_Penghuni` adalah nama penghuni. Batasi akses ke datamart seperti ke database PASTI.

## Seberapa sama dengan data dari SLDK langsung

Pembuktian (tes integrasi terhadap SQL Server, SLDK palsu): **tabel acuan** dibentuk dari SLDK dengan implementasi acuan (ketujuh query asli + konversi acuan, paket `backend/digitalisasi/acuan`), lalu
(1) tabel hasil script datamart dibandingkan dengan tabel acuan, dan (2) tabel PASTI hasil sinkronisasi aplikasi yang sungguhan (membaca datamart; SLDK bahkan dihapus lebih dulu) dibandingkan dengan tabel acuan:
struktur, indeks, dan isi setiap kolom setiap baris, plus urutan `id` untuk query ber-`ORDER BY`. Fixture memuat kasus tepi: spasi Unicode, teks melebihi panjang kolom dengan emoji di batas potong, koma desimal, koordinat di luar rentang/tak berpasangan/`(0,0)`,
kolom sumber `FLOAT`/`REAL`. Ditambah baterai ribuan masukan acak untuk fungsi konversi (teks, bilangan bulat, desimal, koordinat) terhadap `acuan.KonversiNilai`.

**Sama persis** untuk semua data yang realistis. **Batas yang diketahui** (di luar batas ini hasilnya sedikit berbeda; tidak pernah muncul pada data SLDK yang wajar):

| Masukan | Acuan (aplikasi dahulu) | Datamart (dan PASTI sekarang) |
|---|---|---|
| Angka dengan **lebih dari 15 digit penting** (mis. nilai di atas Rp 9,99 triliun *beserta sen*) | dibaca sebagai `float64`, digit terakhir bisa bergeser | mempertahankan digit aslinya |
| Bilangan bulat di atas 2^53 (±9×10^15) | nilai `float64`-nya | nilai aslinya |
| Bentuk heksadesimal Go (`0x1p3`) pada kolom angka bertipe teks | terbaca | `NULL` |
| Kolom **teks** tujuan yang di SLDK bertipe `FLOAT`, tanggal, atau uang | — | **ditolak** dengan galat yang menyebut kolomnya (bentuk tekstualnya berbeda antara driver Go dan SQL Server); pada query sekarang tidak ada |

## Perawatan: bila query atau kolom Digitalisasi Aset diubah

Script **dihasilkan** dari `backend/digitalisasi` (`datasets.go` dan `queries/*.sql`) oleh `backend/datamart/digitalisasi.go`; jangan disunting manual. Setelah mengubah kolom atau query:

```bash
cd backend
go test ./datamart -run TestSkripDigitalisasiSesuaiGenerator -update
```

lalu pasang ulang script di datamart. Tes biasa (`go test ./...`) gagal bila script tidak sejalan dengan aplikasi. Tes statis juga memeriksa: query asli utuh, konversi tiap kolom sesuai jenis dan panjangnya, struktur tabel sama dengan migrasi 020, SLDK hanya dibaca,
daftar sinonim sama dengan objek yang dibaca query, urutan muat sesuai `ORDER BY`, dan **bahwa kode server tidak mengimpor paket acuan atau menyebut koneksi SLDK** (pagar di `backend/digitalisasi/acuan`).

Tes integrasi (perlu hak `CREATE DATABASE`; membuat dan menghapus database `uji_dm_*`; dilewati bila `PASTI_UJI_MSSQL_DSN` kosong):

```bash
cd backend
PASTI_UJI_MSSQL_DSN='sqlserver://localhost:1433?database=master&authenticator=winsspi&encrypt=disable' go test ./datamart/ -run 'Paritas|DatamartDigitalisasi|Aplikasi' -v
```

Jangan menjalankan `go test ./...` dengan DSN itu: tes integrasi paket lain mengharapkan database yang sudah dimigrasi. Uji skala opsional: `PASTI_UJI_BARIS=100000 go test ./datamart/ -run SkalaBesar -v`.

## Yang belum diverifikasi terhadap SLDK asli

Semua pengujian memakai SLDK palsu di SQL Server lokal (tipe kolomnya perkiraan); script belum pernah dijalankan di data warehouse Pusdatin atau terhadap SLDK asli.

- **Linked server**: bila tabel SLDK dibaca lewat linked server, query yang menggabungkan banyak tabel besar bisa lambat. Periksa lama proses pada percobaan pertama; bila terlalu berat, jalankan script di server yang sama dengan tabel SLDK (data warehouse).
- **Lama proses**: tiap query dijalankan sekali, lalu setiap nilai dikonversi oleh fungsi skalar (puluhan mikrodetik per nilai; SQL Server 2019+ menyisipkan fungsi itu ke query dan lebih cepat). Uji skala di mesin pengembang dengan SLDK palsu:
  100 ribu baris tanah tambahan selesai dalam kira-kira 1,5 sampai 2 menit di luar waktu query SLDK, dengan isi identik dengan hasil acuan (jumlah baris dan checksum). Waktu query asli terhadap SLDK 206 GB tidak terukur di sini; ukur pada percobaan pertama.
- **Tipe dan *collation*** kolom SLDK diasumsikan seperti pada query asli. Bila ada kolom teks tujuan yang di SLDK bertipe `FLOAT`/tanggal/uang, script berhenti dengan galat yang menyebut kolomnya.
- **Penjadwalan**: penyegaran tabel oleh Pusdatin dan sinkronisasi PASTI (penjadwal aplikasi) berjalan terpisah; atur agar penyegaran tabel selesai sebelum jendela sinkronisasi PASTI.
- **Nama tabel di data warehouse** diasumsikan `DJKN.SIMAN2_*` seperti di SLDK; bila berbeda, gunakan pemetaan di atas (script menyebut tabel mana yang tidak terbaca).
