# API Akses Data PASTI (pengelolaan dan keamanan)

Aplikasi lain yang ingin memakai data PASTI **didaftarkan oleh superadmin** (tidak ada pendaftaran mandiri atau formulir publik), lalu menarik data lewat **kunci API** yang diterbitkan PASTI.
Semua pemakaian dibatasi (scope, batas laju, kuota) dan dipantau. Panduan untuk pihak luar ada di [api-akses-data-pemakaian.md](api-akses-data-pemakaian.md); dokumen ini untuk superadmin dan pengembang.

Kode: `backend/apiakses/` (logika), `backend/handlers/api_akses_*.go` (HTTP), `backend/migrations/060_create_api_akses.sql` dan `061_api_akses_tutup_pendaftaran_publik.sql`, frontend `components/apiakses/`,
`lib/apiAkses*.ts`, halaman `/dashboard/api-akses` dan `/dashboard/api-akses/monitoring` (hanya superadmin; **tidak ada halaman atau rute publik**).

## Alur

1. **Pendaftaran oleh superadmin** (`Administrasi > API Akses Data > Daftarkan aplikasi`, `POST /api/v1/api-akses/klien`). Pihak luar menghubungi administrator PASTI; superadmin mengisi data dari
   permintaan mereka: nama aplikasi, instansi, unit, PIC (nama, jabatan, email, telepon), tujuan penggunaan, **dataset yang diizinkan**, dan opsi allowlist IP, pembatasan UE1, serta batas khusus.
   Klien **langsung berstatus disetujui** (tanpa tahap menunggu) dan **kunci pertama diterbitkan** sekaligus, ditampilkan sekali. Validasi masukan ketat (semua kolom yang salah dilaporkan sekaligus,
   tidak ada yang tersimpan bila ada yang salah). Tercatat di Log Audit (`apiakses.klien.buat`, tanpa email dan isi tujuan).
2. **Keputusan superadmin selanjutnya** (rincian klien): *ubah scope dan batas*, *terbitkan/rotasi/cabut kunci*, *cabut akses*, *aktifkan kembali*.

   | Dari | Ke | Catatan |
   |---|---|---|
   | *(dibuat superadmin)* | disetujui | langsung disetujui dengan scope dan batas yang dipilih; kunci pertama diterbitkan dan ditampilkan sekali |
   | disetujui | dicabut | **semua kunci ikut dihentikan**; alasan wajib |
   | dicabut | disetujui | kunci lama **tidak** hidup lagi; kunci baru diterbitkan dan ditampilkan sekali |
   | menunggu | disetujui / ditolak | hanya untuk **sisa pendaftaran dari formulir publik lama** (bila ada di database): superadmin dapat menyetujui (dengan scope pilihannya) atau menolak (final). Tidak ada jalur kode yang membuat status ini lagi. |

3. **Penarikan data** lewat `https://<domain>/api/data/v1/...` dengan `Authorization: Bearer <kunci>`.
4. **Pemantauan** di `Administrasi > API Akses Data > Pemantauan penarikan data`.

## Keputusan rancangan dan alasannya

| Keputusan | Alasan |
|---|---|
| **Kunci API** (bearer opak), bukan OAuth *client credentials* | Penarikan data adalah server-ke-server dengan satu jenis hak. Kunci API tidak butuh titik akhir token, siklus segar-ulang token, atau penyimpanan sesi; cukup satu pencarian per permintaan (di-cache 10 detik). Pencabutan berlaku seketika. OAuth baru bermanfaat bila ada delegasi hak pengguna, yang tidak ada di sini. |
| Kunci acak 256 bit, **hanya hash HMAC-SHA256** yang disimpan, dibandingkan dengan waktu konstan | Rahasia sudah tak dapat ditebak sehingga hash cepat cukup (bcrypt akan membebani setiap permintaan). HMAC dengan rahasia server membuat hash yang bocor dari database tidak dapat dipakai menguji tebakan. Kunci tidak dapat dilihat lagi setelah penerbitan. |
| Rute klien di **`/api/data/v1`**, terpisah dari `/api/v1` | Tidak berbagi middleware dengan JWT/SSO pengguna: token pengguna tidak membuka rute klien, kunci klien tidak membuka rute pengguna (diuji). Hanya `GET`. |
| Penghitung batas laju dan kuota **di memori satu proses**; kuota disalin ke database tiap 5 detik dan dipulihkan saat mulai | Akurat dan murah (tanpa tulisan database per permintaan, tahan banyak klien serentak); restart paling banyak kehilangan 5 detik hitungan. Sama dengan pembatas laju lain di aplikasi ini (satu instance). Redis hanya dibutuhkan bila backend dijalankan lebih dari satu instance. |
| Batas laju = **jendela geser** (stempel per permintaan), kuota = **jendela tetap** kelipatan periode | Jendela geser tidak mengizinkan lonjakan dua kali lipat di batas jendela. Kuota "tiap jam" pulih tepat pada jam UTC (WIB UTC+7: juga tiap jam penuh). |
| Limit dan kuota **di database**, bukan konstanta | Superadmin mengubahnya tanpa deploy ulang (bawaan global dan per klien); berlaku dalam ≤ 5 detik. |
| Hanya dataset **Digitalisasi Aset** (`aset.*`) | Struktur dan kolom pribadinya sudah dinilai. Dataset lain ditambah dengan menambah entri di `apiakses/katalog.go`, bukan dengan pembaca generik. |

## Keamanan

- **Autentikasi**: header `Authorization: Bearer` atau `X-API-Key`; **tidak** dari alamat (URL). Bentuk kunci diperiksa sebelum menyentuh database. Semua kegagalan (tanpa kunci, salah, kedaluwarsa, dicabut,
  klien tidak aktif) dijawab **sama** (401, badan identik); alasan sebenarnya hanya masuk riwayat dan Log Audit. Pekerjaan per permintaan sama untuk kunci yang tidak dikenal (hash tetap dihitung).
- **Penjaga percobaan gagal**: 30 kegagalan per IP dalam 10 menit mengunci IP itu (429 + `Retry-After`) untuk kunci yang tidak valid; kunci sah yang sudah di cache tetap dilayani.
- **Least privilege**: dataset per klien (scope), **allowlist IP/CIDR** opsional, **pembatasan UE1** opsional (hanya baris satker UE1 itu). Dataset di luar scope: 403, tercatat.
- **Kolom yang tidak pernah keluar**: kolom bertanda `Sensitive` pada definisi dataset (mis. nama penghuni rumah negara), tautan foto (`Foto*`), dan `Nama_Pengguna` bangunan. Pencarian (`q`) hanya memakai kolom yang
  keluar. Diuji (`TestKolomPribadiTidakPernahKeluar`), termasuk terhadap SQL Server. Menambah kolom sensitif pada dataset otomatis tertutup.
- **Masukan**: semua nilai penyaring berupa parameter SQL; nama tabel/kolom hanya dari kode; `q` ≥ 3 karakter; `LIKE` di-escape; kursor dan halaman divalidasi; pesan galat tidak memuat isian pemanggil.
- **Kunci**: kedaluwarsa opsional (pengaturan *masa berlaku kunci*), **rotasi** dengan masa tenggang (kunci lama tetap berlaku N jam, bawaan 24), pencabutan per kunci, maksimal 5 kunci aktif per klien.
- **Tidak ada permukaan publik**: pendaftaran klien hanya lewat `POST /api-akses/klien` di balik `AuthRequired` + `RequireSuperadmin`. Formulir dan rute pendaftaran mandiri (beserta captcha, batas per IP,
  dan pengaturan "pendaftaran dibuka") **dihapus**, bukan sekadar disembunyikan: `/pendaftaran-api`, `GET /api-akses/info`, dan `POST /api-akses/pendaftaran` menjawab 404 (diuji, `TestRutePendaftaranPublikTidakAda`).
  Satu-satunya titik akhir yang dapat dijangkau tanpa login hanyalah `/api/data/v1/*` (kunci API) dan rute login yang sudah ada.
- **Log Audit** (kategori *API Akses Data*): pendaftaran klien oleh superadmin, setiap keputusan, perubahan scope/batas, penerbitan/rotasi/pencabutan kunci (hanya awalan kunci), perubahan pengaturan (sebelum dan sesudah), serta kejadian
  keamanan (kunci tidak sah, IP di luar allowlist, dataset di luar scope; tercatat sebagai `akses.ditolak`/`apiakses.akses.tidak_sah`). **Kunci, hash, dan email PIC tidak pernah masuk log**
  (`TestAPIAksesAlurLengkapDenganSQLServer` memeriksanya). Pemakaian data biasa tidak masuk Log Audit tetapi ke riwayat permintaan khusus (di bawah).
- Respons berisi data: `Cache-Control: no-store`. Respons penerbitan kunci: `no-store` juga. Rute klien tidak diberi CORS untuk origin lain (server-ke-server).

## Batas laju dan kuota

Bawaan (tabel `api_akses_pengaturan`, diubah di tab *Batas laju dan kuota*): **1.000 permintaan per 60 detik** per klien dan **kuota 5.000 permintaan per periode 3.600 detik (tiap jam)**. Setiap klien dapat diberi nilai
khusus (kosong = ikut bawaan). Rentang yang diterima: 1–100.000 permintaan, jendela 1–3.600 detik, kuota 1–100 juta, periode 60 detik–31 hari.

- Permintaan yang ditolak (429) **tidak** memakai jatah. Kuota diperiksa lebih dulu dari batas laju (masa tunggunya lebih panjang).
- `GET /saya` dan `/saya/penggunaan` serta `/katalog` tidak memakai kuota tetapi tetap dikenai batas laju.
- 429: `Retry-After` (detik), `X-RateLimit-Limit/Remaining/Reset`, `X-Quota-Limit/Remaining/Reset` (Reset = detik Unix UTC). Header ini juga ada pada respons sukses. Nama header tidak peka huruf besar/kecil.
- Perubahan batas (bawaan atau per klien) berlaku pada permintaan berikutnya (cache pengaturan 5 detik); menurunkan batas di bawah pemakaian dalam jendela berarti 429 sampai cukup permintaan keluar jendela.
- Mengubah *periode kuota* di tengah periode memulai periode baru dengan hitungan nol.

## Pemantauan

- Riwayat per permintaan (`api_akses_log`): waktu, klien, awalan kunci, endpoint, dataset, status, hasil, lama proses, jumlah baris, IP, user agent. **Tanpa** isi respons dan tanpa kunci. Bawaan disimpan **14 hari**.
- Hitungan per jam (`api_akses_agregat`) per klien/endpoint/status/hasil dengan histogram latensi: dasar grafik dan ringkasan jangka panjang (p50/p95 diperkirakan dari histogram). Bawaan **400 hari**.
- Ditulis asinkron (antrean 8.192, ditulis berkelompok tiap ≤ 2 detik); antrean penuh menjatuhkan riwayat (dihitung dan ditampilkan sebagai peringatan) tetapi hitungan per jam tetap lengkap. Sisa antrean ditulis saat server dimatikan.
- Halaman: ringkasan (permintaan, berhasil, galat, ditolak, 429 laju dan kuota, 5xx, latensi), grafik per jam/hari, **pemakaian kuota saat ini** per klien, tabel per klien dan per endpoint, riwayat dengan filter
  (hasil, IP, pencarian) dan rentang (1 jam sampai 1 tahun atau tanggal sendiri). Pemilik klien melihat ringkasan miliknya sendiri lewat `GET /api/data/v1/saya/penggunaan` (dengan kuncinya).

## API pengelolaan (superadmin, `/api/v1/api-akses`)

| Rute | Fungsi |
|---|---|
| `GET /klien`, `GET /klien/:id` | daftar (status, `q`, halaman) dan rincian beserta kunci dan pemakaian |
| `POST /klien` | **daftarkan klien baru** (data pemohon, dataset, allowlist IP, UE1, batas khusus, `label_kunci`): langsung disetujui, kunci pertama pada respons (sekali) |
| `PUT /klien/:id` | ubah scope, allowlist IP, UE1, batas khusus, catatan |
| `POST /klien/:id/setujui` · `/tolak` | hanya untuk sisa pendaftaran lama berstatus menunggu |
| `POST /klien/:id/cabut` · `/aktifkan` | cabut akses (semua kunci dihentikan); aktifkan kembali (kunci baru) |
| `POST /klien/:id/kunci` | terbitkan kunci (`rotasi: true` = rotasi) |
| `DELETE /klien/:id/kunci/:kunciId` | cabut satu kunci |
| `GET`/`PUT /pengaturan` | bawaan global (batas laju, kuota, masa berlaku kunci, tenggang rotasi, halaman maksimal, retensi) |
| `GET /pantau/ringkasan`, `/pantau/riwayat`, `/pantau/opsi` | pemantauan |

**Semua** rute di grup ini memakai `AuthRequired` + `RequireSuperadmin` (superadmin yang bertindak sebagai peran data tidak punya hak ini); tidak ada pengecualian publik. Tes tabel rute
(`routes/api_akses_test.go`) memeriksa bahwa setiap rute di bawah `/api/v1/api-akses/` menolak permintaan tanpa login (401).

## Pengaturan dan penerapan

- Migrasi `060_create_api_akses.sql` (tabel `api_akses_*`) dan `061_api_akses_tutup_pendaftaran_publik.sql` (menghapus baris pengaturan `pendaftaran_dibuka` yang tidak dipakai lagi) dijalankan otomatis oleh
  `deploy.sh`; keduanya idempotent. Migrasi 060 tidak diubah (sudah diterapkan di server); kolom dan status lama (menunggu, ditolak) dibiarkan agar data lama tidak hilang.
- `.env` (opsional): `API_AKSES_PEPPER` = rahasia untuk hash kunci; kosong = diturunkan dari `PASSWORD_PEPPER`. **Mengganti salah satunya membuat semua kunci API yang sudah terbit tidak berlaku.**
- Nginx: aturan `location ~ ^/(api/|...)` yang ada sudah meneruskan `/api/data/` dan `/api/v1/api-akses/`; header `X-Real-IP` yang dipasang Nginx dipakai untuk IP klien (allowlist, batas, Log Audit),
  jadi `TRUSTED_PROXIES` harus benar dan port backend tidak boleh terbuka langsung. Tidak ada rute publik baru yang perlu dibuka atau ditutup di Nginx. Bila perlu lapisan tambahan, `limit_req` Nginx pada `/api/data/` dapat dipasang tanpa mengubah aplikasi.
- Menjalankan lebih dari satu instance backend: batas laju dan kuota menjadi per instance. Pindahkan penghitung ke penyimpanan bersama (mis. Redis) sebelum itu.

## Tes

```bash
cd backend && go test ./apiakses/ ./routes/ ./audit/ ./middleware/        # unit dan rute (tanpa database)
cd frontend && node --test lib/apiAkses.test.mjs
```

Tes integrasi SQL Server (dilewati bila `PASTI_UJI_MSSQL_DSN` kosong; gunakan database KHUSUS TES yang sudah dimigrasi sampai 061, lihat contoh DSN di `apiakses/store_integrasi_test.go`):
`store_integrasi_test.go` (siklus klien dan kunci, pengaturan, kuota, riwayat, agregat, pembacaan data, injeksi SQL, kolom pribadi) dan `routes/api_akses_integrasi_test.go` (alur lengkap lewat router asli: rute publik lama 404, klien hanya dibuat superadmin dan langsung disetujui, kunci, tarik data, scope, batas, rotasi, pencabutan, pemantauan, Log Audit).
Validasi pendaftaran oleh superadmin juga diuji tanpa database (`handlers/api_akses_admin_test.go`). Tes unit mencakup kunci (bentuk, hash, waktu konstan), batas laju (jendela geser tepat, kuota, serentak 200 goroutine, perubahan batas, pemulihan), gerbang (semua kegagalan tampak
sama, allowlist, scope, cache dan pencabutan, kuota, Log Audit tanpa kunci), validasi masukan, dan tabel rute (setiap rute klien menolak tanpa kunci dan menolak JWT).

## Batasan yang perlu diketahui

- Penghitung di memori satu proses (lihat di atas). Setelah restart, batas laju (jendela 60 detik) mulai kosong; kuota dipulihkan dari salinan terakhir (paling banyak 5 detik hitungan hilang).
- Dataset: hanya Digitalisasi Aset. Data pengadaan (Inaproc) belum dibuka karena kolomnya belum dinilai satu per satu.
- Klien tidak pernah dihapus (jejak audit); yang tidak dipakai lagi berstatus *dicabut*.
- Penyerahan kunci ke PIC dan kabar ke pemohon dilakukan superadmin di luar aplikasi (PASTI tidak mengirim email). Pemohon tidak punya halaman atau akun di PASTI untuk fitur ini.
- Sisa pendaftaran berstatus *menunggu* dari formulir publik yang lama (bila ada di database) tidak dihapus otomatis; superadmin dapat menyetujui atau menolaknya dari daftar klien, atau menolak semuanya.
- Koordinat aset (lintang/bujur) ikut dalam dataset yang punya koordinat. Bila ada dataset yang koordinatnya tidak boleh keluar, tambahkan ke `kolomTertutup`.
