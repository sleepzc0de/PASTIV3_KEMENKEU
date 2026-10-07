# Log audit dan monitor resource (khusus superadmin)

Dua halaman di menu **Administrasi** yang hanya dapat dibuka **superadmin**: **Log Audit** (`/dashboard/audit`) untuk mengecek aktivitas pengguna, dan **Monitor Resource** (`/dashboard/monitor`) untuk
melihat apakah CPU, memori, disk, dan database masih cukup serta resource mana yang perlu ditambah. Keduanya hanya membaca. Kode: `backend/audit/`, `backend/monitor/`, `backend/handlers/audit_handler.go`,
`backend/handlers/monitor_handler.go`, `backend/migrations/058_create_audit_dan_monitor.sql`, frontend `components/audit/`, `components/monitor/`, `lib/audit*.ts`, `lib/monitor*.ts`.

## Akses

- Rute `/api/v1/audit/*` dan `/api/v1/monitor/*` memakai `AuthRequired` lalu `RequireSuperadmin`. Peran lain menerima **403**, belum login **401**. Superadmin yang **sedang bertindak sebagai peran data**
  (Satker, Kanwil, UE1, Pengguna Barang) kehilangan hak ini selama peran itu aktif, sama seperti fitur khusus superadmin lainnya.
- Menu muncul hanya bagi superadmin, dan halamannya sendiri menolak menampilkan isi bila yang membuka bukan superadmin.
- **Tidak ada rute untuk mengubah atau menghapus** entri log maupun riwayat (tes memeriksanya). Entri hanya dihapus oleh pembersih retensi.

## Log audit

### Yang dicatat

Satu baris `audit_log` per aktivitas, ditulis oleh middleware `audit.Middleware()` setelah permintaan selesai:

| Dicatat | Contoh aksi |
|---|---|
| Semua **perubahan data** (POST, PUT, PATCH, DELETE) oleh pengguna yang login | `pengguna.ubah`, `pengguna.hapus`, `peran.beri`, `sapa.tahap.selesai`, `referensi.kanwil.simpan`, `pengadaan.tarik.mulai` |
| **Ekspor dan unduhan**, serta pencarian data pegawai (GET yang sengaja dicatat) | `ekspor.digitalisasi`, `ekspor.pengadaan`, `ekspor.sapa.dokumen`, `hris2.cari`, `hris2.lihat` |
| **Login** berhasil dan gagal (kata sandi dan SSO) beserta alasannya | `auth.login.berhasil`, `auth.login.gagal` (kata sandi salah, captcha salah, akun tidak aktif, akun dikunci, SSO gagal, ...) |
| Setiap **akses ditolak** (403) bagi pengguna yang login | `akses.ditolak` (aksi aslinya ada di rincian `aksi_asal`) |
| Mengekspor log audit itu sendiri | `audit.ekspor` |

GET biasa (melihat daftar, dasbor) **tidak** dicatat. Rute pengubah data baru otomatis tercatat dengan uraian umum; tes `TestKatalogAuditSesuaiTabelRute` memaksa pengembang menambahkan aturan berlabel di
`audit/katalog.go` untuk rute baru, dan `TestGETYangDicatatAdalahBacaanSensitif` menjaga daftar GET yang dicatat tetap disengaja.

Setiap entri memuat: waktu (UTC, ditampilkan WIB), pengguna (id, username), peran dan kode peran yang berlaku saat itu, kategori, kode aksi, uraian, metode dan **pola rute** (mis. `/api/v1/users/:id`),
objek (tipe dan id), status HTTP, hasil, lama proses, alamat IP, user agent, ID permintaan (juga dikirim sebagai header `X-Request-ID` untuk mencocokkan dengan log server), dan rincian kecil (JSON).

### Yang TIDAK pernah dicatat

- **Isi permintaan** (badan), **kata sandi**, **token**, dan kode OAuth. Rincian hanya memuat parameter rute, alasan gagal, dan kata kunci pencarian.
- Kueri pada ekspor/pencarian dibersihkan dari parameter bernama `token`, `code`, `state`, `password`, `secret`, `key`, dan sejenisnya, lalu dipotong 300 karakter.
- **Nama yang diketik pada login gagal untuk akun yang tidak ada**: orang kerap salah mengetik kata sandi di kolom nama pengguna. Entrinya tetap tercatat (waktu, IP, alasan), tanpa nama itu.
- Permintaan tanpa login ke rute selain login/daftar (mis. pemindaian bot): tidak dicatat, supaya log tidak dibanjiri.

### Penulisan, retensi, dan keandalan

- Penulisan **asinkron**: entri masuk antrean (4.096) dan ditulis berkelompok tiap ≤1 detik, sehingga tidak memperlambat permintaan. Bila antrean penuh (database macet), entri dijatuhkan dan
  dihitung; hitungannya tampil di halaman (peringatan) dan di Monitor Resource (kartu "Pencatatan log audit"). Penulisan yang gagal dicoba sekali lagi.
- Entri tanpa pengguna (login gagal, dll.) dibatasi 30 per detik agar penyerang tidak bisa memenuhi tabel.
- Saat server dimatikan (`docker stop` mengirim SIGTERM) permintaan yang berjalan diselesaikan lalu **sisa antrean ditulis** ke database (batas 15 detik).
- Retensi: entri lebih dari **`AUDIT_RETENSI_HARI`** hari (bawaan **365**; `0` = tidak pernah dihapus) dihapus otomatis sekali sehari, berkelompok 5.000 baris.
- Alamat IP memakai `c.ClientIP()`. Di balik Nginx, itu berasal dari header `X-Forwarded-For`/`X-Real-IP` yang dipasang Nginx; alamat yang tersambung langsung (proxy) disimpan juga di rincian
  (`remote_addr`) bila berbeda. Pastikan hanya Nginx yang dapat menjangkau port backend (8686) di produksi, supaya header itu tidak dapat dipalsukan klien.

### Halaman Log Audit

- **Ringkasan** rentang waktu: jumlah aktivitas (berhasil/gagal), pengguna aktif, login berhasil/gagal, akses ditolak, ekspor; grafik per jam atau hari; daftar menurut kategori, aksi teratas, dan
  **sumber login gagal terbanyak** (IP; 5 atau lebih ditandai merah). Daftar-daftar itu bisa diklik untuk menyaring log.
- **Filter**: rentang (1 jam sampai 90 hari, semua waktu, atau tanggal sendiri dalam WIB), kategori, aksi, hasil, username, IP, dan pencarian bebas (uraian, rute, objek, rincian).
- **Tabel** per halaman 50 entri; klik baris untuk **rincian lengkap** (termasuk ID permintaan dan rincian tambahan) dan tautan "Lihat semua aktivitas pengguna ini".
- **Per pengguna**: siapa saja yang aktif, jumlah aktivitas dan kegagalan, terakhir aktif, login terakhir, IP terakhir, status akun.
- **Ekspor CSV** menurut filter (maksimal 50.000 baris terbaru; UTF-8 dengan BOM agar terbaca benar di Excel; sel berawalan `=`, `+`, `-`, `@` diamankan dari injeksi rumus).

API (semuanya GET): `/audit` (daftar), `/audit/ringkasan`, `/audit/pengguna`, `/audit/opsi` (kategori dan aksi untuk filter), `/audit/ekspor` (CSV). Parameter: `dari`, `sampai` (RFC3339, atau tanggal `YYYY-MM-DD`
yang dibaca sebagai tanggal WIB), `user_id`, `username`, `kategori`, `aksi`, `hasil` (`semua|berhasil|gagal`), `ip`, `q`, `halaman`, `per_halaman` (maks 200).

## Monitor resource

### Yang diukur

| Bagian | Isi | Sumber |
|---|---|---|
| **Server** | CPU (%), memori (terpakai = total − tersedia, cache tidak dihitung), swap, disk, beban rata-rata, uptime, batas container (cgroup) | Linux: `/proc`, `statfs`, cgroup. Windows (pengembangan): API sistem; beban rata-rata dan swap tidak tersedia |
| **Aplikasi** | goroutine, heap, memori fisik proses (RSS), jeda GC, berkas terbuka, waktu hidup | runtime Go dan `/proc/self` |
| **HTTP** | jumlah, galat 4xx/5xx, rata-rata, p50/p95/p99 pada 5 menit, 1 jam, dan sejak mulai; rute paling lambat dan rute dengan galat | middleware `monitor.Middleware()` (histogram per pola rute; tanpa isi atau alamat lengkap) |
| **Database** | pool koneksi aplikasi dan SLDK (dipakai, menunggu, ping), versi/edisi SQL Server, ukuran file, ruang disk volume database, memori server database, page life expectancy, buffer cache hit ratio, tabel terbesar | `database/sql` dan DMV SQL Server |

Pengukur berkala berjalan tiap **30 detik** dan menyimpan satu **snapshot tiap 5 menit** (rata-rata dan puncak) ke `monitor_snapshot`; riwayat disimpan **`MONITOR_RETENSI_HARI`** hari (bawaan 30) lalu dihapus.
`MONITOR_AKTIF=false` mematikan pengukur berkala (halaman tetap menampilkan keadaan sesaat, tanpa riwayat). Waktu respons keseluruhan **tidak menghitung** rute yang memang berat (ekspor, impor, sinkronisasi,
penarikan, unduh dokumen), supaya p95 mencerminkan pengalaman di halaman biasa. Tugas berat yang sedang berjalan (sinkronisasi SLDK, penarikan Inaproc) ditampilkan di atas halaman karena menjelaskan lonjakan.

Bagian database yang butuh izin (`VIEW SERVER STATE`, `VIEW DATABASE STATE`) dilewati bila akun database aplikasi tidak punya izin itu, dan alasannya ditampilkan; yang lain tetap bekerja.

### Penilaian "mana yang perlu ditambah"

Setiap resource dinilai **cukup**, **perlu perhatian**, atau **perlu ditambah/diperiksa** dari nilai sekarang dan **puncak 24 jam dan 7 hari** (bukan hanya sesaat). Kartu menampilkan tindakan satu kalimat dan
"alasan dan angka" yang bisa dibuka. Ambang (konstanta di `monitor/rekomendasi.go`, diuji di `rekomendasi_test.go`):

| Resource | Perhatian | Perlu ditambah |
|---|---|---|
| CPU server | rata-rata 24 jam ≥ 60%, atau puncak ≥ 95% dengan rata-rata ≥ 30%, atau beban ≥ 1,5 per core | rata-rata 24 jam ≥ 80%, atau beban ≥ 3 per core |
| Memori (RAM) | puncak 24 jam ≥ 85% | ≥ 92%, atau swap aktif (≥ 25%) saat memori ≥ 80% |
| Disk server | terpakai ≥ 80%, atau diperkirakan penuh < 30 hari | ≥ 90%, atau penuh < 7 hari |
| Penyimpanan database | volume ≥ 80%, atau penuh < 30 hari | ≥ 90%, atau penuh < 7 hari |
| Koneksi database (pool) | puncak ≥ 70% dari batas, atau ada permintaan yang menunggu | puncak ≥ 90% dan ada yang menunggu |
| Memori SQL Server | page life expectancy < 300 dtk, atau hit ratio < 90% | PLE < 100 dtk |

Perkiraan "penuh dalam N hari" memakai laju pertumbuhan dari riwayat (butuh minimal 12 jam data). Penilaian **kinerja** (waktu respons p95 ≥ 1 dtk/3 dtk, galat 5xx ≥ 1%/5%, goroutine ≥ 10.000/50.000,
memori proses ≥ 75%/90% dari batas, ketersambungan database, dan pencatatan audit) berstatus "perlu diperiksa" bukan "perlu ditambah", karena menambah resource belum tentu menyelesaikannya.
Banner di atas halaman merangkum: resource server yang perlu ditambah, atau yang perlu diperhatikan.

API (semuanya GET): `/monitor/ringkasan` (keadaan + penilaian), `/monitor/riwayat?rentang=1j|24j|7h|30h` (grafik; 1 jam dari memori tiap 30 detik, selebihnya dari snapshot yang diturunkan resolusinya),
`/monitor/database` (tabel terbesar, disimpan sementara 10 menit).

## Pengaturan (`backend/.env`)

```
AUDIT_RETENSI_HARI=365     # 0 = log audit tidak pernah dihapus
MONITOR_AKTIF=true         # pengukur berkala dan penyimpanan riwayat
MONITOR_RETENSI_HARI=30
```

Migrasi `058_create_audit_dan_monitor.sql` (tabel `audit_log`, `monitor_snapshot`) dijalankan otomatis oleh `deploy.sh`.

## Batasan yang perlu diketahui

- Di Docker, `/proc` menunjukkan keadaan **server induk** (yang memang menentukan perlu tidaknya server ditambah); batas container hanya tampil bila dipasang. CPU dan memori **server database** tidak
  terlihat bila SQL Server berada di mesin lain dan akun database aplikasi tidak punya izin `VIEW SERVER STATE`; hanya ukuran file dan statistik yang bisa dibaca akun biasa yang tampil.
- Penilaian memakai ambang umum; sesuaikan konstanta di `monitor/rekomendasi.go` bila kebutuhan berbeda. Riwayat baru berguna setelah beberapa jam berjalan, dan perkiraan pertumbuhan disk setelah ±12 jam.
- Log audit membuktikan **apa yang dilakukan lewat aplikasi**; perubahan langsung pada database tidak tercatat. Untuk perlindungan tambahan, batasi akun database aplikasi agar tidak boleh `UPDATE`/`DELETE`
  pada `audit_log` selain oleh pembersih retensi (belum dikonfigurasi dari aplikasi).
- Dua tugas pembersihan (log audit dan riwayat resource) berjalan di proses aplikasi; bila aplikasi dijalankan lebih dari satu instance, keduanya akan berjalan di tiap instance (aman, hanya redundan), tetapi
  hitungan HTTP dan resource per instance terpisah.
- Belum diuji pada server Linux/Docker sesungguhnya: pembaca `/proc` dan cgroup diuji dengan contoh isi berkas, dan bangunannya diperiksa untuk Linux; pengukuran langsung hanya dicoba di Windows (pengembangan).
