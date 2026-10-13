# Log Server dan Ketahanan Backend

Dokumen ini menjelaskan dua hal yang dibuat bersamaan (migrasi 059):

1. **Log server yang ringan**: hanya galat (error) dan peringatan (warning) yang dicatat, kejadian yang berulang digabung, dan ada halaman **Log Server** khusus superadmin.
2. **Ketahanan backend** saat diakses banyak pengguna sekaligus: lebih cepat, tidak mudah jatuh, dan pemakaian memori/disk tetap berbatas.

> Soal "admin": aplikasi ini tidak lagi punya peran admin terpisah (role `admin` sudah ditiadakan; hak pengelola hanya ada pada **superadmin** dari `.env`). Halaman Log Server, Log Audit, dan Monitor Resource sama-sama **khusus superadmin**. Pengguna Barang dan peran data lain mendapat 403.

## 1. Log server

### Apa yang dicatat dan apa yang tidak

| Dicatat (disimpan di tabel `server_log`) | Tidak dicatat (tidak disimpan) |
|---|---|
| Baris log berlevel **ERROR** atau **FATAL** (mis. `[INAPROC SYNC ERROR]`) | Log akses per permintaan (`[GIN] 200 \| GET ...`), yang sebelumnya menulis satu baris untuk **setiap** permintaan |
| Baris log berlevel **WARN** | Baris info/debug/kemajuan (`[DIGITALISASI] ...`, `[INAPROC SYNC DEBUG] ...`) |
| Respons **5xx** (galat server) beserta rute, metode, durasi, ID permintaan | Permintaan sukses (2xx) dan galat klien (4xx) |
| **Panik** (dengan jejak tumpukan), termasuk panik di pekerjaan latar belakang | Galat `context canceled` (pengguna menutup tab/membatalkan permintaan); hanya dihitung |
| Permintaan **lambat** (> `LOG_PERMINTAAN_LAMBAT_MS`, rute berat dikecualikan) | Isi permintaan, kata sandi, token |
| Server **kewalahan** (permintaan ditolak 503 karena beban penuh) | |

Level dibaca dari tanda di awal pesan log yang sudah dipakai di seluruh kode: kata terakhir di dalam kurung siku menentukan levelnya (`[SSO ERROR]` → error, `[AUTH WARN]` → warning, `[X DEBUG]`/`[X INFO]`/tanpa kata level → info). Kode baru cukup mengikuti pola yang sama; tidak ada yang perlu diubah di pemanggilnya.

### Mengapa tidak memberatkan server

- **Tidak ada log akses.** `gin.Default()` diganti `gin.New()`; pada uji beban 68.794 permintaan, log lama menulis ±7 MB, log baru 508 byte (hanya baris menyalanya server).
- **Info/debug dibuang** di production (`LOG_LEVEL` bawaan `warn`); satu-satunya pengecualian adalah pesan siklus hidup `[INFO]` polos (server menyala/berhenti, tersambung ke database), supaya `docker logs` tetap bisa dibaca.
- **Kejadian yang sama digabung.** Sidik jari = sumber + rute + pesan yang angka/UUID-nya diseragamkan. Seribu "gagal membaca baris N" menjadi **satu baris berhitungan 1000**, bukan seribu baris. Dalam uji, 541 penolakan beban tersimpan sebagai satu baris `× 541`, dan 606 kejadian menjadi 40 baris. Kejadian yang sama dalam 1 jam digabung ke baris yang sama; lewat itu dibuat baris baru supaya tetap terlihat per jam.
- **Konsol ikut dihemat.** Kejadian yang sama hanya dicetak sekali per 10 detik; sisanya dihitung dan dilaporkan pada cetakan berikutnya ("N kejadian serupa sebelumnya tidak dicetak").
- **Tulis di latar belakang, berbatas.** Antrean 2.048 kejadian + penampung 1.000 jenis kejadian; tulis ke database tiap 5 detik secara berkelompok. Mencatat **tidak pernah menunggu** database. Bila database mati, kelompok ditahan di memori (berbatas) dan dicoba lagi dengan jeda yang makin panjang; bila melewati batas, kejadian dijatuhkan dan **dihitung** (terlihat di halaman Log Server).
- **Retensi otomatis.** Error disimpan `LOG_RETENSI_ERROR_HARI` (30), warning `LOG_RETENSI_WARN_HARI` (7), dan tabel dibatasi `LOG_MAKS_BARIS` (50.000, yang tertua dihapus lebih dulu). Pembersihan jalan tiap jam dalam kelompok kecil agar tabel tidak terkunci lama.
- **Rahasia disamarkan** sebelum dicetak atau disimpan (kata sandi, token, `Authorization: Bearer`, JWT, `client_secret`, `?code=`); pesan dipotong 2.000 karakter dan rincian 8.000.
- **Log container dibatasi** di `docker-compose.yml` (5 berkas × 10 MB), jadi disk tidak penuh oleh log Docker.

### Halaman Log Server (`/dashboard/log-server`)

Menu **Administrasi → Log Server** (hanya superadmin). Isinya: ikhtisar (error dan warning 24 jam/7 hari, jenis kejadian, kejadian terakhir, ukuran tabel, puncak permintaan serentak), grafik kejadian per jam, sumber teratas, kejadian paling sering berulang (biasanya akar masalah yang paling layak diperbaiki), tab **Semua / Error / Warning**, filter rentang waktu, sumber, dan pencarian (pesan, rute, ID permintaan), tabel dengan penanda `× N`, dan rincian (jejak tumpukan, ID permintaan yang sama dengan header `X-Request-ID`).

Halaman juga memperingatkan bila ada catatan yang tidak sempat disimpan atau penulisan ke database gagal, dan menampilkan **tekanan beban** (permintaan ditolak 429/503) beserta petunjuk env mana yang perlu disetel.

API (semuanya `AuthRequired` + `RequireSuperadmin`, hanya GET): `/api/v1/log-server`, `/ringkasan`, `/opsi`, `/:id`. Tidak ada rute untuk mengubah atau menghapus log.

### Pengaturan (`backend/.env`)

| Variabel | Bawaan | Arti |
|---|---|---|
| `LOG_LEVEL` | `warn` (production), `info` (lainnya) | Ambang cetak ke konsol (`error`, `warn`, `info`, `debug`). Tidak memengaruhi apa yang disimpan |
| `LOG_SIMPAN` | `warn` | Yang disimpan ke database: `warn` (error + warning), `error` (hanya error), `off` |
| `LOG_RETENSI_ERROR_HARI` / `LOG_RETENSI_WARN_HARI` | 30 / 7 | Umur baris sebelum dihapus |
| `LOG_MAKS_BARIS` | 50000 | Batas jumlah baris tabel |
| `LOG_PERMINTAAN_LAMBAT_MS` | 3000 | Permintaan lebih lama dari ini dicatat sebagai warning (0 = tidak dicatat) |

## 2. Ketahanan saat banyak pengguna bersamaan

| Langkah | Efeknya |
|---|---|
| **Batas waktu server** (`ReadHeaderTimeout` 10 dtk, `ReadTimeout` 2 mnt, `WriteTimeout` 15 mnt sejalan dengan Nginx, `IdleTimeout` 2 mnt, header maks 64 KB) | Sambungan yang sengaja dilambatkan (slowloris) atau tidak pernah ditutup tidak menumpuk tanpa batas |
| **Cache status akun dan peran** (`AUTH_CACHE_DETIK`, bawaan 10) | Setiap permintaan terautentikasi dulu membaca 2 query ke database; kini 25 permintaan pengguna yang sama = 1 pembacaan. Perubahan lewat aplikasi (kelola pengguna/peran, persetujuan, login SSO) **langsung membuang cache**, jadi penonaktifan akun tetap langsung berlaku. Hanya perubahan langsung di database yang tertunda sampai 10 detik |
| **Pembatas laju per IP** (token bucket; umum 200/dtk lonjakan 600, login 30/mnt, captcha 60/mnt) | Satu klien yang menggila (loop, skrip, tebak kata sandi) tidak bisa menghabiskan server. Bawaan umum sengaja longgar karena satu IP kantor bisa dipakai banyak pengguna. Memori pembatas berbatas (50.000 IP) sehingga tidak bisa dikuras dengan IP palsu. Penolakan: 429 + `Retry-After` |
| **IP klien yang tidak bisa dipalsukan** (`TRUSTED_PROXIES`) | Header `X-Real-IP` (ditimpa Nginx) hanya dipercaya bila sambungan datang dari proksi tepercaya; selain itu header diabaikan. Sebelumnya `X-Forwarded-For` dari klien bisa memalsukan IP di log audit |
| **Pelindung beban** (`MAKS_PERMINTAAN_SERENTAK`, bawaan 400, antre `ANTREAN_TUNGGU_DETIK` 3 dtk) | Saat beban melampaui kemampuan, kelebihan permintaan ditolak cepat (503 + `Retry-After`) alih-alih membuat semuanya melambat sampai server jatuh. `/health` tetap menjawab |
| **Batas rute berat** (`MAKS_BERAT_SERENTAK`, bawaan = jumlah inti CPU, min. 2) | Ekspor Excel/PDF/CSV, impor Excel, pembuatan dokumen Word, analitik, dan peta berbagi satu batas; yang berlebih antre lalu ditolak 503. Ekspor besar tidak lagi menghabiskan CPU dan koneksi database semua pengguna |
| **Antrean bcrypt** | Hitungan bcrypt dibatasi sebanyak inti CPU; badai login tidak menghabiskan semua CPU |
| **Kolam koneksi database** (`DB_MAKS_KONEKSI` 50, idle 25, umur 10 mnt, idle 3 mnt; batas waktu sambung 10/30 dtk; nama aplikasi `PASTI V3 Backend` terlihat di SQL Server) | Cukup untuk beban serentak, dan sambungan mati akibat firewall di antara dua server cepat diganti |
| **Query baca memakai konteks permintaan** | Pengguna yang menutup tab/berpindah halaman membatalkan query-nya, jadi koneksi cepat kembali ke kolam |
| **Percobaan ulang sambung database saat menyala** (12× tiap 5 dtk) | Server database yang belum siap saat container menyala tidak membuat backend jatuh-bangun (crash loop) |
| **Pemulih panik** + `serverlog.Aman` pada pekerjaan latar belakang | Panik pada satu permintaan menjadi 500 yang rapi dan tercatat lengkap; panik pada pengukur/pembersih tidak mematikan seluruh proses |
| **Batas ukuran badan** (`BATAS_BADAN_MB` 12) | Badan permintaan raksasa tidak menghabiskan memori |
| **Batas lunak memori Go** (`BATAS_MEMORI_MB`, atau otomatis 85% dari batas memori container, atau `GOMEMLIMIT`) | Pemulung memori bekerja lebih agresif sebelum proses menyentuh batas dan dimatikan OS |
| **`/health` dan `/health/ready`** | `/health` murah (proses hidup; dipakai `healthcheck` container); `/health/ready` memeriksa database (di-cache 3 dtk) |
| **Nginx**: gzip untuk JSON/CSV, `proxy_connect_timeout 5s` | Lalu lintas dan waktu tampil berkurang; backend yang mati cepat ketahuan |

### Hasil uji (mesin pengembangan, satu komputer menjalankan penguji dan server, database kosong)

Campuran 8 endpoint terautentikasi, pembatas laju umum dimatikan agar sebanding:

| Klien serentak | Sebelum: permintaan/dtk · p50 · p95 | Sesudah: permintaan/dtk · p50 · p95 |
|---|---|---|
| 20 | 2.760 · 5,0 ms · 14,0 ms | **3.813** · 2,8 ms · 13,0 ms |
| 100 | 3.054 · 25,7 ms · 80,7 ms | **4.268** · 15,0 ms · 73,2 ms |
| 300 | 2.858 · 81,3 ms · 269,6 ms | **4.144** · 42,5 ms · 240,9 ms |

Ringkasannya: throughput naik ±40%, median waktu respons turun hampir separuh. Pada 300 klien yang dimulai serentak ada sambungan yang ditolak OS ("actively refused") baik sebelum maupun sesudah; bila klien dinaikkan bertahap, galat itu hilang sepenuhnya (artefak lonjakan SYN pada loopback Windows, bukan perilaku aplikasi).

Uji ketahanan:

- **Sambungan database diputus paksa di tengah beban** (50 sesi di-`KILL` saat 50 klien aktif): dari 13.242 permintaan hanya **3** yang gagal (500), lalu pulih sendiri tanpa restart; galatnya tercatat di Log Server.
- **Pelindung beban**: dengan batas sengaja kecil, 541 permintaan ditolak cepat dengan 503 sementara sisanya dilayani; server pulih penuh setelah beban selesai; seluruh penolakan tersimpan sebagai **satu** baris berhitungan 541.
- **Pembatas laju**: captcha 5/menit per IP → yang ke-6 ditolak 429 + `Retry-After`; IP lain (lewat `X-Real-IP` di balik proksi tepercaya) punya ember sendiri; tanpa proksi tepercaya, `X-Real-IP`/`X-Forwarded-For` palsu **tidak** menghindari batas.
- **Database belum siap saat menyala**: backend mencoba lagi dan melaporkan percobaannya, tidak langsung jatuh.

## 3. Menyetel bila ada masalah

| Gejala di halaman Log Server / Monitor | Yang dilakukan |
|---|---|
| "permintaan ditolak (503) karena permintaan serentak melewati batas" sering | Periksa Monitor Resource. Bila CPU/RAM masih lega, naikkan `MAKS_PERMINTAAN_SERENTAK`; bila tidak, tambah resource lebih dulu |
| "antrean pekerjaan berat penuh" | Minta pengguna mengekspor bergantian, atau tambah CPU lalu naikkan `MAKS_BERAT_SERENTAK` |
| 429 pada pengguna sah di satu kantor | Naikkan `RATE_LIMIT_UMUM_PER_DETIK`; pastikan `TRUSTED_PROXIES` memuat tempat Nginx berjalan, kalau tidak semua pengguna terlihat berasal dari satu IP proksi |
| Banyak "permintaan lambat" pada rute yang sama | Query rute itu yang perlu dioptimalkan (indeks/filter); lihat durasi dan ID permintaan |
| "catatan tidak sempat disimpan" | Ada badai galat atau database macet; cari kejadian dengan hitungan terbesar, itu akar masalahnya |
| Permintaan menunggu koneksi database (Monitor Resource → Database) | Naikkan `DB_MAKS_KONEKSI` (periksa batas koneksi di server database) |

## 4. Batasan yang diketahui

- **Satu instance backend.** Cache autentikasi, pembatas laju, captcha, dan antrean log ada di memori satu proses. Bila nanti dijalankan beberapa replika: cache autentikasi tetap benar (batas TTL), tetapi pembatas laju dihitung per replika, dan captcha perlu penyimpanan terpusat (catatan lama di `handlers/captcha_handler.go`).
- **Linux/Docker asli belum diuji** untuk pembaca `/proc` dan cgroup (Monitor Resource) maupun penentuan batas memori otomatis dari cgroup; uji beban dijalankan di Windows dengan SQL Server lokal. Setelah deploy, cek `docker ps` (kolom STATUS harus `healthy`), `curl /health/ready`, dan halaman Log Server.
- **`-race` tidak tersedia** di mesin pengembangan (butuh cgo); kode bersamaan diuji dengan uji beban dan tes serentak, bukan detektor balapan data.
- Mekanisme `context canceled` diabaikan berdasarkan teks pesan galat; bila suatu saat ada bug yang membatalkan konteks terlalu dini, galatnya hanya terlihat sebagai hitungan "dibatalkan klien" di halaman.
