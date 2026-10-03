# SAPA (Sistem Administrasi Pengelolaan Aset): Penjualan

Menu **SAPA** menyusun administrasi pengelolaan BMN. Saat ini baru modul **Penjualan** (`/dashboard/sapa/penjualan`);
Sewa dan modul lain menyusul. Pengguna melacak satu **usulan penjualan** sepanjang 10 tahap dari Satuan Kerja (Satker),
Kantor Wilayah (Kanwil), sampai Unit Eselon I (UE1). Dokumen Word (SK Tim, Berita Acara, Nota Dinas) **disusun otomatis dari
isian formulir** lalu diunduh dari aplikasi.

## Alur 10 tahap

| # | Peran | Tahap | Cara dikerjakan |
|---|---|---|---|
| 1 | Satker | Pembentukan Tim | Formulir, hasil: SK Tim (Word). Boleh dilewati bila dibuat di luar aplikasi |
| 2 | Satker | Penyusunan Berita Acara Penelitian | Formulir, hasil: Berita Acara (Word). Boleh dilewati |
| 3 | Satker | Penyusunan konsep ND Usulan Penjualan Satker | Formulir, hasil: satu berkas Word berisi Nota Dinas, daftar barang, checklist, dan surat pernyataan |
| 4 | Satker | Penetapan ND Usulan Satker | Di **Nadine**; aplikasi mencatat nomor + tanggal (wajib) |
| 5 | Satker | Pembuatan tiket SIMAN + upload dokumen | Di **SIMAN**; aplikasi hanya mencatat selesai |
| 6 | Kanwil | Penelitian tiket SIMAN | Di **SIMAN**; dicatat |
| 7 | UE1 | Menerima usulan + penelitian | Di **Nadine dan SIMAN**; dicatat |
| 8 | UE1 | Penyusunan konsep ND Usulan Penjualan UE1 | Formulir, hasil: Nota Dinas UE1 (Word) |
| 9 | UE1 | Penetapan ND UE1 | Di **Nadine**; dicatat |
| 10 | UE1 | Penerusan tiket SIMAN | Di **SIMAN**; dicatat |

Aturan tahap:

- Tahap hanya bisa dikerjakan setelah **semua tahap sebelumnya selesai atau dilewati**.
- Tahap yang sudah selesai boleh diubah/dibuat ulang selama **belum ada tahap sesudahnya yang selesai** (agar dokumen tahap
  berikutnya tidak berselisih dengan data baru). Dokumen lama tetap tersimpan sebagai riwayat. Admin dapat **membuka ulang** tahap.
- Hanya tahap 1 dan 2 yang boleh dilewati, dan alasan (mis. nomor dan tanggal dokumen buatan luar aplikasi) wajib dicatat.
- Usulan punya **Noreg** (`PJ-<tahun>-<5 digit>`, mis. `PJ-2026-00001`). Noreg inilah "nomor tiket"/"Noreg aplikasi" pada
  dokumen; "Nomor tiket SIMAN" adalah isian terpisah di formulir ND Satker.

## Peran SAPA dan hak akses

Peran SAPA **terpisah** dari peran aplikasi (user/admin/superadmin) dan ditetapkan admin per pengguna di
**Pengaturan SAPA → Peran pengguna**:

| Peran | Cakupan | Boleh |
|---|---|---|
| Satuan Kerja | satu kode satker (18 digit) | Membuat usulan untuk satkernya, mengerjakan tahap 1-5, melihat usulan satkernya |
| Kantor Wilayah | semua usulan (versi awal; kode wilayah belum ada di data) | Mengerjakan tahap 6 |
| Unit Eselon I | satu kode UE1 (5 digit) | Mengerjakan tahap 7-10, melihat usulan satker di bawahnya |
| Admin/superadmin (peran aplikasi) | semua | Semua tahap, membuka ulang tahap, Pengaturan SAPA |

- Pengguna **tanpa peran** (dan bukan admin) melihat penjelasan "belum dapat memakai SAPA", bukan galat.
- Usulan yang tidak boleh dilihat dijawab **404** (bukan 403) supaya keberadaannya tidak bocor.
- Kode UE1 usulan = 5 digit pertama kode satker. Kode satker di data aset berakhiran `KP`; aplikasi memakai 18 digit pertamanya.

## Template Word

Template **diunggah admin** (Pengaturan SAPA → Template dokumen) dan disimpan di database dengan versi; versi terbaru
langsung dipakai. Dua template bawaan (T01 ND Satker, T02 ND UE1) ada di `backend/sapa/templates/` dan dipakai bila belum ada
unggahan. **SK Tim dan Berita Acara belum punya template bawaan**: sebelum diunggah admin, tahap itu hanya bisa menyimpan draf
atau dilewati.

Saat unggah, aplikasi:

1. memeriksa berkas (.docx, maksimal 5 MB, bukan zip bom);
2. **menguji pengisian** dengan data contoh; template yang strukturnya tidak sesuai (mis. tabel Daftar Barang hilang) ditolak;
3. melaporkan penanda yang **tidak dikenal** (dibiarkan apa adanya) dan penanda yang **dikenal tetapi tidak ada** di template.

### Penanda

Tulis `<<nama penanda>>` di dokumen. Penanda boleh terpecah di beberapa "run" Word (mis. sebagian tebal), format run pertama
dipertahankan. Aturan khusus:

- `[@NomorND]`, `[@TanggalND]` tidak disentuh (diisi Nadine).
- `(<<singkatan>>)` yang nilainya kosong dihapus bersama kurungnya.
- Tabel **Daftar Barang** (diikuti judul kolom "Nama Barang"): baris contoh digandakan per barang, baris JUMLAH diisi total.
- Tabel **Checklist**: setiap baris dikenali dari nama dokumennya, kolom hasil diisi "Ada" / "Tidak ada" beserta nomor dan tanggal.
- SK Tim: baris tabel yang memuat `<<nama anggota>>` diulang per anggota (atau pakai `<<daftar anggota>>`).

Daftar lengkap penanda per jenis dokumen tampil di Pengaturan SAPA dan bersumber dari `backend/sapa/jenis.go` (diuji agar
sejalan dengan template bawaan).

## Referensi UE1

Kode UE1 (5 digit) → nama dan sebutan Sekretaris, dipakai mengisi **tujuan Nota Dinas**. Diisi admin di Pengaturan SAPA →
Referensi UE1. Tidak ada data awal supaya tidak ada nama yang keliru; bila belum diisi, pengguna mengetik tujuan manual.

## Yang dihitung otomatis

- **Jumlah BMN, total nilai perolehan, total nilai limit** dihitung dari daftar barang (tidak diinput), beserta terbilangnya
  ("Tiga Miliar Lima Ratus … Rupiah Lima Puluh Sen"), supaya angka di surat selalu sama dengan tabel lampiran.
- Hari, tanggal, bulan, dan tahun pada Berita Acara dibentuk dari tanggal penelitian.
- Nilai uang boleh diketik `1500000`, `1500000.50`, atau `1.500.000,50` (paling banyak dua desimal). Tabel daftar barang bisa
  **ditempel dari Excel** (kolom: Nama, Kode, NUP, Lokasi/Merk/Tipe, Kondisi, Tahun, Nilai Perolehan, Nilai Limit, Keterangan).
- Saran isian: tujuan surat dari referensi UE1; kota dari data satker (awalan "KOTA ADM."/"KAB." dibuang); nomor dan tanggal
  ND UE1 dari catatan Nadine tahap 4; data penandatangan tidak ditebak (diketik pengguna).

## Tabel database (migrasi `021_create_sapa.sql`)

| Tabel | Isi |
|---|---|
| `sapa_peran` | peran SAPA per pengguna (+ kode satker / kode UE1) |
| `sapa_ref_ue1` | referensi UE1 |
| `sapa_template` | template Word berversi (`VARBINARY`), satu versi aktif per jenis |
| `sapa_urutan` | pencacah Noreg per tahun |
| `sapa_penjualan` | usulan penjualan |
| `sapa_penjualan_tahap` | status, isian (JSON), nomor/tanggal/catatan tiap tahap |
| `sapa_dokumen` | dokumen Word hasil (riwayat semua versi) |

Nama satker dan UE1 pada formulir usulan baru dicari di `DIGITALISASI_SATKER` (data Digitalisasi Aset); bila belum disinkronkan,
pembuat usulan mengetik nama satker manual.

## Struktur kode

| Lokasi | Isi |
|---|---|
| `backend/sapa/docx` | mesin dokumen Word (penggantian penanda lintas run, operasi tabel), tanpa dependensi aplikasi |
| `backend/sapa` | alur tahap dan hak akses (`tahap.go`), data + validasi (`data.go`), pengisi dokumen (`isi.go`), aturan bisnis (`layanan.go`), SQL (`store.go`), versi memori untuk tes (`memrepo.go`) |
| `backend/handlers/sapa_handler.go`, `backend/routes/sapa_routes.go` | HTTP `/api/v1/sapa/...` |
| `frontend/lib/sapa.ts`, `frontend/components/sapa/` | tipe, API, dan halaman |

Rute: `GET /sapa/saya`, `GET /sapa/referensi/satker`, `GET|POST /sapa/penjualan`, `GET /sapa/penjualan/:id`,
`PUT /sapa/penjualan/:id/tahap/:kunci` (draf), `POST .../dokumen`, `.../selesai`, `.../lewati`, `.../buka-ulang` (admin),
`GET /sapa/dokumen/:id/unduh`; admin: `/sapa/template`, `/sapa/peran`, `/sapa/ref-ue1`.

## Pengujian

```bash
cd backend && go test ./sapa/... ./routes/ ./handlers/
cd frontend && node --test components/sapa/sapa.test.mjs
```

- `backend/sapa`: alur tahap, hak akses, validasi, pengisian template asli, aturan bisnis (penyimpanan memori), dan SQL `Store`
  (driver palsu: urutan transaksi, jumlah parameter, escape `LIKE`).
- `backend/routes/sapa_test.go`: HTTP dengan tabel rute asli (`RegisterSapa`): hak akses per peran, 404 vs 403, alur lengkap
  sampai unduhan, unggah template (multipart), batas ukuran, dan galat internal yang tidak bocor.
- `frontend/components/sapa/sapa.test.mjs`: fungsi murni (nilai uang yang sama dengan backend, normalisasi isian, tempel dari Excel).

## Menjalankan pertama kali

1. `./deploy.sh` (migrasi 021 berjalan otomatis).
2. Admin membuka **Pengaturan SAPA**: isi **Referensi UE1**, unggah template SK Tim dan Berita Acara (bila ingin dibuat di
   aplikasi), lalu tetapkan **peran pengguna** (Satker + kode satker, Kanwil, UE1 + kode UE1).
3. Pengguna Satker membuka SAPA → Penjualan → *Buat usulan penjualan*.

## Yang belum diverifikasi / batasan

- Seluruh SQL diuji dengan database **palsu** dan penyimpanan memori; belum dijalankan ke SQL Server asli (sintaks `MERGE`,
  `UPDATE ... WITH (UPDLOCK, SERIALIZABLE)`, indeks unik terfilter, `OFFSET/FETCH` perlu diperiksa saat migrasi pertama).
- Dokumen hasil diperiksa lewat isi XML-nya (penanda terganti, tabel terisi) dan dibuka ulang oleh pustaka .docx; **belum dibuka
  di aplikasi Microsoft Word** (tidak ada di lingkungan pengembangan). Buka satu hasil di Word untuk memastikan tampilannya.
- `<<jumlah bmn>>` pada template mencetak angka saja (mis. "3"); satuan hanya muncul lewat `<<terbilang jumlah bmn>>` bila satuan diisi.
- Template T02 (ND UE1) memuat salah ketik "Pengadaab" pada tembusan; template tidak diubah aplikasi, perbaiki di berkas Word lalu unggah ulang.
- Peran Kanwil melihat **semua** usulan karena kode wilayah belum ada pada data satker.
- Tidak ada pengiriman email: dokumen diunduh dari aplikasi.
- Hanya modul Penjualan; Sewa dan lainnya belum ada.
