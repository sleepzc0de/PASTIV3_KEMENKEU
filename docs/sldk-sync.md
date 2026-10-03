# Sinkronisasi Ringkasan Data Aset (SLDK)

Halaman **Data Aset (SLDK)** punya tiga tab:

| Tab | Sumber data | Catatan |
|---|---|---|
| Pencarian | Langsung ke SLDK | Tabel aset ±132 juta baris (±206 GB). Paling cepat dengan kode register / No. KIB / No. polisi / serial number, atau filter satuan kerja. |
| Ringkasan | Tabel `sldk_agregat` di database PASTI | Diisi oleh `pasti-sldk-sync ringkasan`. Aplikasi tidak pernah menghitung langsung ke tabel aset SLDK. |
| Pemantauan | Tabel `sldk_agregat` di database PASTI | Temuan: aset idle, hilang, dihentikan, nilai buku nol, tanggal tidak valid, rusak berat. |

Ringkasan dan Pemantauan kosong ("Ringkasan aset belum tersedia") sampai sinkronisasi pertama berhasil dijalankan.

## Prasyarat

1. Migrasi `019_create_sldk_ringkasan.sql` sudah berjalan (`deploy.sh` menjalankannya otomatis lewat `pasti-migrate`).
2. `backend/.env` berisi `SLDK_DB_*` dan `SLDK_ASSET_TABLE` (mis. `DJKN.SIMAN2_M_ASET`), dan IP server aplikasi sudah di-whitelist di SLDK.
3. `SLDK_KL_KODE=015` membatasi data ke Kementerian Keuangan (kode K/L 015). Kosong atau tidak ada = semua K/L. Pembatasan berlaku
   untuk Pencarian, Ringkasan, dan Pemantauan sekaligus. `.env` baru buatan `deploy.sh` sudah berisi `SLDK_KL_KODE=015`; untuk
   `backend/.env` yang sudah ada, tambahkan barisnya sendiri, lalu jalankan `probe` (langkah 1) **sebelum** memakainya: pembatasan
   ini bergantung pada keterkaitan `id_satker` → `SIMAN2_R_SATKER` → `SIMAN2_R_KL` yang belum pernah diuji terhadap SLDK asli, dan
   `probe` melaporkan bila tidak ada aset yang tertangkap. Setelah mengubah `.env`, jalankan ulang `./deploy.sh` agar backend memuatnya.

## Langkah pertama kali

Image backend sudah memuat biner `pasti-sldk-sync`. Jalankan dari folder proyek di server:

### 1. `probe` (ringan, baca-saja)

```bash
docker compose --env-file deploy.env run --rm --no-deps --entrypoint ./pasti-sldk-sync backend probe
```

`probe` **tidak** memindai tabel aset penuh. Ia memeriksa versi server, kolom yang dibutuhkan, ukuran dan index tabel, isi tabel
referensi, cakupan K/L, lalu menguji query agregat pada sampel ±50 ribu baris. Baca bagian **Kesimpulan** (masalah yang perlu
diperbaiki) dan **Langkah berikutnya** di akhir keluarannya. Hal yang diverifikasi di sini dan belum pernah diuji terhadap SLDK asli:

- nama kolom tabel referensi dan kesamaan `id_satker` antara tabel aset dan `SIMAN2_R_SATKER`;
- nilai penanda `*_yn` (idle, hilang, dihentikan);
- sebaran `status_data` / `sts_his` / `sts_ast` (lihat "Definisi aset aktif" di bawah);
- ada tidaknya index pada tabel aset (menentukan seberapa cepat Pencarian dengan filter satker).

### 2. `ringkasan -dry-run`

```bash
docker compose --env-file deploy.env run --rm --no-deps --entrypoint ./pasti-sldk-sync backend ringkasan -dry-run
```

Hanya mencetak rencana dan SQL agregatnya; tidak menyentuh SLDK.

### 3. `ringkasan`

```bash
docker compose --env-file deploy.env run --rm --no-deps --entrypoint ./pasti-sldk-sync backend ringkasan
```

Memindai tabel aset **satu kali** (beberapa menit sampai beberapa jam, batas bawaan 3 jam) dan membebani server SLDK, jadi
jalankan **di luar jam kerja** dan koordinasikan dengan pengelola SLDK. Perintah meminta konfirmasi (`ya`). Hasilnya ditulis ke
`sldk_agregat` dalam satu transaksi; bila gagal atau hasilnya kosong, data lama **tidak** diganti. Riwayat tiap percobaan ada di
`sldk_sync_log`.

Opsi: `-kl <kode>` (menimpa `SLDK_KL_KODE`), `-timeout <durasi>`, `-yes` (lewati konfirmasi).

### 4. Tentukan "aset aktif" (admin)

Tabel aset SLDK memuat baris riwayat dan baris yang sudah dihapus. Arti `status_data`, `sts_his`, `sts_ast`, dan `tgl_hapus` belum
terkonfirmasi di sistem ini, jadi sinkronisasi menyimpan angka per **kombinasi penanda** (mis. `1|0|1|A`) dan admin yang memilih
kombinasi mana yang dihitung sebagai aset aktif:

1. Buka Data Aset, tab Ringkasan, tekan **Definisi aset aktif** (khusus admin/superadmin).
2. Centang kombinasi yang berarti aset aktif menurut pemahaman Anda atas data SLDK. Tanpa pilihan, semua baris dihitung (dan halaman
   menampilkan peringatan kuning).
3. Simpan. Berlaku segera, tanpa sinkronisasi ulang. Pengaturan disimpan di `sldk_pengaturan` (kunci `aktif_flag_keys`).

## Penjadwalan (cron)

Jadwalkan di luar jam kerja, mis. tiap Minggu pukul 01.00. Tanpa terminal, `-yes` wajib dan `docker compose run` perlu `-T`:

```cron
0 1 * * 0  cd /opt/pasti-v3 && docker compose --env-file deploy.env run --rm -T --no-deps --entrypoint ./pasti-sldk-sync backend ringkasan -yes >> /var/log/pasti-sldk-sync.log 2>&1
```

Sesuaikan folder proyek. Sinkronisasi yang macet (status `berjalan` lebih dari 6 jam) otomatis ditandai gagal pada percobaan
berikutnya; sinkronisasi kedua ditolak selama masih ada yang berjalan.

## Aturan pemantauan

Definisi tiap aturan hidup di satu tempat, `backend/sldk/rules.go`, dan dipakai bersama oleh query agregat dan filter "Penanda"
di Pencarian, sehingga angka di Pemantauan sama dengan jumlah baris yang ditemukan Pencarian.

| Kunci | Kondisi |
|---|---|
| `idle` | `status_bmn_idle` bernilai ya |
| `hilang` | `brg_hilang_yn` bernilai ya |
| `dihentikan` | `dihentikan_yn` bernilai ya |
| `nilai_nol` | `rph_buku = 0` padahal `rph_aset > 0` |
| `dq_tanggal` | `dq_tgl_invalid_cnt > 0` |

"Rusak berat" tidak termasuk aturan: kodenya dicari dari tabel referensi kondisi (nama mengandung "berat"), tidak ditebak.

Menambah aturan baru: tambahkan ke `sldk.Rules`, tambahkan kolom `n_<kunci>` (`BIGINT NOT NULL DEFAULT 0`) di migrasi baru pada
`sldk_agregat`, lalu jalankan ulang `ringkasan`.

## Pemecahan masalah

| Gejala | Penyebab / tindakan |
|---|---|
| "Ringkasan aset belum tersedia" | Sinkronisasi belum pernah sukses; jalankan langkah di atas. Status terakhir tampil di halaman. |
| Sinkronisasi gagal "hasil agregat kosong" | `SLDK_KL_KODE` tidak cocok atau keterkaitan `id_satker` bermasalah; jalankan `probe`. |
| Sinkronisasi gagal karena batas waktu | Naikkan `-timeout`; periksa beban SLDK; jalankan `probe` untuk melihat ukuran dan index. |
| `Tidak bisa terhubung ke SLDK` | Periksa `SLDK_DB_*` dan whitelist IP server di SLDK. |
| Angka berubah setelah mengganti definisi aset aktif | Wajar; definisi dipakai saat membaca, data mentahnya tidak berubah. |
