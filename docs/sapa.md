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
  berikutnya tidak berselisih dengan data baru). Dokumen lama tetap tersimpan sebagai riwayat. Superadmin dapat **membuka ulang** tahap.
- Hanya tahap 1 dan 2 yang boleh dilewati. Di kartu tahapnya ada kotak centang **"... sudah dibuat di luar aplikasi"**; bila dicentang,
  formulir diganti isian keterangan dokumen (mis. nomor dan tanggal SK, minimal 5 karakter) lalu tombol *Simpan dan lewati tahap*.
  Membatalkan centang mengembalikan formulir dengan isiannya utuh. Tahap yang sudah dilewati masih bisa dibuat di aplikasi lewat
  tombol *Buat dokumen di aplikasi*.
- Usulan punya **Noreg** (`PJ-<tahun>-<5 digit>`, mis. `PJ-2026-00001`). Noreg inilah "nomor tiket"/"Noreg aplikasi" pada
  dokumen; "Nomor tiket SIMAN" adalah isian terpisah di formulir ND Satker.

## Peran SAPA dan hak akses

SAPA **tidak menetapkan peran sendiri**. Peran pengguna di SAPA adalah **peran data aplikasi yang sedang aktif** (lihat [peran-data.md](peran-data.md)):
superadmin atau Pengguna Barang memberinya di **Manajemen Pengguna** (ikon perisai), dan pengguna dengan beberapa peran memilihnya lewat pemilih peran di menu pengguna. Semua
peran data memakai kunci kode satker lengkap yang sama: 5 karakter pertama = UE1, 9 karakter pertama = Kanwil, karakter ke-10 sampai ke-15 = satker.

| Peran aplikasi | Cakupan usulan | Boleh di SAPA |
|---|---|---|
| Satker | kode satker 6 digit (karakter ke-10 sampai ke-15 kode satker usulan; induk dan anak satker sama-sama terhitung) | Membuat usulan untuk satkernya, mengerjakan tahap 1-5, melihat usulan satkernya |
| Kanwil | kode Kanwil 9 digit (9 karakter pertama kode satker usulan) | Mengerjakan tahap 6, melihat usulan di bawah Kanwil-nya |
| UE1 | kode UE1 5 digit | Mengerjakan tahap 7-10, melihat usulan satker di bawahnya |
| Pengguna Barang | semua usulan | **Hanya melihat**: tidak punya tahap dan tidak membuat usulan (keputusan awal; ubah di `sapa/tahap.go` bila perlu) |
| Super Admin (akun, dari `.env`) | semua | Semua tahap, membuka ulang tahap, Pengaturan SAPA |

- Pengguna **tanpa peran data** adalah tamu: seluruh aplikasi, termasuk SAPA, tertutup baginya sampai diberi peran (lihat [akses-pengguna.md](akses-pengguna.md)).
- Superadmin yang sedang **bertindak sebagai peran data** (memilih peran itu di pemilih peran) diperlakukan sebagai peran itu di SAPA, bukan superadmin.
- Usulan yang tidak boleh dilihat dijawab **404** (bukan 403) supaya keberadaannya tidak bocor.
- Kode UE1 usulan = 5 digit pertama kode satker. Kode satker di data aset berakhiran `KP`; aplikasi memakai 18 digit pertamanya. Bagi peran Satker, kode lengkap
  untuk usulan baru diisi dari data Digitalisasi Aset (induk lebih dulu; bila ada beberapa anak satker, dipilih dari daftar). Bila satkernya belum ada di data aset,
  kodenya diketik dan backend memastikan karakter ke-10 sampai ke-15 sama dengan kode perannya.
- Tabel `sapa_peran` (penetapan lama) **tidak dibaca lagi**. Migrasi `054_sapa_peran_ke_peran_aplikasi.sql` menyalin isinya ke `user_roles` agar pengguna SAPA yang ada
  tidak kehilangan akses: Satker (kode satker 18 digit menjadi 6 digit), UE1 (kode apa adanya), dan Kanwil **hanya bila** pegawainya punya kode satker SSO yang sah (9
  karakter pertamanya menjadi kode Kanwil; `sapa_peran` tidak menyimpan kode Kanwil). Kanwil tanpa kode SSO harus diberi peran Kanwil oleh superadmin atau Pengguna Barang.

## Anggota tim dari HRIS2

Pada tahap Pembentukan Tim, setiap anggota punya kotak **Cari pegawai di HRIS2** (nama atau NIP, minimal 3 huruf/angka tanpa
menghitung spasi). Memilih hasilnya mengisi **nama (dengan gelar), jabatan aktif, dan NIP** otomatis; kedudukan diisi pengguna
(saran: Ketua, Sekretaris, Anggota) dan semua bidang tetap bisa diubah.

- Memakai **sesi SSO pengguna** (sama dengan fitur HRIS2 di halaman superadmin). Akun lokal atau sesi SSO yang berakhir mendapat
  penjelasan di kotak pencarian dan tetap bisa mengisi manual.
- Backend (`GET /sapa/pegawai?q=` dan `GET /sapa/pegawai/:nip`, khusus pengguna yang sudah punya akses SAPA) **hanya meneruskan NIP,
  nama, jabatan, dan satuan kerja**. Tanggal lahir, kontak, dan data pribadi lain tidak pernah diteruskan.
- Kata kunci dibersihkan di backend (karakter khusus filter HRIS2 seperti koma, `|`, kurung, dan `\` dibuang) dan hasil dibatasi 15.
- Jabatan aktif hanya pasti ada pada detail pegawai, jadi setelah memilih hasil, aplikasi mengambil detailnya; bila jabatan tidak
  ada, kolom jabatan dibiarkan untuk diisi manual.
- Pegawai yang sudah ada di daftar ditandai "Sudah ditambahkan" dan tidak bisa dipilih dua kali.
- NIP anggota disimpan bersama draf (opsional) dan tersedia pada template lewat penanda `<<nip anggota>>` (baris tabel anggota).
  Anggota tanpa NIP mencetak sel kosong.
## Template Word

Template **diunggah superadmin** (Pengaturan SAPA → Template dokumen) dan disimpan di database dengan versi; versi terbaru
langsung dipakai. Dua template bawaan (T01 ND Satker, T02 ND UE1) ada di `backend/sapa/templates/` dan dipakai bila belum ada
unggahan. **SK Tim dan Berita Acara belum punya template bawaan**: sebelum diunggah superadmin, tahap itu hanya bisa menyimpan draf
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

Kode UE1 (5 digit) → nama dibaca dari referensi UE1 aplikasi (lihat [referensi-ue1-dan-satker.md](referensi-ue1-dan-satker.md)); **sebutan Sekretaris** disimpan di
`sapa_ref_ue1` dan dipakai mengisi **tujuan Nota Dinas**. Diisi superadmin di Pengaturan SAPA → Referensi UE1. Bila belum diisi, pengguna mengetik tujuan manual.

## Jenis BMN dan satuan jumlahnya

Pada Nota Dinas usulan Satker, **jenis BMN dipilih dari daftar** (satu jenis per usulan) dan **satuan jumlah BMN** dipilih dari
satuan yang diizinkan untuk jenis itu. Daftar dan pemetaannya diatur superadmin di Pengaturan SAPA → **Jenis & satuan BMN**.

- Aturan kewajaran ada di pemetaan: tiap jenis hanya boleh memakai satuan yang dicentang padanya, sehingga Peralatan dan Mesin
  tidak mungkin ber-satuan "meter" dan Tanah tidak mungkin ber-satuan "unit". Memilih jenis lain mengganti satuan ke satuan
  bawaan jenis itu bila satuan saat ini tidak diizinkan.
- Aturan ditegakkan **di backend** (`sapa.ValidasiBMN`, dipanggil saat dokumen dibuat), bukan hanya di formulir, jadi API
  tidak bisa dipakai menembusnya. Nama jenis/satuan dikanonkan menurut daftar (huruf besar/kecil).
- Daftar awal (disemai migrasi 022 dan dijaga selaras dengan `sapa.DefaultRefBMN()` oleh tes): satuan bidang, unit, buah,
  set, paket, eksemplar; jenis Tanah (bidang), Gedung dan Bangunan (unit, buah), Tanah dan Bangunan (bidang, unit, paket),
  Peralatan dan Mesin (unit, buah, set, paket), Kendaraan Bermotor (unit), Jalan, Irigasi, dan Jaringan (unit, paket),
  Aset Tetap Lainnya (unit, buah, set, eksemplar). Superadmin bebas mengubahnya.
- Jenis/satuan yang **dinonaktifkan** tidak muncul di formulir; jenis tanpa satuan aktif juga disembunyikan. Satuan yang
  menjadi satu-satunya satuan sebuah jenis tidak bisa dihapus (dijawab 409 dengan nama jenisnya).
- Draf lama yang berisi teks bebas (sebelum fitur ini) tetap terbaca: formulir menandai jenis/satuan yang tidak sesuai daftar
  dan pengguna memilih ulang sebelum membuat dokumen.
- Asumsi: **satu jenis BMN per Nota Dinas** (satu nilai pada `<<jenis bmn>>` dan satu satuan pada terbilang jumlah).
  Usulan yang memuat beberapa jenis dibuat per jenis.

## Pejabat penandatangan dari HRIS2

Penandatangan ND Satker (nama, NIP, jabatan) dan ND UE1 (nama, jabatan) dipilih lewat kotak **Cari pejabat di HRIS2**, dengan
mekanisme yang sama dengan anggota tim (sesi SSO pengguna, hanya NIP/nama/jabatan yang diteruskan). Nama terisi dengan gelar,
NIP dan jabatan otomatis; semua bidang tetap bisa diubah atau diketik manual. NIP tidak disimpan pada ND UE1 (template-nya
tidak memakainya).

## Template Excel daftar barang

Daftar barang bisa diisi lewat Excel, selain mengetik satu per satu atau menempel:

- **Unduh template Excel** (`GET /sapa/barang/template`): berkas `.xlsx` dengan sheet *Daftar Barang* (judul kolom, kolom
  Kode/NUP berformat teks agar nol di depan tidak hilang, daftar pilihan Kondisi, validasi Tahun dan nilai uang, baris judul
  dibekukan) dan sheet *Petunjuk*.
- **Unggah Excel** (`POST /sapa/barang/impor`, multipart `berkas`): kolom dikenali dari **judulnya** (urutan bebas, kolom tambahan
  diabaikan; wajib ada Nama Barang, Nilai Perolehan, Nilai Limit), sheet *Daftar Barang* dipakai bila ada, selain itu sheet
  pertama. Maksimal 2 MB dan 500 barang; baris kosong dilewati. Angka dari Excel dirapikan (notasi ilmiah, sisa pembulatan,
  "2001.0"). Baris yang bermasalah **tetap dimuat** dan dilaporkan per baris (nomor baris = baris di Excel) supaya diperbaiki di
  formulir. Bila daftar saat ini sudah berisi, pengguna memilih *Ganti* atau *Tambahkan*.
- **Unduh daftar ini** (`POST /sapa/barang/ekspor`): daftar yang sedang dikerjakan dalam format template yang sama, untuk
  diedit di Excel lalu diunggah kembali. Semua teks ditulis sebagai teks (bukan rumus) sehingga isi yang diawali `=`, `+`, `-`,
  atau `@` tidak dieksekusi Excel.
- Berkas dibaca dengan batas bongkar zip dan penangkap *panic* agar berkas rusak/zip bomb hanya menghasilkan galat 400.

## Alamat usulan memakai UUID

Halaman dan API usulan dialamatkan dengan **UUID acak**, mis. `/dashboard/sapa/penjualan/6f9619ff-8b86-d011-b42d-00c04fc964ff`, bukan nomor
berurutan, supaya alamat usulan lain tidak bisa ditebak dengan menambah/mengurangi angka.

- Migrasi `023_sapa_penjualan_uuid.sql` menambah kolom `uuid` (UNIQUEIDENTIFIER, indeks unik) pada `sapa_penjualan`; usulan yang sudah ada
  otomatis mendapat UUID. `id` BIGINT tetap menjadi kunci internal (relasi tahap dan dokumen) dan **tidak pernah dikirim ke klien**:
  JSON usulan memuat `id` = UUID, dan dokumen tidak lagi memuat `penjualan_id`.
- Semua rute `/sapa/penjualan/:id/...` menerima UUID bentuk baku (8-4-4-4-12 heksadesimal; huruf besar/kecil sama). Nomor lama (`/penjualan/1`),
  UUID tanpa tanda hubung, atau yang berkurung dijawab **404**. UUID yang tidak ada dan usulan milik satker lain dijawab sama persis (404).
- UUID **bukan** pengganti hak akses: setiap aksi tetap memeriksa peran dan cakupan pengguna. Noreg (`PJ-<tahun>-<5 digit>`) tetap berurutan
  karena hanya nomor tampilan pada surat, bukan alamat.
- Dokumen hasil diunduh lewat `/sapa/dokumen/:id/unduh` dengan nomor dokumen berurutan; tidak terlihat di bilah alamat dan dijaga hak akses yang sama.

## Yang dihitung otomatis

- **Jumlah BMN, total nilai perolehan, total nilai limit** dihitung dari daftar barang (tidak diinput), beserta terbilangnya
  ("Tiga Miliar Lima Ratus … Rupiah Lima Puluh Sen"), supaya angka di surat selalu sama dengan tabel lampiran.
- Hari, tanggal, bulan, dan tahun pada Berita Acara dibentuk dari tanggal penelitian.
- Nilai uang boleh diketik `1500000`, `1500000.50`, atau `1.500.000,50` (paling banyak dua desimal). Tabel daftar barang bisa
  **ditempel dari Excel** (kolom: Nama, Kode, NUP, Lokasi/Merk/Tipe, Kondisi, Tahun, Nilai Perolehan, Nilai Limit, Keterangan)
  atau diunggah dari template `.xlsx` (lihat bagian di atas).
- Saran isian: tujuan surat dari referensi UE1; kota dari data satker (awalan "KOTA ADM."/"KAB." dibuang); nomor dan tanggal
  ND UE1 dari catatan Nadine tahap 4; data penandatangan tidak ditebak (diketik pengguna).

## Tabel database (migrasi `021_create_sapa.sql`)

| Tabel | Isi |
|---|---|
| `sapa_peran` | peran SAPA per pengguna (+ kode satker / kode UE1); **tidak dipakai lagi** sejak migrasi 054 (peran kini dari `user_roles`), dibiarkan sebagai cadangan |
| `sapa_ref_ue1` | referensi UE1 |
| `sapa_template` | template Word berversi (`VARBINARY`), satu versi aktif per jenis |
| `sapa_urutan` | pencacah Noreg per tahun |
| `sapa_penjualan` | usulan penjualan |
| `sapa_penjualan_tahap` | status, isian (JSON), nomor/tanggal/catatan tiap tahap |
| `sapa_dokumen` | dokumen Word hasil (riwayat semua versi) |

Migrasi `022_create_sapa_bmn.sql` menambah `sapa_satuan` (satuan jumlah), `sapa_jenis_bmn` (jenis + satuan bawaan), dan
`sapa_jenis_bmn_satuan` (pemetaan jenis → satuan yang diizinkan, `ON DELETE CASCADE`), lengkap dengan daftar awalnya.

Nama satker dan UE1 pada formulir usulan baru dicari di `DIGITALISASI_SATKER` (data Digitalisasi Aset); bila belum disinkronkan,
pembuat usulan mengetik nama satker manual.

## Struktur kode

| Lokasi | Isi |
|---|---|
| `backend/sapa/docx` | mesin dokumen Word (penggantian penanda lintas run, operasi tabel), tanpa dependensi aplikasi |
| `backend/sapa` | alur tahap dan hak akses (`tahap.go`), data + validasi (`data.go`), pengisi dokumen (`isi.go`), aturan bisnis (`layanan.go`), SQL (`store.go`), versi memori untuk tes (`memrepo.go`) |
| `backend/handlers/sapa_handler.go`, `backend/routes/sapa_routes.go` | HTTP `/api/v1/sapa/...` |
| `frontend/lib/sapa.ts`, `frontend/components/sapa/` | tipe, API, dan halaman |

Rute (`:id` = UUID usulan): `GET /sapa/saya` (peran, kode satker 6/18 digit, kode Kanwil/UE1, `satker_pilihan`), `GET /sapa/referensi/satker`, `GET|POST /sapa/penjualan`, `GET /sapa/penjualan/:id`,
`PUT /sapa/penjualan/:id/tahap/:kunci` (draf), `POST .../dokumen`, `.../selesai`, `.../lewati`, `.../buka-ulang` (superadmin),
`GET /sapa/dokumen/:id/unduh`, `GET /sapa/referensi/bmn`, `GET /sapa/barang/template`, `POST /sapa/barang/impor`,
`POST /sapa/barang/ekspor`; superadmin: `/sapa/template`, `/sapa/ref-ue1`, `GET /sapa/bmn`,
`PUT|DELETE /sapa/bmn/satuan`, `PUT|DELETE /sapa/bmn/jenis` (nama lewat badan JSON atau `?nama=`).

## Pengujian

```bash
cd backend && go test ./sapa/... ./routes/ ./handlers/
cd frontend && node --test components/sapa/sapa.test.mjs
```

- `backend/sapa`: alur tahap, hak akses, validasi, pengisian template asli, aturan bisnis (penyimpanan memori), dan SQL `Store`
  (driver palsu: urutan transaksi, jumlah parameter, escape `LIKE`).
- `backend/routes/sapa_test.go`: HTTP dengan tabel rute asli (`RegisterSapa`): hak akses per peran, 404 vs 403, alur lengkap
  sampai unduhan, unggah template (multipart), batas ukuran, dan galat internal yang tidak bocor.
- `backend/sapa/bmn_test.go`: kewajaran jenis-satuan, pengaturan admin (konflik hapus), dan kesamaan seed migrasi 022 dengan
  `DefaultRefBMN()`; `backend/sapa/barang_xlsx_test.go`: template, ekspor→impor, anti-rumus, urutan kolom bebas, angka ala Excel,
  galat per baris, berkas rusak, dan batas baris.
- `frontend/components/sapa/sapa.test.mjs`: fungsi murni (nilai uang yang sama dengan backend, normalisasi isian, tempel dari Excel,
  pemilihan jenis/satuan BMN, penggabungan hasil impor).

## Menjalankan pertama kali

1. `./deploy.sh` (migrasi 021, 022, 023, dan 054 berjalan otomatis; 022 menyemai daftar jenis BMN dan satuan awal, 023 memberi UUID pada usulan, 054 menyalin peran SAPA lama ke peran aplikasi).
2. Superadmin membuka **Pengaturan SAPA**: isi **Referensi UE1**, periksa **Jenis & satuan BMN** (sesuaikan dengan kebutuhan),
   unggah template SK Tim dan Berita Acara (bila ingin dibuat di aplikasi). Peran pengguna **tidak** diatur di SAPA: beri peran
   data (Satker, Kanwil, UE1, Pengguna Barang) di **Manajemen Pengguna**.
3. Pengguna Satker membuka SAPA → Penjualan → *Buat usulan penjualan*.

## Yang belum diverifikasi / batasan

- Seluruh SQL diuji dengan database **palsu** dan penyimpanan memori; belum dijalankan ke SQL Server asli (sintaks `MERGE`,
  `UPDATE ... WITH (UPDLOCK, SERIALIZABLE)`, indeks unik terfilter, `OFFSET/FETCH` perlu diperiksa saat migrasi pertama).
- Dokumen hasil diperiksa lewat isi XML-nya (penanda terganti, tabel terisi) dan dibuka ulang oleh pustaka .docx; **belum dibuka
  di aplikasi Microsoft Word** (tidak ada di lingkungan pengembangan). Buka satu hasil di Word untuk memastikan tampilannya.
- `<<jumlah bmn>>` pada template mencetak angka saja (mis. "3"); satuan muncul lewat `<<terbilang jumlah bmn>>`. Satuan kini
  wajib dan dibatasi oleh daftar jenis BMN.
- Impor Excel diuji dengan berkas buatan `excelize` sendiri (ekspor → impor, variasi urutan kolom, angka ala Excel); **belum diuji
  dengan berkas yang disimpan oleh aplikasi Microsoft Excel/LibreOffice sungguhan**. Buka template di Excel sekali untuk memastikan
  daftar pilihan Kondisi dan validasi isian tampil.
- Template T02 (ND UE1) memuat salah ketik "Pengadaab" pada tembusan; template tidak diubah aplikasi, perbaiki di berkas Word lalu unggah ulang.
- Peran Kanwil dibatasi 9 karakter pertama kode satker; ketepatannya bergantung pada kode Kanwil yang diberikan superadmin atau Pengguna Barang (atau saran dari kode satker SSO pegawai) sama dengan awalan kode satker pada data aset.
- Tidak ada pengiriman email: dokumen diunduh dari aplikasi.
- Hanya modul Penjualan; Sewa dan lainnya belum ada.
