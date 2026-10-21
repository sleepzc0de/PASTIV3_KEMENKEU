# Deploy pertama kali di VPS Ubuntu (database di server terpisah)

Panduan langkah demi langkah untuk **production** (branch `production_v3`, HTTPS). Untuk dev, lihat [catatan di akhir](#dev-di-vps-yang-sama-atau-berbeda). Skrip yang dipakai: `deploy.sh`
(lihat `./deploy.sh --help`); konfigurasi Nginx contoh: `deploy/nginx/pasti.kemenkeu.go.id.conf`.

Topologi: pengguna → Nginx di VPS (HTTPS) → container backend (Go, port 8686) dan frontend (Next.js, port 3000) → SQL Server di **server lain** lewat TCP 1433. Database tidak dipasang oleh skrip.

## Yang perlu disiapkan dulu

- Domain yang sudah mengarah ke IP VPS (A record) dan sertifikat TLS.
- IP server database dan akses admin (sysadmin) ke SQL Server itu.
- Client ID dan secret SSO Kemenkeu **production**, dan `INAPROC_TOKEN` (wajib di production).
- Akses git ke repo; bila privat, deploy key (read-only) atau token.

## A. Di server database

`deploy.sh` **tidak membuat database**; migrasi hanya membuat tabel di dalamnya. Jalankan sebagai sysadmin:

```sql
CREATE DATABASE pasti_v3_db;
GO
CREATE LOGIN pasti_app WITH PASSWORD = 'GANTI-SANDI-KUAT', CHECK_POLICY = ON;
GO
USE pasti_v3_db;
CREATE USER pasti_app FOR LOGIN pasti_app;
ALTER ROLE db_owner ADD MEMBER pasti_app;   -- migrasi membuat dan mengubah tabel
GO
USE master;
GRANT VIEW SERVER STATE TO pasti_app;       -- opsional: Monitor Resource (memori SQL Server, ruang disk volume)
GO
```

- Sandi sebaiknya hanya huruf dan angka (`$` dan `#` bisa merusak pembacaan `.env`), mis. `openssl rand -base64 36 | tr -dc 'A-Za-z0-9' | head -c 32`.
- Aktifkan **TCP/IP** pada port 1433 dan mode autentikasi **Mixed** (SQL login).
- Firewall server database (OS dan jaringan penyedia): izinkan **hanya IP VPS** ke port 1433.
- Aplikasi memakai `encrypt=true;TrustServerCertificate=true`, jadi sertifikat self-signed di SQL Server tidak masalah.
- Buat backup terjadwal `pasti_v3_db`; deploy production juga memperingatkan agar backup dibuat sebelum migrasi.

## B. Menyiapkan VPS

Sebagai pengguna biasa yang punya sudo (bukan root; `deploy.sh` meminta sudo sendiri):

```bash
sudo apt update && sudo apt -y upgrade
sudo apt -y install git curl openssl ufw
sudo ufw allow OpenSSH && sudo ufw allow 80/tcp && sudo ufw allow 443/tcp && sudo ufw --force enable
```

Docker mengabaikan `ufw` untuk port yang dipublikasikan, sehingga port aplikasi dibuka hanya ke `127.0.0.1` (langkah C), dan hanya Nginx yang menjangkaunya.

Bila RAM kurang dari 2 GB, tambah swap agar build Next.js tidak kehabisan memori:

```bash
sudo fallocate -l 2G /swapfile && sudo chmod 600 /swapfile && sudo mkswap /swapfile && sudo swapon /swapfile
echo '/swapfile none swap sw 0 0' | sudo tee -a /etc/fstab
```

Batasi ukuran log Docker **sebelum** Docker terpasang (`docker-compose.yml` juga membatasi log tiap container 5 × 10 MB, jadi ini lapis kedua):

```bash
sudo mkdir -p /etc/docker
echo '{"log-driver":"json-file","log-opts":{"max-size":"10m","max-file":"5"}}' | sudo tee /etc/docker/daemon.json
```

Clone repo (satu folder = satu environment):

```bash
git clone --branch production_v3 https://github.com/sleepzc0de/PASTIV3_KEMENKEU.git ~/pasti-prod
```

Bila repo privat: `ssh-keygen -t ed25519`, tambahkan kunci publik sebagai **Deploy key** di GitHub, lalu clone lewat `git@github.com:sleepzc0de/PASTIV3_KEMENKEU.git`.

## C. Deploy pertama: skrip berhenti meminta `.env` diisi

```bash
cd ~/pasti-prod && ./deploy.sh prod
```

| Pertanyaan | Jawaban |
|---|---|
| Pasang Docker? | `Y` |
| Alamat backend/API | `https://domain-anda` (wajib https, tanpa `/` di akhir) |
| Port backend / frontend | Enter (8686 / 3000) |
| Diakses lewat Nginx di server ini? | `Y` (port hanya terbuka ke 127.0.0.1) |

Skrip membuat `deploy.env` dan `backend/.env` (`JWT_SECRET`, `PASSWORD_PEPPER`, `TOKEN_ENCRYPTION_KEY` terisi acak), lalu **berhenti** dengan pesan "backend/.env baru dibuat dan BELUM siap". Itu normal.

## D. Isi `backend/.env`

```bash
nano backend/.env
chmod 600 backend/.env
```

| Kunci | Isi |
|---|---|
| `DB_HOST`, `DB_PORT` | **IP server database**, bukan `localhost` (itu menunjuk ke container sendiri). Port 1433 |
| `DB_USER`, `DB_PASSWORD`, `DB_NAME` | `pasti_app`, sandi dari langkah A, `pasti_v3_db`. Sandi berkarakter khusus diapit kutip tunggal `'...'` |
| `SSO_CLIENT_ID`, `SSO_CLIENT_SECRET` | Kredensial SSO **production** (secret contoh di repo ditolak skrip) |
| `SSO_SCOPE` | Tambahkan `hris2 profil.hris` bila memakai Cari Pegawai (HRIS2) |
| `SUPERADMIN_PROTECTED_EMAIL`, `SUPERADMIN_PROTECTED_NIP` | **Periksa.** Isi bawaan contoh adalah identitas pengembang; ganti dengan superadmin yang benar (boleh beberapa, dipisah koma). Jangan dikosongkan di production |
| `INAPROC_TOKEN` | Wajib di production |
| `SLDK_DB_HOST`, `SLDK_DB_PORT`, `SLDK_DB_USER`, `SLDK_DB_PASSWORD`, `SLDK_DB_NAME` | Opsional, untuk sinkronisasi Digitalisasi Aset. Firewall SLDK juga harus mengizinkan IP VPS |

Sudah terisi otomatis dan sebaiknya tidak diubah: `APP_ENV` dan `SSO_ENV` (production), `FRONTEND_URL`, `SSO_REDIRECT_URI`. **`TOKEN_ENCRYPTION_KEY` tidak boleh diganti** setelah dipakai. Simpan salinan `backend/.env` dan `deploy.env` di tempat aman.
Opsional: `AUDIT_RETENSI_HARI`, `MONITOR_AKTIF`, `MONITOR_RETENSI_HARI` (lihat [audit-dan-monitor.md](audit-dan-monitor.md)); `PERAN_DATA_WAJIB=true` setelah semua pengguna punya peran.
Opsional: `API_AKSES_PEPPER` untuk hash kunci API (bawaan diturunkan dari `PASSWORD_PEPPER`; mengganti salah satunya membuat semua kunci API klien tidak berlaku). Batas laju dan kuota API diatur superadmin di aplikasi, bukan di `.env` (lihat [api-akses-data.md](api-akses-data.md)).

Daftarkan **Redirect URI** `https://domain-anda/sso/callback/login` pada client SSO production (persis sama). Itu dilakukan di pihak SSO Kemenkeu.

## E. Deploy sungguhan

```bash
./deploy.sh prod
```

Urutannya: memeriksa port database terjangkau dari VPS → konfirmasi (ketik `prod`) → build image (5 sampai 15 menit) → migrasi (deploy pertama menjalankan semuanya, sekitar satu menit) → menjalankan container → cek kesehatan. Gagal sebelum container dijalankan = belum ada yang berubah; perbaiki lalu jalankan ulang.

| Pesan | Penyebab |
|---|---|
| "Server database ... TIDAK terjangkau" | Firewall database belum mengizinkan IP VPS, TCP/IP belum aktif, atau `DB_HOST`/`DB_PORT` salah |
| "Login failed for user" | `DB_USER` atau `DB_PASSWORD` salah |
| "Cannot open database" | Database belum dibuat (langkah A) |
| "Container ... sudah ada tetapi bukan dibuat oleh Docker Compose" padahal `docker ps -a` kosong | Yang ditemukan sebenarnya IMAGE sisa build yang namanya sama (`pasti-prod-backend`). Sudah diperbaiki di `deploy.sh` (memakai `docker container inspect`); pada server yang belum menarik perbaikan itu, hapus image sisa dengan `docker rmi pasti-prod-backend pasti-prod-frontend` lalu ulangi `./deploy.sh prod` |
| Migrasi gagal dengan "permission denied" | Login belum berperan `db_owner` |
| "Port ... sudah dipakai" | Ubah port di `deploy.env` |
| Build gagal kehabisan memori | Tambah swap (langkah B) |

Pada akhirnya skrip mencetak **alamat halaman login**. Bawaannya `https://domain-anda/TDA5MW5wNHN0MXYzMjAyRw==` (dari `frontend/login-path.txt`); `/login` biasa sengaja dijawab 404. Untuk alamat sendiri isi `LOGIN_PATH=` di `deploy.env` (satu segmen 16 sampai 128 karakter huruf, angka, `_`, `-`, `=`), lalu jalankan `./deploy.sh prod` lagi. Lihat [login-tersembunyi.md](login-tersembunyi.md).

## F. Nginx dan HTTPS

```bash
sudo apt -y install nginx
```

Sertifikat: dari tim infrastruktur (simpan di `/etc/ssl/pasti/fullchain.pem` dan `privkey.pem`, kunci `chmod 600`), atau Let's Encrypt bila domain publik dan DNS sudah mengarah ke VPS:

```bash
sudo apt -y install certbot
sudo certbot certonly --webroot -w /var/www/html -d domain-anda
```

Pasang konfigurasi dari repo:

```bash
sudo cp ~/pasti-prod/deploy/nginx/pasti.kemenkeu.go.id.conf /etc/nginx/sites-available/pasti
sudo nano /etc/nginx/sites-available/pasti
```

Sesuaikan `server_name` (dua tempat), `ssl_certificate` dan `ssl_certificate_key` (certbot: `/etc/letsencrypt/live/domain-anda/...`), serta port `upstream` bila diubah. Lalu aktifkan:

```bash
sudo ln -s /etc/nginx/sites-available/pasti /etc/nginx/sites-enabled/pasti && sudo rm -f /etc/nginx/sites-enabled/default
sudo nginx -t && sudo systemctl reload nginx
```

`/api/`, `/health`, `/sso/login`, dan `/sso/callback/login` diteruskan ke backend; sisanya ke frontend. Pendaftaran akun publik (`/api/v1/auth/register`) ditutup oleh konfigurasi itu. Bila HTTPS diterminasi perangkat lain (WAF atau load balancer), perangkat itu harus meneruskan rute yang sama.

## G. Verifikasi

```bash
curl -s https://domain-anda/health
curl -s https://domain-anda/health/ready
docker ps --format 'table {{.Names}}	{{.Status}}'
sudo ss -ltn | grep -E ':(8686|3000)\b'
docker logs --tail 40 pasti-prod-backend
```

- `/health` menjawab `{"service":"PASTI V3 Backend","status":"ok"}`; `/health/ready` juga memeriksa database (503 bila database tidak menjawab).
- `docker ps` menampilkan backend `(healthy)` setelah ±1 menit.
- Port 8686 dan 3000 hanya tampil di `127.0.0.1`. Bila terbuka ke luar, alamat IP di log audit bisa dipalsukan klien.
- Log backend sengaja singkat (hanya error dan warning, tanpa log akses) dan memuat `[INFO] Log server aktif ...`. Pesan `[AUDIT]`/`[MONITOR]` berlevel info tidak dicetak di production; keberadaannya terlihat di halaman Log Audit dan Monitor Resource.

Di browser:

1. Buka alamat halaman login dan masuk lewat SSO dengan identitas superadmin.
2. **Monitor Resource → Server**: periksa angka CPU, RAM, dan disk (pembacaan di Linux asli belum pernah diuji sebelumnya).
3. **Log Server**: pastikan halaman terbuka dan jumlah error/warning wajar. Bila semua aktivitas di Log Audit tampak berasal dari satu alamat IP, periksa `TRUSTED_PROXIES` di `backend/.env` (lihat `docs/log-server-dan-ketahanan.md`).
4. **Manajemen Pengguna**: beri peran ke pengguna.
5. Isi **Referensi UE1** dan **Referensi Kanwil**, unggah template di **Pengaturan SAPA**, dan cek **Penarikan Data**.

## H. Sesudahnya

- **Pembaruan:** setelah merge ke `production_v3`, cukup `cd ~/pasti-prod && ./deploy.sh prod`. Skrip menarik kode, build, migrasi, cek kesehatan, dan otomatis kembali ke image sebelumnya bila gagal. Log deploy ada di `deploy.log`.
- **Reboot:** Docker dan container menyala sendiri (`restart: unless-stopped`).
- **Mengubah `deploy.env`** (alamat atau port) memerlukan `./deploy.sh prod` lagi, karena `NEXT_PUBLIC_*` ditanam ke frontend saat build.
- **Log dan ketahanan:** log server hanya menyimpan error/warning (retensi 30/7 hari, maks 50.000 baris) dan backend membatasi laju, beban, dan ekspor berat; pengaturan dan cara menyetelnya ada di `docs/log-server-dan-ketahanan.md`.
- **Disk:** pantau di Monitor Resource; `docker system df` dan `docker image prune` bila image lama menumpuk.
- **Cadangan:** database (terjadwal di server database), `backend/.env`, dan `deploy.env`.

## Dev di VPS yang sama atau berbeda

Gunakan folder terpisah: `git clone --branch development ... ~/pasti-dev`, lalu `./deploy.sh dev`. Alamat backend-nya `http://IP-VPS:8686` dan tanpa HTTPS. Bila dev dan prod berjalan di satu VPS, `CONTAINER_PREFIX` dan port di `deploy.env` harus berbeda; skrip memeriksa dan menolak yang bentrok. Database dev sebaiknya terpisah dari production.
